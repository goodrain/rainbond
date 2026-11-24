package handler

import (
	"context"
	"errors"

	"github.com/goodrain/rainbond/api/util/bcode"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kv "kubevirt.io/api/core/v1"
)

// vmPowerSubmittedError distinguishes failed observation after an accepted
// runtime write from a failure before this request was submitted.
type vmPowerSubmittedError struct{ cause error }

func (e *vmPowerSubmittedError) Error() string { return e.cause.Error() }
func (e *vmPowerSubmittedError) Unwrap() error { return e.cause }

func (s *ServiceAction) restoredPower(ctx context.Context, serviceID, action string) (bool, error) {
	manager := s.getDBManager()
	if manager == nil || manager.DB() == nil {
		return false, nil
	}
	service, err := manager.TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		return true, err
	}
	if service == nil || !service.IsVM() {
		return false, nil
	}
	tenant, err := manager.TenantDao().GetTenantByUUID(service.TenantID)
	if err != nil {
		return true, err
	}
	if tenant == nil {
		return true, vmrestore.ErrCorrupt
	}
	scope := vmrestore.Scope{TenantID: service.TenantID, ServiceID: serviceID, Namespace: tenant.Namespace}
	if err := snapshotContextScope(ctx, serviceID, scope); err != nil {
		return true, err
	}
	store := vmrestore.NewStore(manager.DB())
	state, err := store.Read(scope, "")
	if err != nil {
		return true, err
	}
	if state.State.Fence != nil {
		return true, &bcode.Code{Status: 409, Code: 409, Message: "vm_restore_busy"}
	}
	if len(state.State.Bindings) == 0 {
		return false, nil
	}
	if s.kubeClient == nil || s.kubevirtClient == nil {
		return true, vmrestore.ErrReferenceUnavailable
	}
	client := s.kubevirtClient.DynamicClient()
	resolver := vmrestore.NewResolver(store, s.kubeClient, client)
	if state.State.Admission != nil {
		if action == "stop" && (state.State.Admission.Action == "ordinary-vm-start" || state.State.Admission.Action == "ordinary-vm-restart") {
			eventID := ""
			if event, ok := ctx.Value(ctxutil.ContextKey("event")).(*dbmodel.ServiceEvent); ok && event != nil {
				eventID = event.EventID
			}
			err := store.StopPendingOrdinaryVM(ctx, resolver, client, scope, eventID)
			if errors.Is(err, vmrestore.ErrRestorePending) {
				return true, &bcode.Code{Status: 202, Code: 202, Message: "vm_power_pending"}
			}
			if errors.Is(err, vmrestore.ErrRestoreUnknown) || errors.Is(err, vmrestore.ErrReferenceUnavailable) {
				return true, &vmPowerSubmittedError{cause: err}
			}
			return true, err
		}
		err := store.ObserveOrdinaryVMCompletion(ctx, resolver, client, scope)
		if err != nil {
			return true, err
		}
	}
	selection, err := store.ReadRestoredRuntimeSelection(ctx, resolver, scope)
	if err != nil {
		return true, err
	}
	if selection == nil {
		return false, nil
	}
	gvr := schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachines"}
	objects, err := client.Resource(gvr).Namespace(scope.Namespace).List(ctx, metav1.ListOptions{LabelSelector: "service_id=" + serviceID})
	if err != nil {
		return true, vmrestore.ErrReferenceUnavailable
	}
	if len(objects.Items) != 1 {
		return true, vmrestore.ErrRestoreCapability
	}
	var vm kv.VirtualMachine
	if runtime.DefaultUnstructuredConverter.FromUnstructured(objects.Items[0].Object, &vm) != nil {
		return true, vmrestore.ErrCorrupt
	}
	ref := vmrestore.ObjectRef{Kind: "VirtualMachine", Namespace: scope.Namespace, Name: vm.Name, UID: string(vm.UID)}
	projected, err := vmrestore.ProjectRestoredRuntime(ctx, s.kubeClient, &vm, *selection)
	if err != nil {
		return true, err
	}
	previous := []vmrestore.ObjectRef{}
	if action == "restart" {
		if !vmrestore.RuntimeVMMatches(&vm, projected) {
			return true, vmrestore.ErrConflict
		}
		instances, err := client.Resource(schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachineinstances"}).Namespace(scope.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return true, vmrestore.ErrReferenceUnavailable
		}
		for _, instance := range instances.Items {
			for _, owner := range instance.GetOwnerReferences() {
				if owner.Kind == "VirtualMachine" && owner.UID == vm.UID {
					previous = append(previous, vmrestore.ObjectRef{Kind: "VirtualMachineInstance", Namespace: scope.Namespace, Name: instance.GetName(), UID: string(instance.GetUID())})
				}
			}
		}
		if len(previous) != 1 {
			return true, vmrestore.ErrRestorePending
		}
	}
	eventID := ""
	if event, ok := ctx.Value(ctxutil.ContextKey("event")).(*dbmodel.ServiceEvent); ok && event != nil {
		eventID = event.EventID
	}
	_, op, err := store.ClaimOrdinaryVMWithEvent(scope, action, ref, eventID, previous...)
	if err != nil {
		return true, err
	}
	abort := func(cause error) (bool, error) {
		if err := store.FinishOrdinaryVMAttempt(scope, op, true); err != nil {
			return true, err
		}
		return true, cause
	}
	strategy := kv.RunStrategyAlways
	if action == "stop" {
		strategy = kv.RunStrategyHalted
	}
	projected.Spec.Running = nil
	projected.Spec.RunStrategy = &strategy
	current, err := store.Read(scope, op)
	if err != nil || current.State.Fence != nil || current.State.Admission == nil || current.State.Admission.Token != op || current.State.BindingGeneration != selection.BindingGeneration {
		return abort(vmrestore.ErrConflict)
	}
	object, err := vmrestore.PreserveRuntimeVM(&objects.Items[0], &vm, projected)
	if err != nil {
		return abort(err)
	}
	if action == "restart" {
		err = s.kubevirtClient.VirtualMachine(scope.Namespace).Restart(ctx, vm.Name, &kv.RestartOptions{})
	} else {
		_, err = client.Resource(gvr).Namespace(scope.Namespace).Update(ctx, object, metav1.UpdateOptions{})
	}
	if err != nil {
		if err := store.FinishOrdinaryVMAttempt(scope, op, false); err != nil {
			return true, &vmPowerSubmittedError{cause: err}
		}
		return true, vmrestore.ErrRestoreUnknown
	}
	err = store.ObserveOrdinaryVMCompletion(ctx, resolver, client, scope)
	if errors.Is(err, vmrestore.ErrRestorePending) {
		return true, &bcode.Code{Status: 202, Code: 202, Message: "vm_power_pending"}
	}
	if errors.Is(err, vmrestore.ErrOrdinaryVMFailed) {
		return true, err
	}
	if err != nil {
		return true, &vmPowerSubmittedError{cause: err}
	}
	return true, nil
}
