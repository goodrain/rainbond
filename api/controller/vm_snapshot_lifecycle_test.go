package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	mysqldao "github.com/goodrain/rainbond/db/mysql/dao"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"

	"github.com/go-chi/chi"

	"github.com/goodrain/rainbond/api/handler"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/pkg/vmrestore"
)

// capability_id: rainbond.vm-snapshot.http-bounded-contract
func TestSnapshotHTTPRejectsInjectedScopeAndKeepsAcceptanceSeparate(t *testing.T) {
	calls := 0
	c := &VMSnapshotController{createSnapshot: func(serviceID string, request *handler.VMSnapshotRequest) (*handler.VMSnapshotStatus, error) {
		calls++
		return &handler.VMSnapshotStatus{SnapshotName: request.Name}, nil
	}}
	for _, test := range []struct {
		body   string
		status int
	}{{`{"name":"snapshot-a","namespace":"foreign"}`, 400}, {`{"name":"snapshot-a"} {}`, 400}, {`{"name":"snapshot-a","operator":"user"}`, 202}} {
		request := httptest.NewRequest(http.MethodPost, "/vm-snapshots", bytes.NewBufferString(test.body))
		request = request.WithContext(context.WithValue(request.Context(), ctxutil.ContextKey("service_id"), "service"))
		request = request.WithContext(context.WithValue(request.Context(), ctxutil.ContextKey("tenant_id"), "tenant"))
		writer := httptest.NewRecorder()
		c.CreateVMSnapshot(writer, request)
		if writer.Code != test.status {
			t.Fatal("scope/acceptance contract changed")
		}
	}
	if calls != 1 {
		t.Fatal("invalid body reached backend operation")
	}
}

func TestSnapshotHTTPMapsDomainErrorsWithoutNativeText(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
	}{{vmrestore.ErrConflict, 409}, {vmrestore.ErrBusy, 409}, {vmrestore.ErrSnapshotStorage, 422}, {vmrestore.ErrNotFound, 404}, {vmrestore.ErrReferenceUnavailable, 503}, {errors.New("NON_SECRET_INTERNAL_SENTINEL"), 500}} {
		request := httptest.NewRequest(http.MethodGet, "/vm-snapshots/a", nil)
		writer := httptest.NewRecorder()
		snapshotHTTPError(writer, request, test.err)
		if writer.Code != test.status || strings.Contains(writer.Body.String(), "NON_SECRET_INTERNAL_SENTINEL") {
			t.Fatal("domain status or safe error boundary changed")
		}
	}
}

type snapshotHTTPService struct {
	calls   int
	action  string
	request vmrestore.SnapshotActionRequest
	dto     vmrestore.SnapshotOperationDTO
	err     error
}

func (s *snapshotHTTPService) PrepareVMSnapshot(ctx context.Context, id string, req *handler.VMSnapshotRequest) (*handler.VMSnapshotStatus, error) {
	s.calls++
	return &handler.VMSnapshotStatus{SnapshotName: req.Name, SnapshotOperationDTO: &s.dto}, s.err
}
func (s *snapshotHTTPService) ListVMSnapshots(ctx context.Context, id string, page, size int) (vmrestore.SnapshotListDTO, error) {
	s.calls++
	return vmrestore.SnapshotListDTO{Items: []vmrestore.SnapshotOperationDTO{s.dto}, Page: page, PageSize: size, Total: 1}, s.err
}
func (s *snapshotHTTPService) GetVMSnapshot(ctx context.Context, id, name string) (vmrestore.SnapshotOperationDTO, error) {
	s.calls++
	return s.dto, s.err
}
func (s *snapshotHTTPService) VMSnapshotAction(ctx context.Context, id, name, action string, req vmrestore.SnapshotActionRequest) (vmrestore.SnapshotOperationDTO, error) {
	s.calls++
	s.action = action
	s.request = req
	return s.dto, s.err
}
func snapshotHTTPRouter(c *VMSnapshotController) http.Handler {
	r := chi.NewRouter()
	r.Post("/vm-snapshots", c.CreateVMSnapshot)
	r.Get("/vm-snapshots", c.ListVMSnapshots)
	r.Get("/vm-snapshots/{name}", c.GetVMSnapshot)
	r.Post("/vm-snapshots/{name}/{snapshot_action}", c.SnapshotAction)
	r.Delete("/vm-snapshots/{name}", c.SnapshotAction)
	return r
}
func snapshotHTTPRequest(router http.Handler, method, url, body string, trusted bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	if trusted {
		ctx := context.WithValue(req.Context(), ctxutil.ContextKey("service_id"), "service")
		ctx = context.WithValue(ctx, ctxutil.ContextKey("tenant_id"), "tenant")
		req = req.WithContext(ctx)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}
func TestSnapshotHTTPRequiresTrustedScopeForEveryRoute(t *testing.T) {
	service := &snapshotHTTPService{}
	router := snapshotHTTPRouter(&VMSnapshotController{service: service})
	for _, r := range []struct{ method, path, body string }{{"POST", "/vm-snapshots", `{"name":"a"}`}, {"GET", "/vm-snapshots", ""}, {"GET", "/vm-snapshots/a", ""}, {"POST", "/vm-snapshots/a/halt", `{}`}, {"DELETE", "/vm-snapshots/a", `{}`}} {
		w := snapshotHTTPRequest(router, r.method, r.path, r.body, false)
		if w.Code != 403 {
			t.Fatalf("%s %s missing scope status=%d", r.method, r.path, w.Code)
		}
	}
	if service.calls != 0 {
		t.Fatal("untrusted request reached service")
	}
}
func TestSnapshotHTTPStrictBodyAndQuery(t *testing.T) {
	service := &snapshotHTTPService{}
	router := snapshotHTTPRouter(&VMSnapshotController{service: service})
	for _, body := range []string{`null`, `[]`, ``, `{`, `{"name":"a"} {}`, `{"name":"a","namespace":"foreign"}`, `{"name":"a","pvc":"foreign"}`, `{"name":"a","vm_json":{}}`, `{"name":"a","command":"sh"}`, `{"name":"a","description":"` + strings.Repeat("a", 17000) + `"}`} {
		if w := snapshotHTTPRequest(router, "POST", "/vm-snapshots", body, true); w.Code != 400 {
			t.Fatalf("invalid create status=%d", w.Code)
		}
	}
	for _, path := range []string{"/vm-snapshots?page=no", "/vm-snapshots?page_size=no", "/vm-snapshots?page=0", "/vm-snapshots?page_size=101"} {
		if w := snapshotHTTPRequest(router, "GET", path, "", true); w.Code != 400 {
			t.Fatalf("invalid query status=%d", w.Code)
		}
	}
	for _, path := range []string{"/vm-snapshots/a/patch", "/vm-snapshots/a/halt"} {
		if w := snapshotHTTPRequest(router, "POST", path, `{"namespace":"foreign"}`, true); w.Code != 400 {
			t.Fatalf("invalid action status=%d", w.Code)
		}
	}
	if service.calls != 0 {
		t.Fatal("invalid body reached service")
	}
}
func TestSnapshotHTTPActionPendingRetainsSafeDTO(t *testing.T) {
	for _, action := range []string{"halt", "create", "resume", "finalize", "cancel", "delete"} {
		t.Run(action, func(t *testing.T) {
			dto := vmrestore.SnapshotOperationDTO{SnapshotDTO: vmrestore.SnapshotDTO{SnapshotName: "a", OperationID: "op", CoreHash: strings.Repeat("a", 64), ErrorCode: "vm_snapshot_resume_outcome_pending"}, Finalized: true}
			service := &snapshotHTTPService{dto: dto, err: vmrestore.ErrSnapshotPending}
			router := snapshotHTTPRouter(&VMSnapshotController{service: service})
			method, path := "POST", "/vm-snapshots/a/"+action
			if action == "delete" {
				method, path = "DELETE", "/vm-snapshots/a"
			}
			w := snapshotHTTPRequest(router, method, path, `{"request_id":"attempt","native_proof_hash":"persisted-proof"}`, true)
			if w.Code != 202 || !strings.Contains(w.Body.String(), `"operation_id":"op"`) || !strings.Contains(w.Body.String(), `"deleted":false`) || service.action != action || service.request.NativeProofHash != "persisted-proof" {
				t.Fatalf("pending result lost: %d %s", w.Code, w.Body.String())
			}
		})
	}
}
func TestSnapshotHTTPReadAndActionContracts(t *testing.T) {
	service := &snapshotHTTPService{dto: vmrestore.SnapshotOperationDTO{SnapshotDTO: vmrestore.SnapshotDTO{SnapshotName: "a", ReadyToUse: true}}}
	router := snapshotHTTPRouter(&VMSnapshotController{service: service})
	for _, path := range []string{"/vm-snapshots", "/vm-snapshots/a"} {
		w := snapshotHTTPRequest(router, "GET", path, "", true)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"ready_to_use":true`) || !strings.Contains(w.Body.String(), `"manifest_acked":false`) || service.action != "" {
			t.Fatalf("read contract changed: %d %s", w.Code, w.Body.String())
		}
	}
	for _, action := range []string{"halt", "create", "resume", "finalize", "cancel", "delete"} {
		w := snapshotHTTPRequest(router, "POST", "/vm-snapshots/a/"+action, `{}`, true)
		if w.Code != 202 || service.action != action {
			t.Fatalf("action %s status %d", action, w.Code)
		}
	}
	service.err = fmt.Errorf("native failure NON_SECRET_INTERNAL_SENTINEL")
	for _, r := range []struct{ method, path, body string }{{"POST", "/vm-snapshots", `{"name":"a"}`}, {"GET", "/vm-snapshots", ""}, {"GET", "/vm-snapshots/a", ""}, {"POST", "/vm-snapshots/a/halt", `{}`}} {
		w := snapshotHTTPRequest(router, r.method, r.path, r.body, true)
		if w.Code != 500 || strings.Contains(w.Body.String(), "NON_SECRET_INTERNAL_SENTINEL") {
			t.Fatal("upstream error escaped HTTP boundary")
		}
	}
}

// capability_id: rainbond.vm-restore.native-platform-convergence
func TestSnapshotHTTPFixedDomainErrorCodes(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{{vmrestore.ErrSnapshotInvalid, 400, "vm_snapshot_invalid_request"}, {vmrestore.ErrInternalAttribute, 403, "vm_snapshot_forbidden"}, {vmrestore.ErrCorrupt, 409, "vm_restore_conflict"}, {vmrestore.ErrSnapshotConfig, 422, "vm_snapshot_config_unsupported"}, {vmrestore.ErrSnapshotIncomplete, 422, "vm_snapshot_incomplete"}, {vmrestore.ErrSnapshotPending, 409, "vm_snapshot_pending"}, {vmrestore.ErrNativeRestoreConfigurationChanged, 422, "vm_restore_native_configuration_changed"}} {
		w := httptest.NewRecorder()
		snapshotHTTPError(w, httptest.NewRequest("GET", "/", nil), fmt.Errorf("upstream NON_SECRET_INTERNAL_SENTINEL: %w", tc.err))
		var body struct {
			Msg string `json:"msg"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		if w.Code != tc.status || body.Msg != tc.code || strings.Contains(w.Body.String(), "NON_SECRET_INTERNAL_SENTINEL") {
			t.Fatalf("fixed error changed: %d %s", w.Code, w.Body.String())
		}
	}
}

type snapshotHTTPDBManager struct {
	db.Manager
	database *gorm.DB
}

func (m snapshotHTTPDBManager) DB() *gorm.DB { return m.database }
func (m snapshotHTTPDBManager) TenantDao() dbdao.TenantDao {
	return &mysqldao.TenantDaoImpl{DB: m.database}
}
func (m snapshotHTTPDBManager) TenantServiceDao() dbdao.TenantServiceDao {
	return &mysqldao.TenantServicesDaoImpl{DB: m.database}
}
func TestSnapshotHTTPRealFacadeEnforcesDBScopeBeforeNativeDependency(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "snapshot-http.db"))
	if err != nil {
		t.Fatal(err)
	}
	database.LogMode(false)
	previous := db.GetManager()
	db.SetTestManager(snapshotHTTPDBManager{database: database})
	t.Cleanup(func() { db.SetTestManager(previous); database.Close() })
	if err := database.AutoMigrate(&dbmodel.Tenants{}, &dbmodel.TenantServices{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&dbmodel.Tenants{UUID: "tenant", Namespace: "team"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&dbmodel.TenantServices{TenantID: "tenant", ServiceID: "service", ExtendMethod: string(dbmodel.ServiceTypeVM)}).Error; err != nil {
		t.Fatal(err)
	}
	router := snapshotHTTPRouter(&VMSnapshotController{service: &handler.ServiceAction{}})
	for _, r := range []struct{ method, path, body string }{{"POST", "/vm-snapshots", `{"name":"snapshot-a"}`}, {"GET", "/vm-snapshots", ""}, {"GET", "/vm-snapshots/a", ""}, {"POST", "/vm-snapshots/a/halt", `{}`}, {"DELETE", "/vm-snapshots/a", `{}`}} {
		// A real facade reaches DB scope resolution, then the unavailable external native boundary.
		w := snapshotHTTPRequest(router, r.method, r.path, r.body, true)
		if w.Code != 503 || !strings.Contains(w.Body.String(), "vm_snapshot_dependency_unavailable") {
			t.Fatalf("real facade dependency status=%d", w.Code)
		}
		request := httptest.NewRequest(r.method, r.path, strings.NewReader(r.body))
		ctx := context.WithValue(request.Context(), ctxutil.ContextKey("service_id"), "service")
		ctx = context.WithValue(ctx, ctxutil.ContextKey("tenant_id"), "foreign")
		request = request.WithContext(ctx)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, request)
		if w.Code != 403 {
			t.Fatalf("DB tenant mismatch status=%d", w.Code)
		}
	}
}
