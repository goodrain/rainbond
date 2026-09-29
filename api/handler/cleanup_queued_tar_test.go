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
