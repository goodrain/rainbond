package handler

import (
	"errors"
	"testing"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	ktesting "k8s.io/client-go/testing"
)

type meshTenantFixture struct{ TenantHandler }

func (meshTenantFixture) GetTenantsByUUID(string) (*model.Tenants, error) {
	return &model.Tenants{Namespace: "team"}, nil
}

// capability_id: rainbond.cleanup.service-mesh-mutation-admission
func TestServiceMeshMutationsHoldCleanupAdmission(t *testing.T) {
	for _, method := range []string{"create", "update"} {
		for _, scenario := range []string{"blocked", "success", "uncertain", "draining", "db-failed"} {
			t.Run(method+"/"+scenario, func(t *testing.T) {
				oldTenant := defaultTenantHandler
				defaultTenantHandler = meshTenantFixture{}
				defer func() { defaultTenantHandler = oldTenant }()
				database := namespaceAdmissionDB(t)
				if err := database.AutoMigrate(&model.K8sResource{}).Error; err != nil {
					t.Fatal(err)
				}
				old := db.GetManager()
				db.SetTestManager(testManager{Manager: workloadTestManager{database: database}, tenantDao: &testTenantDao{tenant: &model.Tenants{Namespace: "team"}}})
				defer db.SetTestManager(old)
				app := &model.Application{AppID: "application", K8sApp: "app", TenantID: "tenant"}
				if method == "update" {
					if err := database.Create(&model.K8sResource{AppID: app.AppID, Name: app.K8sApp, Kind: "ServiceMesh", Content: "old"}).Error; err != nil {
						t.Fatal(err)
					}
				}
				deletion := guard.CoordinationRequest{StorageID: "store", Generation: "one", OperationID: "selected", Owner: "test", Kind: "delete", Scope: "app", Target: "manifest", Fingerprint: "selection"}
				if scenario == "blocked" {
					if _, err := guard.AcquireOperation(database, deletion); err != nil {
						t.Fatal(err)
					}
				}
				client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
				calls := 0
				for _, verb := range []string{"create", "update"} {
					client.PrependReactor(verb, "servicemeshes", func(ktesting.Action) (bool, runtime.Object, error) {
						calls++
						if _, err := guard.AcquireOperation(database, deletion); err != guard.ErrCoordinationBusy {
							t.Error("CR mutation outside producer admission", err)
						}
						if scenario == "uncertain" {
							return true, nil, errors.New("lost response")
						}
						if scenario == "draining" && calls == 1 {
							gc := guard.CoordinationRequest{StorageID: "store", Generation: "one", OperationID: "gc", Owner: "test", Kind: "gc", Scope: "*", Fingerprint: "manual"}
							if _, err := guard.RequestMaintenance(database, gc); err != nil {
								t.Fatal(err)
							}
						}
						if scenario == "db-failed" && calls == 1 {
							if err := database.DropTable(&model.K8sResource{}).Error; err != nil {
								t.Fatal(err)
							}
						}
						return false, nil, nil
					})
				}
				handler := &ApplicationAction{dynamicClient: client}
				var err error
				if method == "create" {
					_, err = handler.CreateServiceMeshCR(app, "native")
				} else {
					_, err = handler.UpdateServiceMeshCR(app, "native")
				}
				if scenario == "blocked" {
					if calls != 0 || err == nil {
						t.Fatal("cleanup did not block CR mutation", calls, err)
					}
					return
				}
				if scenario == "uncertain" || scenario == "db-failed" {
					if err == nil {
						t.Fatal("lost response accepted")
					}
				} else if err != nil {
					t.Fatal(err)
				}
				var producers []model.CleanupOperation
				if err := database.Where("kind = ?", "producer").Find(&producers).Error; err != nil {
					t.Fatal(err)
				}
				expected := "finished"
				if scenario == "uncertain" || scenario == "db-failed" {
					expected = "uncertain"
				}
				if len(producers) != 1 || producers[0].State != expected {
					t.Fatal("incorrect original producer outcome", producers)
				}
			})
		}
	}
}
