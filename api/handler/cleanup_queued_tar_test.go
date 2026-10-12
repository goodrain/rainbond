package handler

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/mq/client"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
)

type queuedTarDB struct {
	db.Manager
	database *gorm.DB
}

func (d queuedTarDB) DB() *gorm.DB { return d.database }

type queuedTarMQ struct {
	client.MQClient
	send func(client.TaskStruct) error
}

func (q queuedTarMQ) SendBuilderTopic(task client.TaskStruct) error { return q.send(task) }

// capability_id: rainbond.cleanup.tar-admission-before-enqueue
func TestTarAPIReservesBeforeEnqueueAndRetainsLostAcknowledgement(t *testing.T) {
	for _, lost := range []bool{false, true} {
		database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "enqueue.db"))
		if err != nil {
			t.Fatal(err)
		}
		if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
			t.Fatal(err)
		}
		binding, err := guard.ProvisionRegistryStorage(database, "owned-volume", "/registry")
		if err != nil {
			t.Fatal(err)
		}
		old := db.GetManager()
		db.SetTestManager(queuedTarDB{database: database})
		called := 0
		handler := &TarImageHandle{MQClient: queuedTarMQ{send: func(task client.TaskStruct) error {
			called++
			body, err := json.Marshal(task.TaskBody)
			if err != nil {
				t.Fatal(err)
			}
			input := task.TaskBody.(map[string]interface{})
			expected := guard.NativeTarRequest(guard.StoreIdentity{StorageID: binding.StorageID, Generation: binding.Generation}, input["load_id"].(string), body)
			var op model.CleanupOperation
			if err := database.Where("operation_id = ?", expected.OperationID).First(&op).Error; err != nil || op.State != "queued" || op.Fingerprint != expected.Fingerprint {
				t.Fatal("MQ called before exact-body reservation", err)
			}
			if lost {
				return errors.New("fixture response lost")
			}
			return nil
		}}}
		_, apiErr := handler.LoadTarImage("tenant", apimodel.LoadTarImageReq{EventID: "owned", TarFilePath: "owned.tar"})
		db.SetTestManager(old)
		if called != 1 || (apiErr != nil) != lost {
			t.Fatal("incorrect enqueue result")
		}
		var count int
		if err := database.Model(&model.CleanupOperation{}).Where("state = ?", "queued").Count(&count).Error; err != nil || count != 1 {
			t.Fatal("queued evidence released before execution", err)
		}
		database.Close()
	}
}

// capability_id: rainbond.cleanup.package-check-before-enqueue
func TestServiceCheckReservesOnlyUploadedPackageSources(t *testing.T) {
	for _, kind := range []string{"package_build", "sourcecode-pkg", "sourcecode-git", "docker-compose", "docker-run-tar", "docker-run-image"} {
		t.Run(kind, func(t *testing.T) {
			database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "check.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}).Error; err != nil {
				t.Fatal(err)
			}
			binding, err := guard.ProvisionRegistryStorage(database, "owned", "/registry")
			if err != nil {
				t.Fatal(err)
			}
			old := db.GetManager()
			db.SetTestManager(queuedTarDB{database: database})
			defer db.SetTestManager(old)
			req := &apimodel.ServiceCheckStruct{}
			req.Body.SourceType = kind
			req.Body.EventID = "owned"
			switch kind {
			case "sourcecode-pkg":
				req.Body.SourceType = "sourcecode"
				req.Body.SourceBody = `{"server_type":"pkg"}`
			case "sourcecode-git":
				req.Body.SourceType = "sourcecode"
				req.Body.SourceBody = `{"server_type":"git"}`
			case "docker-run-tar":
				req.Body.SourceType = "docker-run"
				req.Body.SourceBody = "event owned"
			case "docker-run-image":
				req.Body.SourceType = "docker-run"
				req.Body.SourceBody = "docker run nginx"
			}
			needs := kind != "sourcecode-git" && kind != "docker-run-image"
			calls := 0
			h := &ServiceAction{MQClient: queuedTarMQ{send: func(task client.TaskStruct) error {
				calls++
				raw, err := json.Marshal(task.TaskBody)
				if err != nil {
					t.Fatal(err)
				}
				expected := guard.QueuedNativeRequest(guard.StoreIdentity{StorageID: binding.StorageID, Generation: binding.Generation}, "service-check", req.Body.CheckUUID, raw)
				var rows []model.CleanupOperation
				if err := database.Find(&rows).Error; err != nil {
					t.Fatal(err)
				}
				if needs {
					if len(rows) != 1 || rows[0].OperationID != expected.OperationID || rows[0].Fingerprint != expected.Fingerprint || rows[0].State != "queued" {
						t.Fatal("package check enqueued without matching reservation")
					}
				} else if len(rows) != 0 {
					t.Fatal("ordinary source check changed behavior")
				}
				return nil
			}}}
			if _, _, err := h.ServiceCheck(req); err != nil || calls != 1 {
				t.Fatal("check failed", err)
			}
		})
	}
}
