package handler

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	ktesting "k8s.io/client-go/testing"
)

func namespaceAdmissionDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "ns.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	database.LogMode(false)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := guard.RegisterStorage(database, guard.StorageRegistration{StorageID: "store", Generation: "one", VolumeUID: "volume", RootPath: "/registry"}); err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", "store").Update("mode", "ready").Error; err != nil {
		t.Fatal(err)
	}
	return database
}
func TestNamespaceMutationsCannotBypassCleanupAdmission(t *testing.T) {
	for _, operation := range []string{"create", "update"} {
		for _, blocked := range []bool{false, true} {
			t.Run(operation+map[bool]string{false: "-uncertain", true: "-blocked"}[blocked], func(t *testing.T) {
				database := namespaceAdmissionDB(t)
				previous := db.GetManager()
				db.SetTestManager(testManager{Manager: workloadTestManager{database: database}, tenantDao: &testTenantDao{tenant: &model.Tenants{Namespace: "team"}}})
				defer db.SetTestManager(previous)
				deletion := guard.CoordinationRequest{StorageID: "store", Generation: "one", OperationID: "delete", Owner: "test", Kind: "delete", Scope: "app", Target: "manifest", Fingerprint: "selection"}
				if blocked {
					if _, err := guard.AcquireOperation(database, deletion); err != nil {
						t.Fatal(err)
					}
				}
				client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
				calls := 0
				client.PrependReactor(operation, "deployments", func(ktesting.Action) (bool, runtime.Object, error) {
					calls++
					if _, err := guard.AcquireOperation(database, deletion); err != guard.ErrCoordinationBusy {
						t.Error("Kubernetes mutation outside admission", err)
					}
					return true, nil, errors.New("uncertain fixture transport")
				})
				mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "apps", Version: "v1"}})
				mapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
				oldClient, oldMapper := nsResourceDynamicClient, nsResourceRESTMapper
				nsResourceDynamicClient = func() dynamic.Interface { return client }
				nsResourceRESTMapper = func() meta.RESTMapper { return mapper }
				defer func() { nsResourceDynamicClient = oldClient; nsResourceRESTMapper = oldMapper }()
				body := []byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: app\n  resourceVersion: '1'\n")
				if operation == "create" {
					_, _, _ = GetNsResourceHandler().CreateNsResource("team", "yaml", body)
				} else {
					_, _ = GetNsResourceHandler().UpdateNsResource("team", "apps", "v1", "deployments", "app", body)
				}
				want := 1
				if blocked {
					want = 0
				}
				if calls != want {
					t.Fatal("wrong external dispatch count", calls, want)
				}
				if !blocked {
					var rows []model.CleanupOperation
					if err := database.Where("kind = ?", "producer").Find(&rows).Error; err != nil || len(rows) != 1 || rows[0].State != "uncertain" {
						t.Fatal("uncertain effect lost protection", rows, err)
					}
				}
			})
		}
	}
}

func TestClusterMutationsCannotBypassCleanupAdmission(t *testing.T) {
	for _, operation := range []string{"create", "update"} {
		for _, blocked := range []bool{false, true} {
			t.Run(operation+map[bool]string{false: "-uncertain", true: "-blocked"}[blocked], func(t *testing.T) {
				database := namespaceAdmissionDB(t)
				previous := db.GetManager()
				db.SetTestManager(testManager{Manager: workloadTestManager{database: database}, tenantDao: &testTenantDao{tenant: &model.Tenants{Namespace: "team"}}})
				defer db.SetTestManager(previous)
				deletion := guard.CoordinationRequest{StorageID: "store", Generation: "one", OperationID: "delete", Owner: "test", Kind: "delete", Scope: "app", Target: "manifest", Fingerprint: "selection"}
				if blocked {
					if _, err := guard.AcquireOperation(database, deletion); err != nil {
						t.Fatal(err)
					}
				}
				client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
				calls := 0
				client.PrependReactor(operation, "widgets", func(ktesting.Action) (bool, runtime.Object, error) {
					calls++
					if _, err := guard.AcquireOperation(database, deletion); err != guard.ErrCoordinationBusy {
						t.Error("Kubernetes mutation outside admission", err)
					}
					return true, nil, errors.New("uncertain fixture transport")
				})
				mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "example.test", Version: "v1"}})
				mapper.Add(schema.GroupVersionKind{Group: "example.test", Version: "v1", Kind: "Widget"}, meta.RESTScopeNamespace)
				oldClient, oldMapper := clusterResourceDynamicClient, nsResourceRESTMapper
				clusterResourceDynamicClient = func() dynamic.Interface { return client }
				nsResourceRESTMapper = func() meta.RESTMapper { return mapper }
				defer func() { clusterResourceDynamicClient = oldClient; nsResourceRESTMapper = oldMapper }()
				body := []byte("apiVersion: example.test/v1\nkind: Widget\nmetadata:\n  name: app\n  resourceVersion: '1'\n")
				if operation == "create" {
					_, _ = GetClusterResourceHandler().CreateResource("example.test", "v1", "widgets", body)
				} else {
					_, _ = GetClusterResourceHandler().UpdateResource("example.test", "v1", "widgets", "app", body)
				}
				want := 1
				if blocked {
					want = 0
				}
				if calls != want {
					t.Fatal("wrong external dispatch count", calls, want)
				}
				if !blocked {
					var rows []model.CleanupOperation
					if err := database.Where("kind = ?", "producer").Find(&rows).Error; err != nil || len(rows) != 1 || rows[0].State != "uncertain" {
						t.Fatal("uncertain effect lost protection", rows, err)
					}
				}
			})
		}
	}
}
