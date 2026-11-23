package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	mysqldao "github.com/goodrain/rainbond/db/mysql/dao"
	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	rainbondclient "github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	rainbondfake "github.com/goodrain/rainbond/pkg/generated/clientset/versioned/fake"
	workerclient "github.com/goodrain/rainbond/worker/client"
	"github.com/goodrain/rainbond/worker/server/pb"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

type pluginReadStatusClient struct {
	pb.AppRuntimeSyncClient
	list func(context.Context, *pb.AppStatusesReq) (*pb.AppStatuses, error)
}

func (c pluginReadStatusClient) ListAppStatuses(ctx context.Context, req *pb.AppStatusesReq, _ ...grpc.CallOption) (*pb.AppStatuses, error) {
	return c.list(ctx, req)
}

type pluginReadManager struct {
	db.Manager
	database *gorm.DB
}

func (m pluginReadManager) ApplicationDao() dbdao.ApplicationDao {
	return &mysqldao.ApplicationDaoImpl{DB: m.database}
}

func (m pluginReadManager) TenantDao() dbdao.TenantDao {
	return &mysqldao.TenantDaoImpl{DB: m.database}
}

func pluginReadDatabase(t *testing.T) sqlmock.Sqlmock {
	t.Helper()
	raw, mock, err := sqlmock.New()
	require.NoError(t, err)
	database, err := gorm.Open("mysql", raw)
	require.NoError(t, err)
	database.LogMode(false)
	previous := db.GetManager()
	db.SetTestManager(pluginReadManager{database: database})
	t.Cleanup(func() {
		db.SetTestManager(previous)
		assert.NoError(t, mock.ExpectationsWereMet())
		_ = raw.Close()
	})
	return mock
}

func pluginReadObject(name, appID string, official bool) *v1alpha1.RBDPlugin {
	labels := map[string]string{"app_id": appID}
	if official {
		labels[OfficialPluginLabel] = "official-" + name
	}
	return &v1alpha1.RBDPlugin{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "plugins", Labels: labels},
		Spec: v1alpha1.RBDPluginSpec{
			DisplayName: "Plugin " + name, PluginType: v1alpha1.JSInject,
			PluginViews: []v1alpha1.PluginView{v1alpha1.Team}, FrontendComponent: "fixture",
			EntryPath: "/entry", MenuTitle: "Fixture", RoutePath: "/fixture", Namespace: "backend",
			BackendService: "backend.fixture.svc", FrontendService: "frontend.fixture.svc",
		},
	}
}

// capability_id: rainbond.plugins.scoped-context-reads
func TestPluginReadsScopeTenantQueriesAndPreserveResponse(t *testing.T) {
	for _, official := range []bool{false, true} {
		t.Run(map[bool]string{false: "all", true: "official"}[official], func(t *testing.T) {
			mock := pluginReadDatabase(t)
			mock.ExpectQuery("SELECT .*applications.*app_id in").WithArgs("app-a", "app-b", "missing").
				WillReturnRows(sqlmock.NewRows([]string{"app_id", "tenant_id"}).AddRow("app-a", "team-a").AddRow("app-b", "team-a"))
			mock.ExpectQuery("SELECT .*tenants.*uuid IN").WithArgs("team-a").
				WillReturnRows(sqlmock.NewRows([]string{"uuid", "name"}).AddRow("team-a", "workspace"))
			plugins := []*v1alpha1.RBDPlugin{pluginReadObject("a", "app-a", true), pluginReadObject("b", "app-b", false), pluginReadObject("c", "app-a", false), pluginReadObject("d", "missing", true)}
			objects := make([]runtime.Object, len(plugins))
			for i := range plugins {
				objects[i] = plugins[i]
			}
			calls := 0
			action := &clusterAction{rainbondClient: rainbondfake.NewSimpleClientset(objects...), statusCli: &workerclient.AppRuntimeSyncClient{AppRuntimeSyncClient: pluginReadStatusClient{list: func(ctx context.Context, req *pb.AppStatusesReq) (*pb.AppStatuses, error) {
				calls++
				assert.Equal(t, []string{"app-a", "app-b", "missing"}, req.AppIds)
				deadline, ok := ctx.Deadline()
				assert.True(t, ok, "status RPC must have a deadline")
				if ok {
					assert.LessOrEqual(t, time.Until(deadline), 8*time.Second)
				}
				return &pb.AppStatuses{AppStatuses: []*pb.AppStatus{{AppId: "app-a", Status: "running"}}}, nil
			}}}}
			got, err := action.ListPlugins(context.Background(), official)
			require.NoError(t, err)
			require.Equal(t, 1, calls)
			expected := make([]*apimodel.RainbondPlugins, 0)
			for _, plugin := range plugins {
				name := plugin.Name
				if official {
					if plugin.Labels[OfficialPluginLabel] == "" {
						continue
					}
					name = plugin.Labels[OfficialPluginLabel]
				}
				status, team := "NIL", "workspace"
				if plugin.Labels["app_id"] == "app-a" {
					status = "running"
				}
				if plugin.Labels["app_id"] == "missing" {
					team = ""
				}
				expected = append(expected, &apimodel.RainbondPlugins{RegionAppID: plugin.Labels["app_id"], Name: name, TeamName: team, Status: status, Alias: plugin.Spec.DisplayName, Labels: plugin.Labels, PluginType: "JSInject", PluginViews: []string{"Team"}, EnableStatus: "true", FrontendComponent: "fixture", EntryPath: "/entry", MenuTitle: "Fixture", RoutePath: "/fixture", Namespace: "backend", BackendService: "backend.fixture.svc", FrontendService: "frontend.fixture.svc"})
			}
			assert.Equal(t, expected, got)
		})
	}
}

func TestPluginReadsSkipUnrelatedEnrichment(t *testing.T) {
	for _, name := range []string{"empty plugins", "no app labels", "missing apps"} {
		t.Run(name, func(t *testing.T) {
			mock := pluginReadDatabase(t)
			objects := []runtime.Object{}
			if name != "empty plugins" {
				appID := ""
				if name == "missing apps" {
					appID = "missing"
					mock.ExpectQuery("SELECT .*applications.*app_id in").WithArgs(appID).WillReturnRows(sqlmock.NewRows([]string{"app_id", "tenant_id"}))
				}
				objects = append(objects, pluginReadObject("a", appID, false))
			}
			calls := 0
			action := &clusterAction{rainbondClient: rainbondfake.NewSimpleClientset(objects...), statusCli: &workerclient.AppRuntimeSyncClient{AppRuntimeSyncClient: pluginReadStatusClient{list: func(context.Context, *pb.AppStatusesReq) (*pb.AppStatuses, error) {
				calls++
				return &pb.AppStatuses{}, nil
			}}}}
			got, err := action.ListPlugins(context.Background(), false)
			require.NoError(t, err)
			if name == "empty plugins" {
				encoded, encodeErr := json.Marshal(got)
				require.NoError(t, encodeErr)
				assert.Equal(t, "[]", string(encoded))
			} else {
				require.Len(t, got, 1)
				assert.Equal(t, "NIL", got[0].Status)
				assert.Empty(t, got[0].TeamName)
			}
			assert.Equal(t, map[bool]int{true: 1, false: 0}[name == "missing apps"], calls)
		})
	}
}

func TestPluginReadsCarryCallerContextToStatusRPC(t *testing.T) {
	type contextKey string
	ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), contextKey("request"), "fixture"), 100*time.Millisecond)
	defer cancel()
	wantDeadline, _ := ctx.Deadline()
	wantErr := errors.New("fixture RPC failure")
	action := &clusterAction{rainbondClient: rainbondfake.NewSimpleClientset(pluginReadObject("a", "app-a", false)), statusCli: &workerclient.AppRuntimeSyncClient{AppRuntimeSyncClient: pluginReadStatusClient{list: func(got context.Context, _ *pb.AppStatusesReq) (*pb.AppStatuses, error) {
		assert.Equal(t, "fixture", got.Value(contextKey("request")))
		deadline, ok := got.Deadline()
		assert.True(t, ok)
		assert.Equal(t, wantDeadline, deadline, "do not restart or extend a shorter caller budget")
		cancel()
		assert.ErrorIs(t, got.Err(), context.Canceled)
		return nil, wantErr
	}}}}
	_, err := action.ListPlugins(ctx, false)
	assert.ErrorIs(t, err, wantErr)
}

func TestPluginReadsCancelKubernetesRequest(t *testing.T) {
	started, canceled := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
			close(canceled)
		case <-time.After(time.Second):
			http.Error(w, "fixture: caller cancellation was lost", http.StatusGatewayTimeout)
		}
	}))
	defer server.Close()
	client, err := rainbondclient.NewForConfig(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { <-started; cancel() }()
	_, err = (&clusterAction{rainbondClient: client}).ListPlugins(ctx, false)
	assert.ErrorIs(t, err, context.Canceled)
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Error("Kubernetes request outlived the caller")
	}
}

type pluginReadTransport func(*http.Request) (*http.Response, error)

func (f pluginReadTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPluginReadsShareOneBudgetAcrossKubernetesAndRPC(t *testing.T) {
	var kubernetesDeadline time.Time
	client, err := rainbondclient.NewForConfig(&rest.Config{Host: "http://fixture.invalid", Transport: pluginReadTransport(func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		require.True(t, ok, "Kubernetes request needs the same finite budget as status enrichment")
		kubernetesDeadline = deadline
		require.InDelta(t, 8, time.Until(deadline).Seconds(), 0.5)
		body, err := json.Marshal(&v1alpha1.RBDPluginList{Items: []v1alpha1.RBDPlugin{*pluginReadObject("a", "app-a", false)}})
		require.NoError(t, err)
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(body)))}, nil
	})})
	require.NoError(t, err)
	stop := errors.New("fixture: stop before database")
	action := &clusterAction{rainbondClient: client, statusCli: &workerclient.AppRuntimeSyncClient{AppRuntimeSyncClient: pluginReadStatusClient{list: func(ctx context.Context, _ *pb.AppStatusesReq) (*pb.AppStatuses, error) {
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		assert.Equal(t, kubernetesDeadline, deadline, "RPC must not restart the eight-second budget")
		return nil, stop
	}}}}
	_, err = action.ListPlugins(context.Background(), false)
	assert.ErrorIs(t, err, stop)
}

func TestPluginReadsStopExpiredOrCanceledWork(t *testing.T) {
	t.Run("already canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		// No clients: canceled callers must not enter either downstream dependency.
		_, err := (&clusterAction{}).ListPlugins(ctx, false)
		assert.ErrorIs(t, err, context.Canceled)
	})
	t.Run("RPC deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		action := &clusterAction{rainbondClient: rainbondfake.NewSimpleClientset(pluginReadObject("a", "app-a", false)), statusCli: &workerclient.AppRuntimeSyncClient{AppRuntimeSyncClient: pluginReadStatusClient{list: func(ctx context.Context, _ *pb.AppStatusesReq) (*pb.AppStatuses, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}}}}
		_, err := action.ListPlugins(ctx, false)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})
}

func TestPluginReadsPreserveDatabaseErrors(t *testing.T) {
	for _, fail := range []string{"application", "tenant"} {
		t.Run(fail, func(t *testing.T) {
			mock := pluginReadDatabase(t)
			wantErr := errors.New("fixture database unavailable")
			query := mock.ExpectQuery("SELECT .*applications.*app_id in").WithArgs("app-a")
			if fail == "application" {
				query.WillReturnError(wantErr)
			} else {
				query.WillReturnRows(sqlmock.NewRows([]string{"app_id", "tenant_id"}).AddRow("app-a", "team-a"))
				mock.ExpectQuery("SELECT .*tenants.*uuid IN").WithArgs("team-a").WillReturnError(wantErr)
			}
			action := &clusterAction{rainbondClient: rainbondfake.NewSimpleClientset(pluginReadObject("a", "app-a", false)), statusCli: &workerclient.AppRuntimeSyncClient{AppRuntimeSyncClient: pluginReadStatusClient{list: func(context.Context, *pb.AppStatusesReq) (*pb.AppStatuses, error) {
				return &pb.AppStatuses{}, nil
			}}}}
			got, err := action.ListPlugins(context.Background(), false)
			assert.Nil(t, got)
			assert.ErrorIs(t, err, wantErr)
			assert.Contains(t, err.Error(), map[string]string{"application": "get apps by app ids", "tenant": "get tenants"}[fail])
		})
	}
}
