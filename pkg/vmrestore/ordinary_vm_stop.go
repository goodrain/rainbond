package vmrestore

import (
	"context"
	"errors"
	"strings"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmstorage"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// ErrOrdinaryVMFailed identifies an observed and stopped failed startup.
var ErrOrdinaryVMFailed = errors.New("VM startup failed")

const ordinaryStopStep = "ordinary-stop-request"
const ordinaryFailureStopStep = "ordinary-failure-stop"
const ordinaryStopAnnotation = "rainbond.io/vm-power-stop-request"

func ordinaryStopIntent(receipt *Receipt) (eventID, step string, failed bool, err error) {
	if receipt == nil {
		return "", "", false, ErrCorrupt
	}
	for name, result := range receipt.Steps {
		if !result.Applied || name != ordinaryFailureStopStep && name != ordinaryStopStep && !strings.HasPrefix(name, ordinaryStopStep+"-") {
			continue
		}
		if step != "" {
			return "", "", false, ErrCorrupt
		}
		step, failed = name, name == ordinaryFailureStopStep
		if strings.HasPrefix(name, ordinaryStopStep+"-") {
			eventID = strings.TrimPrefix(name, ordinaryStopStep+"-")
			if !idPattern.MatchString(eventID) || len(eventID) > 40 {
				return "", "", false, ErrCorrupt
			}
		}
	}
	return
}

// StopPendingOrdinaryVM stops the VM owned by an unfinished power operation.
func (s *Store) StopPendingOrdinaryVM(ctx context.Context, resolver *Resolver, client dynamic.Interface, scope Scope, eventID string) error {
	return s.stopPendingOrdinaryVM(ctx, resolver, client, scope, eventID, false)
}

func (s *Store) stopPendingOrdinaryVM(ctx context.Context, resolver *Resolver, client dynamic.Interface, scope Scope, eventID string, failed bool) error {
	if resolver == nil || client == nil || eventID != "" && (!idPattern.MatchString(eventID) || len(eventID) > 40) {
		return ErrCorrupt
	}
	current, err := s.ReadOnly(scope, "")
	if err != nil {
		return err
	}
	admission := current.State.Admission
	if current.State.Fence != nil || admission == nil || admission.Action != "ordinary-vm-start" && admission.Action != "ordinary-vm-restart" {
		return ErrBusy
	}
	current, err = s.ReadOnly(scope, admission.Token)
	if err != nil {
		return err
	}
	if current.Receipt == nil || len(current.Receipt.Resources) == 0 || current.Receipt.Steps["ordinary-prepare"].BindingGeneration != current.State.BindingGeneration {
		return ErrConflict
	}
	ref := current.Receipt.Resources[0].ObjectRef
	vm, err := client.Resource(restoreVMGVR).Namespace(scope.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return ErrReferenceUnavailable
	}
	if !sameObject(ref, refOf("VirtualMachine", vm)) {
		return ErrConflict
	}
	step := ordinaryStopStep
	if failed {
		step = ordinaryFailureStopStep
	} else if eventID != "" {
		step += "-" + eventID
	}
	_, existing, _, err := ordinaryStopIntent(current.Receipt)
	if err != nil {
		return err
	}
	if existing != "" && existing != step {
		return ErrBusy
	}
	if existing == "" {
		expect := Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration, OperationID: admission.Token, ManifestHash: current.Receipt.ManifestHash, Step: step}
		expect.RequestHash, _ = snapshotDigest(struct {
			VM     ObjectRef
			Event  string
			Failed bool
		}{ref, eventID, failed})
		tx := s.db.Begin()
		if tx.Error != nil {
			return tx.Error
		}
		defer tx.Rollback()
		_, err = s.MutateTx(tx, scope, expect, func(now Current) (Mutation, error) {
			if now.State.Fence != nil || now.State.Admission == nil || now.State.Admission.Token != admission.Token || now.Receipt == nil || now.Receipt.Resources[0].ObjectRef != ref {
				return Mutation{}, ErrConflict
			}
			if eventID != "" {
				var count int
				if err := tx.Model(&dbmodel.ServiceEvent{}).Where("event_id = ? AND tenant_id = ? AND service_id = ?", eventID, scope.TenantID, scope.ServiceID).Count(&count).Error; err != nil {
					return Mutation{}, err
				}
				if count != 1 || eventID == now.Receipt.PowerEventID {
					return Mutation{}, ErrConflict
				}
			}
			return Mutation{State: now.State, Receipt: now.Receipt}, nil
		})
		if err != nil {
			return err
		}
		if err = tx.Commit().Error; err != nil {
			return err
		}
	}
	return s.ObserveOrdinaryVMCompletion(ctx, resolver, client, scope)
}

func (s *Store) observeOrdinaryStop(ctx context.Context, resolver *Resolver, client dynamic.Interface, scope Scope, current Current, vm *unstructured.Unstructured, failed bool) error {
	op := current.Receipt.OperationID
	strategy, _, _ := unstructured.NestedString(vm.Object, "spec", "runStrategy")
	if strategy != "Halted" || vm.GetAnnotations()[ordinaryStopAnnotation] != op {
		if vm.GetResourceVersion() == "" {
			return ErrReferenceUnavailable
		}
		updated := vm.DeepCopy()
		unstructured.RemoveNestedField(updated.Object, "spec", "running")
		_ = unstructured.SetNestedField(updated.Object, "Halted", "spec", "runStrategy")
		annotations := updated.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[ordinaryStopAnnotation] = op
		updated.SetAnnotations(annotations)
		latest, err := s.ReadOnly(scope, op)
		if err != nil {
			return err
		}
		if latest.State.Admission == nil || latest.State.Admission.Token != op || latest.State.Fence != nil || latest.State.ControlRevision != current.State.ControlRevision || latest.State.BindingGeneration != current.State.BindingGeneration {
			return ErrConflict
		}
		if _, err = client.Resource(restoreVMGVR).Namespace(scope.Namespace).Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
			return ErrRestoreUnknown
		}
	}
	instances, err := client.Resource(restoreVMIGVR).Namespace(scope.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return ErrReferenceUnavailable
	}
	for _, instance := range instances.Items {
		owned := false
		for _, owner := range instance.GetOwnerReferences() {
			if owner.Kind == "VirtualMachine" && owner.UID == vm.GetUID() {
				owned = true
			}
		}
		if instance.GetName() == vm.GetName() && !owned {
			return ErrConflict
		}
		if owned {
			return ErrRestorePending
		}
	}
	pods, err := resolver.core.CoreV1().Pods(scope.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return ErrReferenceUnavailable
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		for _, volume := range pod.Spec.Volumes {
			for _, binding := range current.State.Bindings {
				if binding.Attached && volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == binding.ActivePVC.Name {
					return ErrRestorePending
				}
			}
		}
	}
	latestVM, err := client.Resource(restoreVMGVR).Namespace(scope.Namespace).Get(ctx, vm.GetName(), metav1.GetOptions{})
	if err != nil {
		return ErrReferenceUnavailable
	}
	if !sameObject(refOf("VirtualMachine", vm), refOf("VirtualMachine", latestVM)) {
		return ErrConflict
	}
	strategy, _, _ = unstructured.NestedString(latestVM.Object, "spec", "runStrategy")
	if strategy != "Halted" || latestVM.GetAnnotations()[ordinaryStopAnnotation] != op {
		return ErrRestorePending
	}
	expect := Expectation{ControlRevision: current.State.ControlRevision, BindingGeneration: current.State.BindingGeneration, OperationID: op, ManifestHash: current.Receipt.ManifestHash, RequestHash: current.Receipt.ManifestHash, Step: "ordinary-cancelled"}
	if err := s.ordinaryOutcome(scope, expect, true, dbmodel.EventStatusFailure); err != nil {
		return err
	}
	if failed {
		return ErrOrdinaryVMFailed
	}
	return nil
}

func ordinaryInstanceFailed(ctx context.Context, client dynamic.Interface, scope Scope, ref ObjectRef, retiredUID string) (bool, error) {
	instances, err := client.Resource(restoreVMIGVR).Namespace(scope.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, ErrReferenceUnavailable
	}
	for _, instance := range instances.Items {
		if retiredUID != "" && string(instance.GetUID()) == retiredUID {
			continue
		}
		for _, owner := range instance.GetOwnerReferences() {
			if owner.Kind == "VirtualMachine" && string(owner.UID) == ref.UID {
				phase, _, _ := unstructured.NestedString(instance.Object, "status", "phase")
				if phase == "Failed" {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func ordinaryCloneFailed(ctx context.Context, resolver *Resolver, client dynamic.Interface, scope Scope, current Current) (bool, error) {
	for _, binding := range current.State.Bindings {
		if !binding.Attached {
			continue
		}
		pvc, err := resolver.core.CoreV1().PersistentVolumeClaims(scope.Namespace).Get(ctx, binding.ActivePVC.Name, metav1.GetOptions{})
		if err != nil {
			return false, ErrReferenceUnavailable
		}
		if !sameObject(binding.ActivePVC, refOf("PersistentVolumeClaim", pvc)) {
			return false, ErrConflict
		}
		if pvc.Spec.DataSource == nil || pvc.Spec.DataSource.Kind != "VolumeSnapshot" || pvc.Spec.VolumeName == "" {
			continue
		}
		pv, err := resolver.core.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
		if err != nil {
			return false, ErrReferenceUnavailable
		}
		if pv.Spec.CSI == nil || pv.Spec.CSI.Driver != vmstorage.LonghornSnapshotDriver {
			continue
		}
		if pv.Spec.ClaimRef == nil || pv.Spec.ClaimRef.UID != pvc.UID || pv.Spec.ClaimRef.Name != pvc.Name || pv.Spec.ClaimRef.Namespace != scope.Namespace {
			return false, ErrConflict
		}
		volume, err := readLonghornVolume(ctx, client, pv.Spec.CSI.VolumeHandle)
		if err != nil {
			return false, err
		}
		source, _, _ := unstructured.NestedString(volume.Object, "spec", "dataSource")
		state, _, _ := unstructured.NestedString(volume.Object, "status", "cloneStatus", "state")
		if strings.HasPrefix(source, "snap://") && state == "failed" {
			return true, nil
		}
	}
	return false, nil
}
