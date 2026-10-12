package client

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	guard "github.com/goodrain/rainbond/pkg/cleanup"
	"github.com/jinzhu/gorm"
	"google.golang.org/grpc"
)

type queueTestDB struct {
	db.Manager
	database *gorm.DB
}

func (m queueTestDB) DB() *gorm.DB { return m.database }

type reservedQueue struct {
	pb.TaskQueueClient
	send func(*pb.EnqueueRequest) (*pb.TaskReply, error)
}

func (q reservedQueue) Enqueue(_ context.Context, r *pb.EnqueueRequest, _ ...grpc.CallOption) (*pb.TaskReply, error) {
	return q.send(r)
}

// capability_id: rainbond.cleanup.native-queue-reservation
func TestNativeMQTaskReservesStableIdentityBeforeNetwork(t *testing.T) {
	for _, kind := range []string{"build_from_source_code", "import_app", "backup_apps_restore", "build_from_image", "build_from_vm", "plugin_image_build", "plugin_dockerfile_build", "share-image"} {
		t.Run(kind, func(t *testing.T) {
			database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "queue.db"))
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
			db.SetTestManager(queueTestDB{database: database})
			defer db.SetTestManager(old)
			calls := 0
			client := &mqClient{ctx: context.Background(), TaskQueueClient: reservedQueue{send: func(r *pb.EnqueueRequest) (*pb.TaskReply, error) {
				calls++
				if r.Message.TaskId == "" {
					t.Fatal("task identity allocated after producer admission")
				}
				raw, _ := json.Marshal(map[string]string{"event_id": "owned"})
				if string(r.Message.TaskBody) != string(raw) {
					t.Fatal("task body changed")
				}
				var rows []model.CleanupOperation
				if err := database.Find(&rows).Error; err != nil || len(rows) != 1 || rows[0].State != "queued" {
					t.Fatal("queue called without committed protection", err)
				}
				expected := guard.QueuedNativeRequest(guard.StoreIdentity{StorageID: binding.StorageID, Generation: binding.Generation}, guard.QueuedTaskKind(kind), r.Message.TaskId, r.Message.TaskBody)
				if rows[0].OperationID != expected.OperationID || rows[0].Fingerprint != expected.Fingerprint {
					t.Fatal("consumer identity does not match queued record")
				}
				return nil, errors.New("fixture lost acknowledgement")
			}}}
			if err := client.SendBuilderTopic(TaskStruct{Topic: BuilderTopic, TaskType: kind, TaskBody: map[string]string{"event_id": "owned"}}); err == nil || calls != 1 {
				t.Fatal("lost enqueue result hidden")
			}
			var count int
			database.Model(&model.CleanupOperation{}).Where("state = ?", "queued").Count(&count)
			if count != 1 {
				t.Fatal("lost queue acknowledgement released protection")
			}
		})
	}
}

func TestNonNativeMQMessagesKeepExistingDelivery(t *testing.T) {
	client := &mqClient{ctx: context.Background(), TaskQueueClient: reservedQueue{send: func(r *pb.EnqueueRequest) (*pb.TaskReply, error) {
		if r.Message.TaskId != "" {
			t.Fatal("unrelated MQ identity changed")
		}
		return &pb.TaskReply{}, nil
	}}}
	for _, kind := range []string{"warmup", "service_check", "load-tar-image", "rolling_upgrade"} {
		if err := client.SendBuilderTopic(TaskStruct{Topic: BuilderTopic, TaskType: kind, TaskBody: map[string]string{"fixture": "owned"}}); err != nil {
			t.Fatal(err)
		}
	}
}
