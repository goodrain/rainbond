package vmrestore

import (
	"context"
	"reflect"
	"strings"
	"time"

	"github.com/goodrain/rainbond/pkg/vmrestore/copyio"
	"github.com/goodrain/rainbond/pkg/vmrestore/restoreplan"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const restoreNonceAnnotation = "rainbond.io/vm-restore-intent"

var restoreGVR = map[string]schema.GroupVersionResource{
	"PersistentVolumeClaim": {Version: "v1", Resource: "persistentvolumeclaims"},
	"Pod":                   {Version: "v1", Resource: "pods"},
	"ConfigMap":             {Version: "v1", Resource: "configmaps"},
	"Job":                   {Group: "batch", Version: "v1", Resource: "jobs"},
}

// CertifiedRestoreStorage is an exact, privileged provider proof, never a UI input.
// Its zero value denies all writes. Classes remain separate ROX/writable paths.
type CertifiedRestoreStorage struct {
	Capability                                                 restoreplan.Capability
	ROXClassName, ROXClassUID, TargetClassName, TargetClassUID string
	RuntimeImageID                                             string
	DeleteReclaimVerified                                      bool
}

func (c CertifiedRestoreStorage) validate(plan RestoreResourcePlan) error {
	if c.Capability.Protocol != copyio.Protocol || !c.Capability.ROXVerified || !c.Capability.SparsePersistenceVerified || !c.DeleteReclaimVerified || c.Capability.WorkerImage != plan.WorkerImage || c.RuntimeImageID != plan.RuntimeImageID || c.Capability.StorageClassUID != c.TargetClassUID || c.ROXClassUID == c.TargetClassUID || c.ROXClassName == c.TargetClassName {
		return ErrRestoreCapability
	}
	for _, d := range plan.Disks {
		if d.ROXClass != c.ROXClassName || d.ROXClassUID != c.ROXClassUID || d.TargetClass != c.TargetClassName || d.TargetClassUID != c.TargetClassUID {
			return ErrRestoreCapability
		}
	}
	return nil
}

// RestoreResourceExecutor makes one API side effect per call. The caller has
// persisted its frame and all requests are re-fenced immediately before mutation.
type RestoreResourceExecutor struct {
	Store          *Store
	Core           kubernetes.Interface
	Dynamic        dynamic.Interface
	Certification  CertifiedRestoreStorage
	BeforeMutation func(context.Context, Scope, string, string) error
}

func (e *RestoreResourceExecutor) current(ctx context.Context, scope Scope, op string) (Current, error) {
	if ctx.Err() != nil || e == nil || e.Store == nil || e.Core == nil || e.Dynamic == nil {
		return Current{}, ErrReferenceUnavailable
	}
	c, err := e.Store.Read(scope, op)
	if err != nil {
		return c, err
	}
	if c.Receipt == nil || c.Receipt.Execution == nil || c.Receipt.Restore == nil || c.State.Fence == nil || c.State.Fence.Kind != "restore" || c.State.Fence.OperationID != op || c.State.Fence.ManifestHash != c.Receipt.ManifestHash {
		return c, ErrBusy
	}
	// Follow the existing commit/rollback witnesses, never a caller's latest
	// generation or an unsealed state change while this operation owns the fence.
	generation := c.Receipt.Execution.Frame.SourceGeneration
	if step, committed := c.Receipt.Steps[RestoreCommitStep]; committed {
		if !step.Applied || step.BindingGeneration <= generation || step.BindingGeneration-generation != 1 || step.RequestHash != c.Receipt.ConfigCommitHash {
			return c, ErrConflict
		}
		generation = step.BindingGeneration
		if rollback, rolledBack := c.Receipt.Steps["restore-rollback"]; rolledBack {
			if !rollback.Applied || rollback.BindingGeneration <= generation || rollback.BindingGeneration-generation != 1 || !hashPattern.MatchString(c.Receipt.RollbackConfigHash) {
				return c, ErrConflict
			}
			generation = rollback.BindingGeneration
		}
	} else if c.Receipt.Steps["restore-rollback"].Applied {
		return c, ErrConflict
	}
	if c.State.BindingGeneration != generation {
		return c, ErrConflict
	}
	if err := e.Certification.validate(c.Receipt.Execution.Frame); err != nil {
		return c, err
	}
	return c, nil
}
func desiredObject(obj interface{}, kind string) (*unstructured.Unstructured, error) {
	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, ErrCorrupt
	}
	result := &unstructured.Unstructured{Object: data}
	result.SetKind(kind)
	version := "v1"
	if kind == "Job" {
		version = "batch/v1"
	}
	result.SetAPIVersion(version)
	return result, nil
}

func resourceCreateAllowed(c Current) bool {
	if c.Receipt == nil || c.Receipt.Execution == nil || c.State.Fence == nil {
		return false
	}
	phase := c.State.Fence.Phase
	return c.State.BindingGeneration == c.Receipt.Execution.Frame.SourceGeneration && !c.Receipt.Steps[RestoreCommitStep].Applied && !strings.HasPrefix(phase, "cancel") && !strings.HasPrefix(phase, "rollback")
}

func currentResourceIntent(c Current, expected RestoreObjectIntent) (RestoreObjectIntent, error) {
	if c.Receipt == nil || c.Receipt.Execution == nil {
		return RestoreObjectIntent{}, ErrConflict
	}
	current, exists := c.Receipt.Execution.Intents[expected.Key]
	if !exists || current.Nonce != expected.Nonce || current.DesiredHash != expected.DesiredHash || current.Purpose != expected.Purpose || current.Object.Kind != expected.Object.Kind || current.Object.Namespace != expected.Object.Namespace || current.Object.Name != expected.Object.Name || current.DeleteRequested || current.Deleted {
		return RestoreObjectIntent{}, ErrConflict
	}
	return current, nil
}

// sameDesired permits Kubernetes defaults but never a changed desired field.
// The annotation is not trusted as a substitute for comparing the actual spec.
func sameDesired(want, got *unstructured.Unstructured) bool {
	if want.GetKind() != got.GetKind() || want.GetName() != got.GetName() || want.GetNamespace() != got.GetNamespace() || got.GetDeletionTimestamp() != nil || len(got.GetOwnerReferences()) != 0 {
		return false
	}
	for k, v := range want.GetAnnotations() {
		if got.GetAnnotations()[k] != v {
			return false
		}
	}
	for k, v := range want.GetLabels() {
		if got.GetLabels()[k] != v {
			return false
		}
	}
	for _, key := range []string{"spec", "data", "immutable"} {
		if value, ok := want.Object[key]; ok && !subsetJSON(value, got.Object[key]) {
			return false
		}
	}
	if want.GetKind() == "PersistentVolumeClaim" {
		_, a, _ := unstructured.NestedFieldNoCopy(want.Object, "spec", "dataSource")
		_, b, _ := unstructured.NestedFieldNoCopy(got.Object, "spec", "dataSource")
		if a != b {
			return false
		}
		wantRef, _, _ := unstructured.NestedFieldNoCopy(want.Object, "spec", "dataSourceRef")
		gotRef, hasRef, _ := unstructured.NestedFieldNoCopy(got.Object, "spec", "dataSourceRef")
		if wantRef == nil && hasRef {
			source, hasSource, _ := unstructured.NestedMap(want.Object, "spec", "dataSource")
			if !hasSource || !reflect.DeepEqual(source, gotRef) {
				return false
			}
		} else if !reflect.DeepEqual(wantRef, gotRef) {
			return false
		}
	}
	return true
}
func subsetJSON(want, got interface{}) bool {
	switch w := want.(type) {
	case map[string]interface{}:
		g, ok := got.(map[string]interface{})
		if !ok {
			return false
		}
		for k, v := range w {
			if !subsetJSON(v, g[k]) {
				return false
			}
		}
		return true
	case []interface{}:
		g, ok := got.([]interface{})
		if !ok || len(w) != len(g) {
			return false
		}
		for i := range w {
			if !subsetJSON(w[i], g[i]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(want, got)
	}
}

// Ensure records nonce+fingerprint before Create. Unknown Create is reconciled
// only through that intent. A known UID can never be replaced or recreated.
func (e *RestoreResourceExecutor) Ensure(ctx context.Context, scope Scope, op, key, purpose, kind string, obj interface{}) (ObjectRef, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	c, err := e.current(ctx, scope, op)
	if err != nil {
		return ObjectRef{}, err
	}
	gvr, ok := restoreGVR[kind]
	if kind == "Pod" {
		return ObjectRef{}, ErrCorrupt
	}
	if !ok {
		return ObjectRef{}, ErrCorrupt
	}
	want, err := desiredObject(obj, kind)
	if err != nil {
		return ObjectRef{}, err
	}
	if want.GetNamespace() != scope.Namespace || !validName(want.GetName()) || !idPattern.MatchString(key) || !idPattern.MatchString(purpose) || len(want.GetOwnerReferences()) != 0 {
		return ObjectRef{}, ErrCorrupt
	}
	fingerprint, err := snapshotDigest(want.Object)
	if err != nil {
		return ObjectRef{}, err
	}
	ri := e.Dynamic.Resource(gvr).Namespace(scope.Namespace)
	intent, exists := c.Receipt.Execution.Intents[key]
	if !exists {
		// An object present before our durable intent is not ours, even with labels.
		_, getErr := ri.Get(ctx, want.GetName(), metav1.GetOptions{})
		if getErr == nil {
			return ObjectRef{}, ErrConflict
		}
		if !apierrors.IsNotFound(getErr) {
			return ObjectRef{}, ErrReferenceUnavailable
		}
		intent = RestoreObjectIntent{Key: key, Nonce: uuid.NewString(), DesiredHash: fingerprint, Object: ObjectRef{Kind: kind, Namespace: scope.Namespace, Name: want.GetName()}, Purpose: purpose}
		if _, err = e.Store.resourceIntent(scope, op, intent); err != nil {
			return ObjectRef{}, err
		}
	}
	if intent.DesiredHash != fingerprint || intent.Purpose != purpose || intent.Object.Kind != kind || intent.Object.Name != want.GetName() || intent.DeleteRequested {
		return ObjectRef{}, ErrConflict
	}
	annotations := want.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[restoreNonceAnnotation] = intent.Nonce
	want.SetAnnotations(annotations)
	got, getErr := ri.Get(ctx, want.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(getErr) {
		if intent.Object.UID != "" {
			return ObjectRef{}, ErrConflict
		}
		// Before creating a Job, caller separately seals no prior active copy writer.
		if _, err = e.current(ctx, scope, op); err != nil {
			return ObjectRef{}, err
		}
		if e.BeforeMutation == nil {
			return ObjectRef{}, ErrCorrupt
		}
		if err = e.BeforeMutation(ctx, scope, op, key); err != nil {
			return ObjectRef{}, err
		}
		latest, currentErr := e.current(ctx, scope, op)
		if currentErr != nil {
			return ObjectRef{}, currentErr
		}
		intent, err = currentResourceIntent(latest, intent)
		if err != nil || !resourceCreateAllowed(latest) {
			return ObjectRef{}, ErrConflict
		}
		if intent.Object.UID != "" {
			return ObjectRef{}, ErrConflict
		}
		issued, issueErr := e.Store.resourceCreateIssued(scope, op, intent)
		if issueErr != nil {
			return ObjectRef{}, issueErr
		}
		if issued.Replayed {
			// A previous request may still be in flight or have had an unknown
			// outcome. Only observe it; a 404 cannot authorize another Create.
			got, err = ri.Get(ctx, want.GetName(), metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return ObjectRef{}, ErrRestoreUnknown
			}
			if err != nil {
				return ObjectRef{}, ErrReferenceUnavailable
			}
		} else {
			if _, err = e.current(ctx, scope, op); err != nil {
				return ObjectRef{}, err
			}
			got, err = ri.Create(ctx, want, metav1.CreateOptions{})
			if err != nil {
				return ObjectRef{}, ErrRestoreUnknown
			}
		}
	} else if getErr != nil {
		return ObjectRef{}, ErrReferenceUnavailable
	}
	latest, err := e.current(ctx, scope, op)
	if err != nil {
		return ObjectRef{}, err
	}
	intent, err = currentResourceIntent(latest, intent)
	if err != nil {
		return ObjectRef{}, err
	}
	if !sameDesired(want, got) || !validUID(string(got.GetUID())) || (intent.Object.UID != "" && intent.Object.UID != string(got.GetUID())) {
		return ObjectRef{}, ErrConflict
	}
	ref := refOf(kind, got)
	if intent.Object.UID == "" {
		if _, err = e.Store.resourceObserved(scope, op, key, ref, nil); err != nil {
			return ObjectRef{}, err
		}
	}
	latest, err = e.current(ctx, scope, op)
	if err != nil {
		return ObjectRef{}, err
	}
	observed, err := currentResourceIntent(latest, intent)
	if err != nil || !sameObject(observed.Object, ref) {
		return ObjectRef{}, ErrConflict
	}
	return ref, nil
}

// PinBoundClaim verifies actual PV claimRef UID and storage driver and records
// its UID before cleanup can be requested. It never persists the CSI volumeHandle.
func (e *RestoreResourceExecutor) PinBoundClaim(ctx context.Context, scope Scope, op, key string) (ObjectRef, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c, err := e.current(ctx, scope, op)
	if err != nil {
		return ObjectRef{}, err
	}
	intent, ok := c.Receipt.Execution.Intents[key]
	if !ok || intent.Object.Kind != "PersistentVolumeClaim" || intent.Object.UID == "" {
		return ObjectRef{}, ErrConflict
	}
	pvc, err := e.Core.CoreV1().PersistentVolumeClaims(scope.Namespace).Get(ctx, intent.Object.Name, metav1.GetOptions{})
	if err != nil {
		return ObjectRef{}, ErrReferenceUnavailable
	}
	if CheckLivePVC(intent.Object, pvc.Namespace, pvc.Name, string(pvc.UID)) != nil || pvc.DeletionTimestamp != nil {
		return ObjectRef{}, ErrConflict
	}
	if pvc.Status.Phase != corev1.ClaimBound || pvc.Spec.VolumeName == "" {
		return ObjectRef{}, ErrRestorePending
	}
	pv, err := e.Core.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
	if err != nil {
		return ObjectRef{}, ErrReferenceUnavailable
	}
	if pv.Spec.ClaimRef == nil || pv.Spec.ClaimRef.UID != pvc.UID || pv.Spec.ClaimRef.Name != pvc.Name || pv.Spec.ClaimRef.Namespace != scope.Namespace || pv.Spec.CSI == nil || pv.Spec.CSI.Driver != e.Certification.Capability.Driver || pv.Spec.PersistentVolumeReclaimPolicy != corev1.PersistentVolumeReclaimDelete || pv.DeletionTimestamp != nil {
		return ObjectRef{}, ErrConflict
	}
	ref := ObjectRef{Kind: "PersistentVolume", Name: pv.Name, UID: string(pv.UID)}
	if intent.BoundPV != nil && !sameObject(*intent.BoundPV, ref) {
		return ObjectRef{}, ErrConflict
	}
	// Separate pin step: initial observed receipt may predate the PVC binding.
	_, err = e.Store.executionUpdate(scope, op, "pv-"+key, ref, func(c Current) (Mutation, error) {
		v := c.Receipt.Execution.Intents[key]
		if v.BoundPV != nil && !sameObject(*v.BoundPV, ref) {
			return Mutation{}, ErrConflict
		}
		v.BoundPV = &ref
		c.Receipt.Execution.Intents[key] = v
		return Mutation{State: c.State, Receipt: c.Receipt}, nil
	})
	return ref, err
}
func classMatches(sc *storagev1.StorageClass, name, uid, driver, mounter string, rox bool) bool {
	if sc.Name != name || string(sc.UID) != uid || sc.Provisioner != driver || sc.DeletionTimestamp != nil || sc.Parameters["mounter"] != mounter || sc.ReclaimPolicy == nil || *sc.ReclaimPolicy != corev1.PersistentVolumeReclaimDelete {
		return false
	}
	if rox {
		return sc.Parameters["backingSnapshot"] == "true" && sc.Parameters["pool"] == "" && len(sc.AllowedTopologies) == 0
	}
	return sc.Parameters["backingSnapshot"] != "true"
}

// VerifySource checks exact class/snapshot/content identities before any ROX create.
func (e *RestoreResourceExecutor) VerifySource(ctx context.Context, scope Scope, op string, diskID uint) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c, err := e.current(ctx, scope, op)
	if err != nil {
		return err
	}
	frame := c.Receipt.Execution.Frame
	var d *RestoreDiskResource
	for i := range frame.Disks {
		if frame.Disks[i].VolumeID == diskID {
			d = &frame.Disks[i]
		}
	}
	if d == nil {
		return ErrConflict
	}
	target, getErr := e.Core.StorageV1().StorageClasses().Get(ctx, d.TargetClass, metav1.GetOptions{})
	if getErr != nil {
		return ErrReferenceUnavailable
	}
	rox, getErr := e.Core.StorageV1().StorageClasses().Get(ctx, d.ROXClass, metav1.GetOptions{})
	if getErr != nil {
		return ErrReferenceUnavailable
	}
	if !classMatches(target, d.TargetClass, d.TargetClassUID, e.Certification.Capability.Driver, e.Certification.Capability.Mounter, false) || !classMatches(rox, d.ROXClass, d.ROXClassUID, e.Certification.Capability.Driver, e.Certification.Capability.Mounter, true) {
		return ErrRestoreCapability
	}
	expectedParameters := map[string]string{}
	for key, value := range target.Parameters {
		expectedParameters[key] = value
	}
	delete(expectedParameters, "pool")
	expectedParameters["backingSnapshot"] = "true"
	if !reflect.DeepEqual(rox.Parameters, expectedParameters) || !reflect.DeepEqual(rox.MountOptions, target.MountOptions) || !reflect.DeepEqual(rox.VolumeBindingMode, target.VolumeBindingMode) {
		return ErrRestoreCapability
	}
	gvr := schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}
	snap, err := e.Dynamic.Resource(gvr).Namespace(scope.Namespace).Get(ctx, d.Snapshot.Name, metav1.GetOptions{})
	if err != nil {
		return ErrReferenceUnavailable
	}
	ready, _, _ := unstructured.NestedBool(snap.Object, "status", "readyToUse")
	contentName, _, _ := unstructured.NestedString(snap.Object, "status", "boundVolumeSnapshotContentName")
	if !sameObject(d.Snapshot, refOf("VolumeSnapshot", snap)) || snap.GetDeletionTimestamp() != nil {
		return ErrConflict
	}
	if !ready || contentName == "" {
		return ErrRestorePending
	}
	content, err := e.Dynamic.Resource(schema.GroupVersionResource{Group: gvr.Group, Version: gvr.Version, Resource: "volumesnapshotcontents"}).Get(ctx, contentName, metav1.GetOptions{})
	if err != nil {
		return ErrReferenceUnavailable
	}
	uid, _, _ := unstructured.NestedString(content.Object, "spec", "volumeSnapshotRef", "uid")
	ns, _, _ := unstructured.NestedString(content.Object, "spec", "volumeSnapshotRef", "namespace")
	name, _, _ := unstructured.NestedString(content.Object, "spec", "volumeSnapshotRef", "name")
	driver, _, _ := unstructured.NestedString(content.Object, "spec", "driver")
	if !sameObject(d.SnapshotContent, refOf("VolumeSnapshotContent", content)) || contentName != d.SnapshotContent.Name || uid != d.Snapshot.UID || ns != scope.Namespace || name != d.Snapshot.Name || driver != e.Certification.Capability.Driver || content.GetDeletionTimestamp() != nil {
		return ErrConflict
	}
	return nil
}

// Delete only targets a sealed UID, records its intent first and observes PV
// deletion for ROX. A PVC404 is not proof its snapshot reference is released.
func (e *RestoreResourceExecutor) Delete(ctx context.Context, scope Scope, op, key string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	c, err := e.current(ctx, scope, op)
	if err != nil {
		return err
	}
	v, ok := c.Receipt.Execution.Intents[key]
	if !ok || v.Object.UID == "" {
		return ErrConflict
	}
	if v.Deleted {
		return nil
	}
	if v.Object.Kind == "PersistentVolumeClaim" && v.BoundPV == nil {
		return ErrConflict
	}
	if !v.DeleteRequested {
		if _, err = e.Store.resourceDeleting(scope, op, key, false); err != nil {
			return err
		}
	}
	ri := e.Dynamic.Resource(restoreGVR[v.Object.Kind]).Namespace(scope.Namespace)
	got, getErr := ri.Get(ctx, v.Object.Name, metav1.GetOptions{})
	if getErr == nil {
		if string(got.GetUID()) != v.Object.UID {
			return ErrConflict
		}
		if _, err = e.current(ctx, scope, op); err != nil {
			return err
		}
		if e.BeforeMutation == nil {
			return ErrCorrupt
		}
		if err = e.BeforeMutation(ctx, scope, op, "delete:"+key); err != nil {
			return err
		}
		if _, err = e.current(ctx, scope, op); err != nil {
			return err
		}
		uid := types.UID(v.Object.UID)
		policy := metav1.DeletePropagationForeground
		if err = ri.Delete(ctx, v.Object.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}, PropagationPolicy: &policy}); err != nil && !apierrors.IsNotFound(err) {
			return ErrRestoreUnknown
		}
		return ErrRestorePending
	}
	if !apierrors.IsNotFound(getErr) {
		return ErrReferenceUnavailable
	}
	if v.BoundPV != nil {
		pv, pvErr := e.Core.CoreV1().PersistentVolumes().Get(ctx, v.BoundPV.Name, metav1.GetOptions{})
		if pvErr == nil {
			if string(pv.UID) != v.BoundPV.UID {
				return ErrConflict
			}
			return ErrRestorePending
		}
		if !apierrors.IsNotFound(pvErr) {
			return ErrReferenceUnavailable
		}
	}
	_, err = e.Store.resourceDeleting(scope, op, key, true)
	return err
}
