package vmrestore

import (
	"context"
	"strings"
	"time"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	kv "kubevirt.io/api/core/v1"
)

// ClaimOrdinaryVM owns normal power mutation through its observed completion.
// The receipt pins VM UID/generation and never replaces the binding authority.
func (s *Store) ClaimOrdinaryVM(scope Scope, action string, vm ObjectRef, previous ...ObjectRef) (Current, string, error) {
	return s.ClaimOrdinaryVMWithEvent(scope, action, vm, "", previous...)
}

// ClaimOrdinaryVMWithEvent binds only an existing server-scoped power event.
func (s *Store) ClaimOrdinaryVMWithEvent(scope Scope, action string, vm ObjectRef, eventID string, previous ...ObjectRef) (Current, string, error) {
	if eventID != "" && (!idPattern.MatchString(eventID) || len(eventID) > 40) {
		return Current{}, "", ErrCorrupt
	}
	if (action != "start" && action != "stop" && action != "restart") || vm.Kind != "VirtualMachine" || vm.Namespace != scope.Namespace || !validName(vm.Name) || !validUID(vm.UID) {
		return Current{}, "", ErrCorrupt
	}
	if action == "restart" {
		if len(previous) != 1 || previous[0].Kind != "VirtualMachineInstance" || previous[0].Namespace != scope.Namespace || !validName(previous[0].Name) || !validUID(previous[0].UID) {
			return Current{}, "", ErrCorrupt
		}
	} else if len(previous) != 0 {
		return Current{}, "", ErrCorrupt
	}
	c, err := s.Read(scope, "")
	if err != nil {
		return c, "", err
	}
	if c.State.Fence != nil || c.State.Admission != nil {
		return c, "", ErrBusy
	}
	op := "ordinary-vm-" + uuid.NewString()
	hash, _ := snapshotDigest(struct {
		Action     string
		VM         ObjectRef
		Generation uint64
		Previous   []ObjectRef
		EventID    string
	}{action, vm, c.State.BindingGeneration, previous, eventID})
	expect := Expectation{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration, OperationID: op, ManifestHash: hash, RequestHash: hash, Step: "ordinary-prepare"}
	tx := s.db.Begin()
	if tx.Error != nil {
		return Current{}, "", tx.Error
	}
	defer tx.Rollback()
	c, err = s.MutateTx(tx, scope, expect, func(c Current) (Mutation, error) {
		if eventID != "" {
			var count int
			if err := tx.Model(&dbmodel.ServiceEvent{}).Where("event_id = ? AND tenant_id = ? AND service_id = ?", eventID, scope.TenantID, scope.ServiceID).Count(&count).Error; err != nil {
				return Mutation{}, err
			}
			if count != 1 {
				return Mutation{}, ErrConflict
			}
		}
		if err := c.State.AdmitMutation(op, "ordinary-vm-"+action); err != nil {
			return Mutation{}, err
		}
		r := NewReceipt(scope, op, hash)
		r.PowerEventID = eventID
		r.OldBindings = cloneBindings(c.State.Bindings)
		r.NewBindings = cloneBindings(c.State.Bindings)
		r.Resources = []ResourceRef{{ObjectRef: vm, Purpose: "ordinary-vm-" + action}}
		for _, ref := range previous {
			r.Resources = append(r.Resources, ResourceRef{ObjectRef: ref, Purpose: "ordinary-vm-previous-instance"})
		}
		return Mutation{State: c.State, Receipt: &r}, nil
	})
	if err == nil {
		err = tx.Commit().Error
	}
	return c, op, err
}

func (s *Store) ordinaryOutcome(scope Scope, expect Expectation, known bool, status dbmodel.EventStatus) error {
	tx := s.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	_, err := s.MutateTx(tx, scope, expect, func(now Current) (Mutation, error) {
		stopEvent, stopStep, _, err := ordinaryStopIntent(now.Receipt)
		if err != nil {
			return Mutation{}, err
		}
		if known && stopStep != "" && (expect.Step != "ordinary-cancelled" || status != dbmodel.EventStatusFailure) {
			return Mutation{}, ErrConflict
		}
		if err := now.State.FinishMutation(expect.OperationID, known); err != nil {
			return Mutation{}, err
		}
		if known && now.Receipt.PowerEventID != "" {
			result := tx.Model(&dbmodel.ServiceEvent{}).Where("event_id = ? AND tenant_id = ? AND service_id = ?", now.Receipt.PowerEventID, scope.TenantID, scope.ServiceID).Updates(map[string]interface{}{"status": string(status), "final_status": string(dbmodel.EventFinalStatusComplete), "end_time": time.Now().Format(time.RFC3339)})
			if result.Error != nil {
				return Mutation{}, result.Error
			}
			if result.RowsAffected != 1 {
				return Mutation{}, ErrConflict
			}
		}
		if known && stopEvent != "" {
			result := tx.Model(&dbmodel.ServiceEvent{}).Where("event_id = ? AND tenant_id = ? AND service_id = ?", stopEvent, scope.TenantID, scope.ServiceID).Updates(map[string]interface{}{"status": string(dbmodel.EventStatusSuccess), "final_status": string(dbmodel.EventFinalStatusComplete), "end_time": time.Now().Format(time.RFC3339)})
			if result.Error != nil {
				return Mutation{}, result.Error
			}
			if result.RowsAffected != 1 {
				return Mutation{}, ErrConflict
			}
		}
		return Mutation{State: now.State, Receipt: now.Receipt}, nil
	})
	if err != nil {
		return err
	}
	return tx.Commit().Error
}

// FinishOrdinaryVMAttempt releases a known pre-I/O failure, or retains ownership
// of an uncertain write until an exact runtime observation resolves it.
func (s *Store) FinishOrdinaryVMAttempt(scope Scope, op string, known bool) error {
	c, err := s.Read(scope, op)
	if err != nil {
		return err
	}
	if c.Receipt == nil || c.State.Admission == nil || c.State.Admission.Token != op || !strings.HasPrefix(c.State.Admission.Action, "ordinary-vm-") || c.State.Fence != nil {
		return ErrConflict
	}
	step := "ordinary-outcome-unknown"
	if known {
		step = "ordinary-aborted-before-write"
	}
	expect := Expectation{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration, OperationID: op, ManifestHash: c.Receipt.ManifestHash, RequestHash: c.Receipt.ManifestHash, Step: step}
	return s.ordinaryOutcome(scope, expect, known, dbmodel.EventStatusFailure)
}

// runtimeUsesBindingView checks actual device references against the one live
// Resolver view. It never guesses a PVC from a volume name or creates a map.
func runtimeUsesBindingView(object map[string]interface{}, instance bool, view BindingView) error {
	volumes := vmVolumes(object, instance)
	for _, resolved := range view.Volumes {
		count := 0
		for _, raw := range volumes {
			volume, ok := raw.(map[string]interface{})
			if !ok {
				return ErrCorrupt
			}
			uses := volumeUses(volume, resolved)
			if !resolved.Attached && uses {
				return ErrConflict
			}
			if textAt(volume, "name") == resolved.CanonicalName {
				if !resolved.Attached || !uses {
					return ErrConflict
				}
				count++
			}
		}
		if resolved.Attached && count != 1 {
			return ErrConflict
		}
	}
	return nil
}

// ObserveOrdinaryVMCompletion is called by existing worker observation events
// and explicit power requests, never by a frontend GET or a new daemon.
func (s *Store) ObserveOrdinaryVMCompletion(ctx context.Context, resolver *Resolver, client dynamic.Interface, scope Scope) error {
	if resolver == nil || client == nil {
		return ErrReferenceUnavailable
	}
	c, err := s.Read(scope, "")
	if err != nil {
		return err
	}
	a := c.State.Admission
	if a == nil || !strings.HasPrefix(a.Action, "ordinary-vm-") {
		return nil
	}
	if c.State.Fence != nil {
		return ErrBusy
	}
	c, err = s.Read(scope, a.Token)
	if err != nil {
		return err
	}
	if c.Receipt == nil || len(c.Receipt.Resources) == 0 || c.Receipt.Steps["ordinary-prepare"].BindingGeneration != c.State.BindingGeneration {
		return ErrConflict
	}
	if (a.Action == "ordinary-vm-restart" && len(c.Receipt.Resources) != 2) || (a.Action != "ordinary-vm-restart" && len(c.Receipt.Resources) != 1) {
		return ErrConflict
	}
	ref := c.Receipt.Resources[0].ObjectRef
	vm, err := client.Resource(restoreVMGVR).Namespace(scope.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return ErrReferenceUnavailable
	}
	if !sameObject(ref, refOf("VirtualMachine", vm)) {
		return ErrConflict
	}
	_, stopStep, failedStop, err := ordinaryStopIntent(c.Receipt)
	if err != nil {
		return err
	}
	if stopStep != "" {
		return s.observeOrdinaryStop(ctx, resolver, client, scope, c, vm, failedStop)
	}
	if a.Action != "ordinary-vm-stop" {
		retiredUID := ""
		if a.Action == "ordinary-vm-restart" {
			retiredUID = c.Receipt.Resources[1].UID
		}
		failed, err := ordinaryInstanceFailed(ctx, client, scope, ref, retiredUID)
		if err != nil {
			return err
		}
		if failed {
			return s.stopPendingOrdinaryVM(ctx, resolver, client, scope, "", true)
		}
		failed, err = ordinaryCloneFailed(ctx, resolver, client, scope, c)
		if err != nil {
			return err
		}
		if failed {
			return s.stopPendingOrdinaryVM(ctx, resolver, client, scope, "", true)
		}
	}
	view, err := resolver.ReadBindingView(ctx, scope)
	if err != nil {
		return err
	}
	if view.BindingGeneration != c.State.BindingGeneration {
		return ErrConflict
	}
	if err := runtimeUsesBindingView(vm.Object, false, view); err != nil {
		return err
	}
	var desired kv.VirtualMachine
	if runtime.DefaultUnstructuredConverter.FromUnstructured(vm.Object, &desired) != nil {
		return ErrCorrupt
	}
	if (a.Action != "ordinary-vm-stop") != vmWantsRunning(&desired) {
		return ErrRestorePending
	}
	selected, err := s.ReadRestoredRuntimeSelection(ctx, resolver, scope)
	if err != nil {
		return err
	}
	instances, err := client.Resource(restoreVMIGVR).Namespace(scope.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return ErrReferenceUnavailable
	}
	running, count := false, 0
	for _, instance := range instances.Items {
		for _, owner := range instance.GetOwnerReferences() {
			if owner.Kind == "VirtualMachine" && string(owner.UID) == ref.UID {
				count++
				phase, _, _ := unstructured.NestedString(instance.Object, "status", "phase")
				running = phase == string(kv.Running) && instance.GetDeletionTimestamp() == nil
				if a.Action != "ordinary-vm-stop" {
					if err := runtimeUsesBindingView(instance.Object, true, view); err != nil {
						return err
					}
					if a.Action == "ordinary-vm-restart" && string(instance.GetUID()) == c.Receipt.Resources[1].UID {
						return ErrRestorePending
					}
					if selected != nil {
						var actual kv.VirtualMachineInstance
						if runtime.DefaultUnstructuredConverter.FromUnstructured(instance.Object, &actual) != nil || !runtimeInstanceMatches(&actual, &desired, *selected) {
							return ErrConflict
						}
					}
				}
			}
		}
	}
	if a.Action != "ordinary-vm-stop" && (!running || count != 1) {
		return ErrRestorePending
	}
	if a.Action == "ordinary-vm-stop" && count != 0 {
		return ErrRestorePending
	}
	expect := Expectation{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration, OperationID: a.Token, ManifestHash: c.Receipt.ManifestHash, RequestHash: c.Receipt.ManifestHash, Step: "ordinary-complete"}
	return s.ordinaryOutcome(scope, expect, true, dbmodel.EventStatusSuccess)
}
