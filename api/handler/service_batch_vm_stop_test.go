package handler

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/golang/mock/gomock"
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	mysqldao "github.com/goodrain/rainbond/db/mysql/dao"
	workerclient "github.com/goodrain/rainbond/worker/client"
	wmodel "github.com/goodrain/rainbond/worker/discover/model"
	workerpb "github.com/goodrain/rainbond/worker/server/pb"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kv "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
)

type vmBatchStopManager struct {
	db.Manager
	database *gorm.DB
}

func (m vmBatchStopManager) DB() *gorm.DB { return m.database }
func (m vmBatchStopManager) TenantServiceDao() dao.TenantServiceDao {
	return &mysqldao.TenantServicesDaoImpl{DB: m.database}
}
func (m vmBatchStopManager) ServiceEventDao() dao.EventDao {
	return &mysqldao.EventDaoImpl{DB: m.database}
}
func (m vmBatchStopManager) ServiceEventDaoTransactions(tx *gorm.DB) dao.EventDao {
	return &mysqldao.EventDaoImpl{DB: tx}
}
func (m vmBatchStopManager) TenantServiceRelationDao() dao.TenantServiceRelationDao {
	return &mysqldao.TenantServiceRelationDaoImpl{DB: m.database}
}

type vmBatchResourceClient struct{ workerpb.AppRuntimeSyncClient }

func (vmBatchResourceClient) GetTenantResource(context.Context, *workerpb.TenantRequest, ...grpc.CallOption) (*workerpb.TenantResource, error) {
	return &workerpb.TenantResource{}, nil
}

// capability_id: rainbond.vm-power.batch-stop-preserves-disks
func TestBatchStopUsesVMPowerAndKeepsContainerWorkerPath(t *testing.T) {
	testBatchVMPower(t, "stop")
}

// capability_id: rainbond.vm-power.batch-start-preserves-disks
func TestBatchStartUsesVMPowerAndKeepsContainerWorkerPath(t *testing.T) {
	testBatchVMPower(t, "start")
}

func testBatchVMPower(t *testing.T, action string) {
	t.Helper()
	for _, failure := range []string{"", "power", "lookup"} {
		name := "success"
		if failure != "" {
			name = failure + " failure"
		}
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "batch.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			database.LogMode(false)
			if err := database.AutoMigrate(&dbmodel.TenantServices{}, &dbmodel.ServiceEvent{}, &dbmodel.TenantServiceRelation{}).Error; err != nil {
				t.Fatal(err)
			}
			manager := vmBatchStopManager{database: database}
			for _, service := range []dbmodel.TenantServices{
				{ServiceID: "windows", TenantID: "tenant", ExtendMethod: "vm"},
				{ServiceID: "container", TenantID: "tenant", ExtendMethod: "stateless"},
			} {
				if err := database.Create(&service).Error; err != nil {
					t.Fatal(err)
				}
			}
			previousDB, previousService := db.GetManager(), defaultServieHandler
			db.SetTestManager(manager)
			defer db.SetTestManager(previousDB)
			defer func() { defaultServieHandler = previousService }()

			client := kubecli.NewMockKubevirtClient(ctrl)
			vmClient := kubecli.NewMockVirtualMachineInterface(ctrl)
			if failure != "lookup" {
				client.EXPECT().VirtualMachine("team").Return(vmClient)
			}
			var stopErr error
			if failure == "power" {
				stopErr = errors.New("power operation failed")
			}
			if failure == "lookup" {
				// A failed runtime lookup must not leave a newly created event running.
			} else if action == "stop" {
				vmClient.EXPECT().Stop(gomock.Any(), "windows-vm", gomock.Any()).Return(stopErr)
			} else {
				vmClient.EXPECT().Start(gomock.Any(), "windows-vm", gomock.Any()).Return(stopErr)
			}
			// No Delete expectation: deleting a VM would cascade to its DataVolumes/PVCs.
			defaultServieHandler = &ServiceAction{
				dbmanager:                  resourceSyncTestManager{},
				kubevirtClient:             client,
				syncVirtualMachineSpecHook: func(string) error { return nil },
				getVirtualMachineByServiceIDHook: func(serviceID string) (*kv.VirtualMachine, error) {
					if failure == "lookup" {
						return nil, errors.New("runtime lookup unavailable")
					}
					if serviceID != "windows" {
						t.Fatalf("unexpected VM power target: %s", serviceID)
					}
					status := kv.VirtualMachineStatusRunning
					if action == "start" {
						status = kv.VirtualMachineStatusStopped
					}
					return &kv.VirtualMachine{
						ObjectMeta: metav1.ObjectMeta{Name: "windows-vm", Namespace: "team", UID: "original-vm"},
						Status:     kv.VirtualMachineStatus{PrintableStatus: status},
					}, nil
				},
			}
			queue := &recordingMQClient{}
			batch := &BatchOperationHandler{operationHandler: &OperationHandler{mqCli: queue}, statusCli: &workerclient.AppRuntimeSyncClient{AppRuntimeSyncClient: vmBatchResourceClient{}}}
			requests := apimodel.BatchOpRequesters{
				&apimodel.ComponentStopReq{ComponentStartReq: apimodel.ComponentStartReq{ComponentOpGeneralReq: apimodel.ComponentOpGeneralReq{ServiceID: "windows", EventID: "vm-stop"}}},
				&apimodel.ComponentStopReq{ComponentStartReq: apimodel.ComponentStartReq{ComponentOpGeneralReq: apimodel.ComponentOpGeneralReq{ServiceID: "container", EventID: "container-stop"}}},
			}
			var result apimodel.BatchOpResult
			var operationErr error
			if action == "start" {
				requests = apimodel.BatchOpRequesters{
					&apimodel.ComponentStartReq{ComponentOpGeneralReq: apimodel.ComponentOpGeneralReq{ServiceID: "windows", EventID: "vm-start"}},
					&apimodel.ComponentStartReq{ComponentOpGeneralReq: apimodel.ComponentOpGeneralReq{ServiceID: "container", EventID: "container-start"}},
				}
				result, operationErr = batch.Start(context.Background(), &dbmodel.Tenants{UUID: "tenant", LimitMemory: 65536}, "tester", requests)
			} else {
				result, operationErr = batch.Stop(context.Background(), &dbmodel.Tenants{UUID: "tenant"}, "tester", requests)
			}
			if operationErr != nil {
				t.Fatal(operationErr)
			}
			if len(queue.tasks) != 1 || queue.tasks[0].TaskType != action {
				t.Fatalf("only the container may receive worker %s, got %#v", action, queue.tasks)
			}
			body, ok := queue.tasks[0].TaskBody.(*wmodel.StartTaskBody)
			if !ok || body.ServiceID != "container" {
				t.Fatalf("unexpected worker stop body: %#v", queue.tasks[0].TaskBody)
			}
			if len(result) != 2 || result[1].Status != apimodel.BatchOpResultItemStatusSuccess {
				t.Fatalf("container batch result changed: %#v", result)
			}
			wantResult, wantEvent := apimodel.BatchOpResultItemStatusSuccess, "success"
			if failure != "" {
				wantResult, wantEvent = apimodel.BatchOpResultItemStatusFailure, "failure"
			}
			if result[0].Status != wantResult || failure != "" && result[0].ErrMsg == "" {
				t.Fatalf("wrong VM batch outcome: %#v", result[0])
			}
			event, err := manager.ServiceEventDao().GetEventByEventID("vm-" + action)
			if err != nil || event.Status != wantEvent || event.FinalStatus != "complete" {
				t.Fatalf("VM event must close as %s, got %#v, %v", wantEvent, event, err)
			}
		})
	}
}
