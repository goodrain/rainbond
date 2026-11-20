package handler

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	workerclient "github.com/goodrain/rainbond/worker/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ktesting "k8s.io/client-go/testing"
	kv "kubevirt.io/api/core/v1"
)

func batchPowerRequest(action, serviceID string) apimodel.BatchOpRequesters {
	start := apimodel.ComponentStartReq{ComponentOpGeneralReq: apimodel.ComponentOpGeneralReq{ServiceID: serviceID, EventID: "batch-" + action}}
	if action == "start" {
		return apimodel.BatchOpRequesters{&start}
	}
	return apimodel.BatchOpRequesters{&apimodel.ComponentStopReq{ComponentStartReq: start}}
}

func installRestoredBatch(t *testing.T, action *ServiceAction) (*BatchOperationHandler, *recordingMQClient) {
	t.Helper()
	database := action.getDBManager().DB()
	if err := database.AutoMigrate(&dbmodel.TenantServiceRelation{}).Error; err != nil {
		t.Fatal(err)
	}
	previousDB, previousService := db.GetManager(), defaultServieHandler
	db.SetTestManager(vmBatchStopManager{database: database})
	defaultServieHandler = action
	t.Cleanup(func() { db.SetTestManager(previousDB); defaultServieHandler = previousService })
	queue := &recordingMQClient{}
	return &BatchOperationHandler{operationHandler: &OperationHandler{mqCli: queue}, statusCli: &workerclient.AppRuntimeSyncClient{AppRuntimeSyncClient: vmBatchResourceClient{}}}, queue
}

func runRestoredBatch(t *testing.T, batch *BatchOperationHandler, scope vmrestore.Scope, action string) apimodel.BatchOpResult {
	t.Helper()
	tenant := &dbmodel.Tenants{UUID: scope.TenantID, LimitMemory: 65536}
	var result apimodel.BatchOpResult
	var err error
	if action == "start" {
		result, err = batch.Start(context.Background(), tenant, "tester", batchPowerRequest(action, scope.ServiceID))
	} else {
		result, err = batch.Stop(context.Background(), tenant, "tester", batchPowerRequest(action, scope.ServiceID))
	}
	if err != nil || len(result) != 1 {
		t.Fatalf("batch %s: %#v, %v", action, result, err)
	}
	return result
}

// capability_id: rainbond.vm-power.batch-restored-event-lifecycle
func TestBatchRestoredVMPowerRetainsPendingAndUnknownUntilObserved(t *testing.T) {
	testBatchRestoredVMPower(t, []string{"pending", "write response lost"})
}

// capability_id: rainbond.vm-power.batch-post-submit-observation
func TestBatchRestoredVMPowerKeepsSubmittedObservationFailurePending(t *testing.T) {
	testBatchRestoredVMPower(t, []string{"post-write observation unavailable", "write response and outcome persistence unavailable"})
}

func testBatchRestoredVMPower(t *testing.T, modes []string) {
	t.Helper()
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			outcomeUnavailable := mode == "write response and outcome persistence unavailable"
			unknown := mode == "write response lost" || outcomeUnavailable
			a, store, resolver, scope, dyn := restoredPowerFixture(t)
			batch, queue := installRestoredBatch(t, a)
			vmGVR := schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachines"}
			vmiGVR := schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachineinstances"}
			writes := 0
			observationUnavailable := false
			dyn.PrependReactor("update", "virtualmachines", func(action ktesting.Action) (bool, runtime.Object, error) {
				writes++
				observationUnavailable = mode == "post-write observation unavailable"
				if unknown {
					object := action.(ktesting.UpdateAction).GetObject()
					if err := dyn.Tracker().Update(vmGVR, object.DeepCopyObject(), scope.Namespace); err != nil {
						t.Fatal(err)
					}
					if outcomeUnavailable {
						if err := a.getDBManager().DB().Exec(`CREATE TRIGGER reject_power_outcome BEFORE UPDATE ON component_k8s_attributes BEGIN SELECT RAISE(ABORT,'outcome persistence unavailable'); END`).Error; err != nil {
							t.Fatal(err)
						}
					}
					return true, nil, errors.New("write response unavailable")
				}
				return false, nil, nil
			})
			dyn.PrependReactor("list", "virtualmachineinstances", func(ktesting.Action) (bool, runtime.Object, error) {
				if observationUnavailable {
					observationUnavailable = false
					return true, nil, errors.New("observation temporarily unavailable")
				}
				return false, nil, nil
			})
			for _, action := range []string{"start", "stop"} {
				result := runRestoredBatch(t, batch, scope, action)
				var event dbmodel.ServiceEvent
				if err := a.getDBManager().DB().Where("event_id = ?", "batch-"+action).Take(&event).Error; err != nil {
					t.Fatal(err)
				}
				if event.ServiceID != scope.ServiceID || event.TenantID != scope.TenantID || event.TargetID != scope.ServiceID {
					t.Fatalf("batch event is not bound to restored VM scope: %#v", event)
				}
				if result[0].Status != apimodel.BatchOpResultItemStatusSuccess {
					t.Fatalf("accepted %s request reported failure: %#v", action, result[0])
				}
				if event.Status != "checking" || event.FinalStatus != "" {
					t.Fatalf("unobserved %s request was prematurely completed: %#v", action, event)
				}
				current, err := store.Read(scope, "")
				if err != nil || current.State.Admission == nil || unknown && !outcomeUnavailable && !current.State.Admission.OutcomeUnknown {
					t.Fatalf("pending write lost admission: %#v, %v", current.State.Admission, err)
				}
				if outcomeUnavailable {
					if err := a.getDBManager().DB().Exec(`DROP TRIGGER reject_power_outcome`).Error; err != nil {
						t.Fatal(err)
					}
				}
				vm, err := dyn.Resource(vmGVR).Namespace(scope.Namespace).Get(context.Background(), "vm", metav1.GetOptions{})
				if err != nil || string(vm.GetUID()) != "vm-uid" {
					t.Fatalf("VM identity changed: %v", err)
				}
				if action == "start" {
					var typed kv.VirtualMachine
					if err := runtime.DefaultUnstructuredConverter.FromUnstructured(vm.Object, &typed); err != nil {
						t.Fatal(err)
					}
					instance := &kv.VirtualMachineInstance{TypeMeta: metav1.TypeMeta{APIVersion: "kubevirt.io/v1", Kind: "VirtualMachineInstance"}, ObjectMeta: metav1.ObjectMeta{Name: "vm", Namespace: scope.Namespace, UID: "batch-instance", OwnerReferences: []metav1.OwnerReference{{Kind: "VirtualMachine", Name: "vm", UID: "vm-uid"}}}, Spec: typed.Spec.Template.Spec, Status: kv.VirtualMachineInstanceStatus{Phase: kv.Running}}
					raw, _ := json.Marshal(instance)
					object := &unstructured.Unstructured{}
					if err := object.UnmarshalJSON(raw); err != nil {
						t.Fatal(err)
					}
					if _, err := dyn.Resource(vmiGVR).Namespace(scope.Namespace).Create(context.Background(), object, metav1.CreateOptions{}); err != nil {
						t.Fatal(err)
					}
				} else if err := dyn.Resource(vmiGVR).Namespace(scope.Namespace).Delete(context.Background(), "vm", metav1.DeleteOptions{}); err != nil {
					t.Fatal(err)
				}
				if err := store.ObserveOrdinaryVMCompletion(context.Background(), resolver, dyn, scope); err != nil {
					t.Fatal(err)
				}
				if err := a.getDBManager().DB().Where("event_id = ?", event.EventID).Take(&event).Error; err != nil || event.Status != "success" || event.FinalStatus != "complete" {
					t.Fatalf("observed runtime did not finish batch event: %#v, %v", event, err)
				}
			}
			if len(queue.tasks) != 0 || writes != 2 {
				t.Fatalf("batch replayed accepted VM power writes: tasks=%#v writes=%d", queue.tasks, writes)
			}
		})
	}
}

// capability_id: rainbond.vm-power.batch-restored-event-lifecycle
func TestBatchRestoredVMPowerClosesRejectedEventWithoutMutatingFence(t *testing.T) {
	a, store, _, scope, dyn := restoredPowerFixture(t)
	batch, queue := installRestoredBatch(t, a)
	hash := strings.Repeat("c", 64)
	beforeState, err := store.Read(scope, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Mutate(scope, vmrestore.Expectation{ControlRevision: beforeState.State.ControlRevision, BindingGeneration: beforeState.State.BindingGeneration, OperationID: "busy-restore", ManifestHash: hash, RequestHash: hash, Step: "busy"}, func(current vmrestore.Current) (vmrestore.Mutation, error) {
		current.State.Fence = &vmrestore.Fence{Kind: "restore", Phase: "prepared", OperationID: "busy-restore", ManifestHash: hash, TargetVMUID: "vm-uid"}
		receipt := vmrestore.NewReceipt(scope, "busy-restore", hash)
		return vmrestore.Mutation{State: current.State, Receipt: &receipt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	before := len(dyn.Actions())
	for _, action := range []string{"start", "stop"} {
		result := runRestoredBatch(t, batch, scope, action)
		var event dbmodel.ServiceEvent
		err := a.getDBManager().DB().Where("event_id = ?", "batch-"+action).Take(&event).Error
		if result[0].Status != apimodel.BatchOpResultItemStatusFailure || err != nil || event.Status != "failure" || event.FinalStatus != "complete" {
			t.Fatalf("rejected %s request left an unfinished event: %#v, %#v, %v", action, result[0], event, err)
		}
	}
	current, err := store.Read(scope, "")
	if err != nil || current.State.Fence == nil || current.State.Fence.OperationID != "busy-restore" || len(dyn.Actions()) != before || len(queue.tasks) != 0 {
		t.Fatalf("busy restore was mutated by batch power: %v", err)
	}
}
