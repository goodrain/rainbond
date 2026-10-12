package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"

	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	"github.com/goodrain/rainbond/db/model"
	mysqldao "github.com/goodrain/rainbond/db/mysql/dao"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

type workloadTestManager struct {
	db.Manager
	database *gorm.DB
}

func (m workloadTestManager) DB() *gorm.DB { return m.database }
func (m workloadTestManager) K8sResourceDao() dbdao.K8sResourceDao {
	return &mysqldao.K8sResourceDaoImpl{DB: m.database}
}
func (m workloadTestManager) K8sResourceDaoTransactions(tx *gorm.DB) dbdao.K8sResourceDao {
	return &mysqldao.K8sResourceDaoImpl{DB: tx}
}

func TestYAMLApplyAdmissionCoversRequestsAndPartialResults(t *testing.T) {
	for _, actionName := range []string{"add", "update", "sync"} {
		for _, scenario := range []string{"success", "partial-rejection", "uncertain", "invalid-yaml"} {
			if actionName == "update" && scenario == "partial-rejection" {
				continue
			}
			t.Run(actionName+"/"+scenario, func(t *testing.T) {
				database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "apply.db"))
				if err != nil {
					t.Fatal(err)
				}
				defer database.Close()
				database.LogMode(false)
				if err := database.AutoMigrate(&model.K8sResource{}, &model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
					t.Fatal(err)
				}
				if err := guard.RegisterStorage(database, guard.StorageRegistration{StorageID: "store", Generation: "one", VolumeUID: "volume", RootPath: "/registry"}); err != nil {
					t.Fatal(err)
				}
				if err := database.Model(&model.CleanupStorage{}).Where("storage_id = ?", "store").Update("mode", "ready").Error; err != nil {
					t.Fatal(err)
				}
				previous := db.GetManager()
				db.SetTestManager(workloadTestManager{database: database})
				defer db.SetTestManager(previous)
				requests := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests++
					deletion := guard.CoordinationRequest{StorageID: "store", Generation: "one", OperationID: "delete", Owner: "test", Kind: "delete", Scope: "app", Target: "manifest", Fingerprint: "selected"}
					if _, err := guard.AcquireOperation(database, deletion); err != guard.ErrCoordinationBusy {
						t.Error("mutation not protected", err)
					}
					var body map[string]interface{}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					w.Header().Set("Content-Type", "application/json")
					if scenario == "partial-rejection" && requests == 2 {
						w.WriteHeader(400)
						w.Write([]byte(`{"apiVersion":"v1","kind":"Status","status":"Failure","reason":"BadRequest","message":"fixture rejection","code":400}`))
						return
					}
					if scenario == "uncertain" {
						w.WriteHeader(500)
						w.Write([]byte(`{"apiVersion":"v1","kind":"Status","status":"Failure","reason":"InternalError","code":500}`))
						return
					}
					w.WriteHeader(201)
					json.NewEncoder(w).Encode(body)
				}))
				defer server.Close()
				mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Group: "apps", Version: "v1"}})
				mapper.Add(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, meta.RESTScopeNamespace)
				action := &clusterAction{config: &rest.Config{Host: server.URL}, mapper: mapper}
				yaml := "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: first\nspec:\n  template:\n    spec:\n      containers:\n      - name: app\n        image: goodrain.me/app:v1\n"
				if scenario == "partial-rejection" {
					yaml += "---\napiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: second\n"
				}
				if scenario == "invalid-yaml" {
					yaml = "invalid: ["
				}
				var failure *util.APIHandleError
				switch actionName {
				case "add":
					_, failure = action.AddAppK8SResource(context.Background(), "team", "app", yaml)
				case "update":
					if err := database.Create(&model.K8sResource{AppID: "app", Name: "first", Kind: "Deployment", Content: "original"}).Error; err != nil {
						t.Fatal(err)
					}
					_, failure = action.UpdateAppK8SResource(context.Background(), "team", "app", "first", yaml, "Deployment")
				case "sync":
					req := &apimodel.SyncResources{AppID: "app"}
					for _, document := range strings.Split(yaml, "---\n") {
						req.K8sResources = append(req.K8sResources, apimodel.HandleResource{AppID: "app", Name: "first", Namespace: "team", Kind: "Deployment", ResourceYaml: document})
					}
					_, failure = action.SyncAppK8SResources(context.Background(), req)
				}
				if failure != nil && scenario != "invalid-yaml" {
					t.Fatal(failure)
				}
				if scenario == "invalid-yaml" && requests != 0 {
					t.Fatal("invalid YAML reached Kubernetes")
				}
				var operations []model.CleanupOperation
				if err := database.Find(&operations).Error; err != nil || len(operations) != 1 {
					t.Fatal(operations, err)
				}
				expected := "finished"
				if scenario == "uncertain" {
					expected = "uncertain"
				}
				if operations[0].State != expected {
					t.Fatal("wrong durable outcome", operations[0].State, expected)
				}
			})
		}
	}
}
