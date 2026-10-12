package handler

import (
	"context"
	"errors"
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	"github.com/goodrain/rainbond/db/model"
	mysqldao "github.com/goodrain/rainbond/db/mysql/dao"
	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	rainfake "github.com/goodrain/rainbond/pkg/generated/clientset/versioned/fake"
	"github.com/jinzhu/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

type helmAdmissionManager struct{ workloadTestManager }

func (m helmAdmissionManager) ApplicationDaoTransactions(tx *gorm.DB) dbdao.ApplicationDao {
	return &mysqldao.ApplicationDaoImpl{DB: tx}
}
func (m helmAdmissionManager) TenantDao() dbdao.TenantDao { return helmAdmissionTenant{} }

type helmAdmissionTenant struct{ dbdao.TenantDao }

func (helmAdmissionTenant) GetTenantByUUID(string) (*model.Tenants, error) {
	return &model.Tenants{Namespace: "team"}, nil
}

// capability_id: rainbond.cleanup.helmapp-mutation-admission
func TestHelmAppMutationsHoldCleanupAdmission(t *testing.T) {
	for _, method := range []string{"create", "update", "install"} {
		for _, scenario := range []string{"blocked", "success", "uncertain"} {
			t.Run(method+"/"+scenario, func(t *testing.T) {
				database := namespaceAdmissionDB(t)
				if err := database.AutoMigrate(&model.Application{}).Error; err != nil {
					t.Fatal(err)
				}
				old := db.GetManager()
				db.SetTestManager(helmAdmissionManager{workloadTestManager{database: database}})
				defer db.SetTestManager(old)
				oldTenant := defaultTenantHandler
				defaultTenantHandler = meshTenantFixture{}
				defer func() { defaultTenantHandler = oldTenant }()
				app := &model.Application{AppID: "application", TenantID: "tenant", AppName: "app", K8sApp: "app", AppType: apimodel.AppTypeHelm}
				if method != "create" {
					if err := database.Create(app).Error; err != nil {
						t.Fatal(err)
					}
				}
				deletion := guard.CoordinationRequest{StorageID: "store", Generation: "one", OperationID: "selected", Owner: "test", Kind: "delete", Scope: "app", Target: "manifest", Fingerprint: "selection"}
				if scenario == "blocked" {
					if _, err := guard.AcquireOperation(database, deletion); err != nil {
						t.Fatal(err)
					}
				}
				objects := []runtime.Object{}
				if method != "create" {
					objects = append(objects, &v1alpha1.HelmApp{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "team"}})
				}
				client := rainfake.NewSimpleClientset(objects...)
				calls := 0
				for _, verb := range []string{"create", "update"} {
					client.PrependReactor(verb, "helmapps", func(ktesting.Action) (bool, runtime.Object, error) {
						calls++
						var producers []model.CleanupOperation
						if err := database.Where("kind = ? AND state = ?", "producer", "active").Find(&producers).Error; err != nil || len(producers) != 1 {
							t.Error("HelmApp write outside producer admission", err, len(producers))
						}
						if scenario == "uncertain" {
							return true, nil, errors.New("lost response")
						}
						return false, nil, nil
					})
				}
				action := &ApplicationAction{rainbondClient: client, kubeClient: kubefake.NewSimpleClientset()}
				var err error
				switch method {
				case "create":
					_, err = action.CreateApp(context.Background(), &apimodel.Application{TenantID: "tenant", AppName: "app", K8sApp: "app", AppType: apimodel.AppTypeHelm})
				case "update":
					_, err = action.UpdateApp(context.Background(), app, apimodel.UpdateAppRequest{K8sApp: "app", Version: "2"})
				case "install":
					err = action.Install(context.Background(), app, []string{"image.tag=2"})
				}
				if scenario == "blocked" {
					if err == nil || calls != 0 {
						t.Fatal("cleanup did not block HelmApp mutation", err, calls)
					}
					return
				}
				if calls != 1 || (err == nil) != (scenario == "success") {
					t.Fatal("unexpected execution", calls, err)
				}
				var producers []model.CleanupOperation
				if err := database.Where("kind = ?", "producer").Find(&producers).Error; err != nil {
					t.Fatal(err)
				}
				state := "finished"
				if scenario == "uncertain" {
					state = "uncertain"
				}
				if len(producers) != 1 || producers[0].State != state {
					t.Fatal("wrong durable outcome", producers)
				}
			})
		}
	}
}
