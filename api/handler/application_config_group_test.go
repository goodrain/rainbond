package handler

import (
	"path/filepath"
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	mysqldao "github.com/goodrain/rainbond/db/mysql/dao"
	"github.com/goodrain/rainbond/worker/appm/conversion"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type configGroupBindingTestManager struct {
	db.Manager
	database *gorm.DB
}

func (m configGroupBindingTestManager) Begin() *gorm.DB { return m.database.Begin() }
func (m configGroupBindingTestManager) TenantServiceDao() dao.TenantServiceDao {
	return &mysqldao.TenantServicesDaoImpl{DB: m.database}
}
func (m configGroupBindingTestManager) AppConfigGroupDao() dao.AppConfigGroupDao {
	return m.AppConfigGroupDaoTransactions(m.database)
}
func (m configGroupBindingTestManager) AppConfigGroupDaoTransactions(tx *gorm.DB) dao.AppConfigGroupDao {
	return &mysqldao.AppConfigGroupDaoImpl{DB: tx}
}
func (m configGroupBindingTestManager) AppConfigGroupServiceDao() dao.AppConfigGroupServiceDao {
	return m.AppConfigGroupServiceDaoTransactions(m.database)
}
func (m configGroupBindingTestManager) AppConfigGroupServiceDaoTransactions(tx *gorm.DB) dao.AppConfigGroupServiceDao {
	return &mysqldao.AppConfigGroupServiceDaoImpl{DB: tx}
}
func (m configGroupBindingTestManager) AppConfigGroupItemDao() dao.AppConfigGroupItemDao {
	return m.AppConfigGroupItemDaoTransactions(m.database)
}
func (m configGroupBindingTestManager) AppConfigGroupItemDaoTransactions(tx *gorm.DB) dao.AppConfigGroupItemDao {
	return &mysqldao.AppConfigGroupItemDaoImpl{DB: tx}
}

// capability_id: rainbond.app-config-group.pending-component-bindings
func TestConfigGroupRetainsPendingComponentBindings(t *testing.T) {
	for _, operation := range []string{"create", "update"} {
		for _, existing := range []bool{false, true} {
			name := operation + "/pending-only"
			if existing {
				name = operation + "/mixed"
			}
			t.Run(name, func(t *testing.T) {
				database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "groups.db"))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { database.Close() })
				database.LogMode(false)
				if err := database.AutoMigrate(&dbmodel.TenantServices{}, &dbmodel.ApplicationConfigGroup{}, &dbmodel.ConfigGroupService{}, &dbmodel.ConfigGroupItem{}).Error; err != nil {
					t.Fatal(err)
				}
				manager := configGroupBindingTestManager{database: database}
				old := db.GetManager()
				db.SetTestManager(manager)
				t.Cleanup(func() { db.SetTestManager(old) })
				ids := []string{"pending", ""}
				if existing {
					if err := database.Create(&dbmodel.TenantServices{ServiceID: "ready", ServiceAlias: "gr-ready", AppID: "app"}).Error; err != nil {
						t.Fatal(err)
					}
					ids = append(ids, "ready", "pending")
				}
				items := []apimodel.ConfigItem{{ItemKey: "APP_NAME", ItemValue: "${_SERVICE_NAME}"}}
				action := &ApplicationAction{}
				var response *apimodel.ApplicationConfigGroupResp
				if operation == "create" {
					response, err = action.AddConfigGroup("app", &apimodel.ApplicationConfigGroup{ConfigGroupName: "java", Enable: true, DeployType: "env", ServiceIDs: ids, ConfigItems: items})
				} else {
					if err := database.Create(&dbmodel.ApplicationConfigGroup{AppID: "app", ConfigGroupName: "java", Enable: true, DeployType: "env"}).Error; err != nil {
						t.Fatal(err)
					}
					if err := database.Create(&dbmodel.ConfigGroupService{AppID: "app", ConfigGroupName: "java", ServiceID: "removed"}).Error; err != nil {
						t.Fatal(err)
					}
					response, err = action.UpdateConfigGroup("app", "java", &apimodel.UpdateAppConfigGroupReq{Enable: true, ServiceIDs: ids, ConfigItems: items})
				}
				if err != nil {
					t.Fatal(err)
				}
				got := map[string]string{}
				for _, binding := range response.Services {
					if _, duplicate := got[binding.ServiceID]; duplicate {
						t.Fatalf("duplicate binding for %s", binding.ServiceID)
					}
					got[binding.ServiceID] = binding.ServiceAlias
				}
				if _, ok := got["pending"]; !ok {
					t.Fatalf("%s discarded component before creation: %v", operation, got)
				}
				wantCount := 1
				if existing {
					wantCount++
					if got["ready"] != "gr-ready" {
						t.Fatal("existing component alias lost")
					}
				}
				if len(got) != wantCount {
					t.Fatalf("unexpected bindings: %v", got)
				}
				// Completing component creation must make the saved group discoverable,
				// without requiring the user to save the configuration group again.
				if err := database.Create(&dbmodel.TenantServices{ServiceID: "pending", ServiceAlias: "gr-pending", AppID: "app"}).Error; err != nil {
					t.Fatal(err)
				}
				groups, err := manager.AppConfigGroupDao().ListByServiceID("pending")
				if err != nil {
					t.Fatal(err)
				}
				if len(groups) != 1 || groups[0].ConfigGroupName != "java" {
					t.Fatalf("group missing after component creation: %v", groups)
				}
				for _, appID := range []string{"app", "other-app"} {
					if err := database.Model(&dbmodel.TenantServices{}).Where("service_id = ?", "pending").Update("app_id", appID).Error; err != nil {
						t.Fatal(err)
					}
					as := &v1.AppService{}
					as.ServiceID = "pending"
					as.AppID = appID
					as.SetTenant(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-team"}})
					if err := conversion.TenantServiceConfigGroup(as, manager); err != nil {
						t.Fatal(err)
					}
					secrets := as.GetEnvVarSecrets(false)
					if appID != "app" {
						if len(secrets) != 0 {
							t.Fatal("pending binding exposed configuration to another application")
						}
						continue
					}
					if len(secrets) != 1 || string(secrets[0].Data["APP_NAME"]) != "${_SERVICE_NAME}" {
						t.Fatalf("saved binding did not reach worker after component creation: %v", secrets)
					}
				}
				if _, err := action.UpdateConfigGroup("app", "java", &apimodel.UpdateAppConfigGroupReq{Enable: true, ConfigItems: items}); err != nil {
					t.Fatal(err)
				}
				bindings, err := manager.AppConfigGroupServiceDao().GetConfigGroupServicesByID("app", "java")
				if err != nil {
					t.Fatal(err)
				}
				if len(bindings) != 0 {
					t.Fatalf("clearing selection left stale pending bindings: %v", bindings)
				}

			})
		}
	}
}
