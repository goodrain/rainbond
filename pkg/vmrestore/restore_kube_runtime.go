package vmrestore

import (
	"context"
	"strconv"

	dbmodel "github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	kv "kubevirt.io/api/core/v1"
)

// KubeRestoreRuntime observes real VM/VMI/Pod state and applies one exact-UID
// halted/runtime update. It does not schedule work or own the Console workflow.
type KubeRestoreRuntime struct {
	Store         *Store
	Resolver      *Resolver
	Core          kubernetes.Interface
	Dynamic       dynamic.Interface
	BuildPlatform func(context.Context, Scope) (*kv.VirtualMachine, error)
}

var restoreVMGVR = schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachines"}
var restoreVMIGVR = schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachineinstances"}
var restoreMigrationGVR = schema.GroupVersionResource{Group: "kubevirt.io", Version: "v1", Resource: "virtualmachineinstancemigrations"}

// NetworkIdentityHash seals only current platform IP/port identities, never
// credentials or old guest initialization. Both prepare and runtime use it.
func (s *Store) NetworkIdentityHash(scope Scope) (string, error) {
	var ports []dbmodel.TenantServicesPort
	if err := s.db.Where("service_id = ?", scope.ServiceID).Order("ID").Find(&ports).Error; err != nil {
		return "", err
	}
	type port struct {
		ID                              uint
		Tenant, Service, Protocol, Name string
		Container, Mapping              int
	}
	values := []port{}
	for _, p := range ports {
		if p.TenantID != scope.TenantID {
			return "", ErrCorrupt
		}
		values = append(values, port{p.ID, p.TenantID, p.ServiceID, p.Protocol, p.K8sServiceName, p.ContainerPort, p.MappingPort})
	}
	var attrs []dbmodel.ComponentK8sAttributes
	if err := s.db.Where("component_id = ? AND name IN (?)", scope.ServiceID, []string{"vm_fixed_ip", "vm_fixed_ip_enabled", "vm_interface_mac_addresses"}).Order("name, ID").Find(&attrs).Error; err != nil {
		return "", err
	}
	metadata := map[string]string{}
	for _, a := range attrs {
		if a.TenantID != scope.TenantID {
			return "", ErrCorrupt
		}
		if _, ok := metadata[a.Name]; ok {
			return "", ErrCorrupt
		}
		metadata[a.Name] = a.AttributeValue
	}
	return snapshotDigest(struct {
		Scope      Scope
		Ports      []port
		Attributes map[string]string
	}{scope, values, metadata})
}

func (r *KubeRestoreRuntime) loadRaw(ctx context.Context, request RestoreRuntimeRequest) (Current, *kv.VirtualMachine, *unstructured.Unstructured, error) {
	if r == nil || r.Store == nil || r.Core == nil || r.Dynamic == nil || ctx.Err() != nil {
		return Current{}, nil, nil, ErrReferenceUnavailable
	}
	c, err := r.Store.Read(request.Scope, request.OperationID)
	if err != nil {
		return c, nil, nil, err
	}
	if restoreOwner(c, request.Scope, Expectation{OperationID: request.OperationID, ManifestHash: request.CoreHash}) != nil || c.Receipt.Execution == nil || c.State.BindingGeneration != request.BindingGeneration {
		return c, nil, nil, ErrConflict
	}
	object, err := r.Dynamic.Resource(restoreVMGVR).Namespace(request.Scope.Namespace).Get(ctx, request.VM.Name, metav1.GetOptions{})
	if err != nil {
		return c, nil, nil, ErrReferenceUnavailable
	}
	if !sameObject(request.VM, refOf("VirtualMachine", object)) || object.GetDeletionTimestamp() != nil || object.GetLabels()["service_id"] != request.Scope.ServiceID {
		return c, nil, nil, ErrConflict
	}
	if tenant := object.GetLabels()["tenant_id"]; tenant != "" && tenant != request.Scope.TenantID {
		return c, nil, nil, ErrConflict
	}
	var vm kv.VirtualMachine
	if runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, &vm) != nil || vm.Spec.Template == nil {
		return c, nil, nil, ErrCorrupt
	}
	return c, &vm, object, nil
}

func (r *KubeRestoreRuntime) load(ctx context.Context, request RestoreRuntimeRequest) (Current, *kv.VirtualMachine, error) {
	current, vm, _, err := r.loadRaw(ctx, request)
	return current, vm, err
}

func (r *KubeRestoreRuntime) writers(ctx context.Context, c Current) (bool, bool, error) {
	names := map[string]bool{}
	dvs := map[string]bool{}
	for _, set := range []map[string]Binding{c.Receipt.OldBindings, c.Receipt.NewBindings} {
		for _, b := range set {
			names[b.ActivePVC.Name] = true
			if b.PreviousPVC != nil {
				names[b.PreviousPVC.Name] = true
			}
			if b.ActiveDV != nil {
				dvs[b.ActiveDV.Name] = true
			}
			if b.PreviousDV != nil {
				dvs[b.PreviousDV.Name] = true
			}
		}
	}
	for _, v := range c.Receipt.Execution.Intents {
		if v.Object.Kind == "PersistentVolumeClaim" {
			names[v.Object.Name] = true
		}
	}
	instances, err := r.Dynamic.Resource(restoreVMIGVR).Namespace(c.State.Scope.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, false, ErrReferenceUnavailable
	}
	running, clear := false, true
	ownedCount := 0
	touchedInstances := map[string]bool{c.Receipt.Restore.CurrentVM.Name: true}
	for _, v := range instances.Items {
		owned := false
		for _, owner := range v.GetOwnerReferences() {
			owned = owned || (owner.Kind == "VirtualMachine" && owner.UID == types.UID(c.Receipt.Restore.CurrentVM.UID))
		}
		phase, _, _ := unstructured.NestedString(v.Object, "status", "phase")
		if owned {
			ownedCount++
			if phase == string(kv.Running) && v.GetDeletionTimestamp() == nil {
				running = true
			}
		}
		volumes, _, _ := unstructured.NestedSlice(v.Object, "spec", "volumes")
		touch := owned
		for _, raw := range volumes {
			m, _ := raw.(map[string]interface{})
			touch = touch || names[textAt(m, "persistentVolumeClaim", "claimName")] || dvs[textAt(m, "dataVolume", "name")]
		}
		if touch {
			touchedInstances[v.GetName()] = true
		}
		if touch && (v.GetDeletionTimestamp() != nil || (phase != string(kv.Succeeded) && phase != string(kv.Failed))) {
			clear = false
		}
	}
	if ownedCount != 1 {
		running = false
	}
	migrations, err := r.Dynamic.Resource(restoreMigrationGVR).Namespace(c.State.Scope.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, false, ErrReferenceUnavailable
	}
	for _, migration := range migrations.Items {
		if !touchedInstances[textAt(migration.Object, "spec", "vmiName")] {
			continue
		}
		phase := textAt(migration.Object, "status", "phase")
		if migration.GetDeletionTimestamp() != nil || (phase != "Succeeded" && phase != "Failed") {
			clear = false
		}
	}
	pods, err := r.Core.CoreV1().Pods(c.State.Scope.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return false, false, ErrReferenceUnavailable
	}
	for _, p := range pods.Items {
		for _, v := range p.Spec.Volumes {
			if v.PersistentVolumeClaim != nil && names[v.PersistentVolumeClaim.ClaimName] && (p.DeletionTimestamp != nil || (p.Status.Phase != corev1.PodSucceeded && p.Status.Phase != corev1.PodFailed)) {
				clear = false
			}
		}
	}
	return running, clear, nil
}

func (r *KubeRestoreRuntime) selection(ctx context.Context, c Current, request RestoreRuntimeRequest) (RuntimeSelection, error) {
	if r.Resolver == nil {
		return RuntimeSelection{}, ErrReferenceUnavailable
	}
	view, err := r.Resolver.ReadBindingView(ctx, request.Scope)
	if err != nil {
		return RuntimeSelection{}, err
	}
	if view.BindingGeneration != c.State.BindingGeneration {
		return RuntimeSelection{}, ErrConflict
	}
	config, err := r.Store.selectedRuntimeConfig(request.Scope)
	if err != nil {
		return RuntimeSelection{}, err
	}
	hash, err := RestoreConfigHash(config)
	if err != nil || hash != request.ConfigHash {
		return RuntimeSelection{}, ErrConflict
	}
	return RuntimeSelection{Scope: request.Scope, BindingGeneration: request.BindingGeneration, Bindings: cloneBindings(c.State.Bindings), Config: config, Startup: request.Startup, BeforeStartup: c.Receipt.Execution.BeforeStartup}, nil
}

// RuntimeVMMatches compares runtime fields using Kubernetes quantity semantics.
func RuntimeVMMatches(a, b *kv.VirtualMachine) bool {
	if a == nil || b == nil || a.Spec.Template == nil || b.Spec.Template == nil {
		return false
	}
	return apiequality.Semantic.DeepEqual(a.Spec.Template.Spec.Domain, b.Spec.Template.Spec.Domain) && apiequality.Semantic.DeepEqual(a.Spec.Template.Spec.Volumes, b.Spec.Template.Spec.Volumes) && apiequality.Semantic.DeepEqual(a.Spec.DataVolumeTemplates, b.Spec.DataVolumeTemplates)
}

// runtimeInstanceMatches reuses the approved safe hardware/layout projection.
// Guest memory is checked independently of KubeVirt launcher overhead/defaults.
func runtimeInstanceMatches(instance *kv.VirtualMachineInstance, vm *kv.VirtualMachine, selected RuntimeSelection) bool {
	if instance == nil || vm == nil || vm.Spec.Template == nil || instance.Spec.Domain.Firmware == nil || string(instance.Spec.Domain.Firmware.UUID) != selected.Startup.FirmwareUUID || !apiequality.Semantic.DeepEqual(instance.Spec.Volumes, vm.Spec.Template.Spec.Volumes) {
		return false
	}
	disks := []SnapshotDisk{}
	for _, layout := range selected.Config.Hardware.DiskLayout {
		b, ok := selected.Bindings[strconv.FormatUint(uint64(layout.VolumeID), 10)]
		if !ok || !b.Attached {
			return false
		}
		disks = append(disks, SnapshotDisk{VolumeID: b.VolumeID, DiskKey: b.DiskKey, CanonicalName: b.CanonicalName, Role: b.Role, SourcePVC: b.ActivePVC, SourceDV: b.ActiveDV, ReadOnly: layout.ReadOnly})
	}
	actual := &kv.VirtualMachine{Spec: kv.VirtualMachineSpec{Template: &kv.VirtualMachineInstanceTemplateSpec{Spec: instance.Spec}}}
	config, err := SnapshotSafeConfig(actual, disks)
	if err != nil {
		return false
	}
	observed := selected.Config
	observed.Hardware = config
	a, err := RestoreConfigHash(observed)
	if err != nil {
		return false
	}
	b, err := RestoreConfigHash(selected.Config)
	return err == nil && a == b
}

func (r *KubeRestoreRuntime) desired(ctx context.Context, c Current, request RestoreRuntimeRequest, vm *kv.VirtualMachine) (*kv.VirtualMachine, error) {
	selection, err := r.selection(ctx, c, request)
	if err != nil {
		return nil, err
	}
	base := vm
	if request.Startup.BootstrapMode == "platform" {
		if r.BuildPlatform == nil {
			return nil, ErrRestoreCapability
		}
		base, err = r.BuildPlatform(ctx, request.Scope)
		if err != nil {
			return nil, err
		}
		if base == nil || base.Spec.Template == nil {
			return nil, ErrCorrupt
		}
		base = base.DeepCopy()
		base.ObjectMeta = vm.ObjectMeta
		base.Spec.Template.Spec.Networks = vm.Spec.Template.Spec.Networks
		base.Spec.Template.Spec.Domain.Devices.Interfaces = vm.Spec.Template.Spec.Domain.Devices.Interfaces
	}
	return ProjectRestoredRuntime(ctx, r.Core, base, selection)
}

// Observe returns current evidence only; it cannot create identity or fix state.
func (r *KubeRestoreRuntime) Observe(ctx context.Context, request RestoreRuntimeRequest) (RestoreRuntimeObservation, error) {
	c, vm, err := r.load(ctx, request)
	if err != nil {
		return RestoreRuntimeObservation{}, err
	}
	network, err := r.Store.NetworkIdentityHash(request.Scope)
	if err != nil {
		return RestoreRuntimeObservation{}, err
	}
	running, clear, err := r.writers(ctx, c)
	if err != nil {
		return RestoreRuntimeObservation{}, err
	}
	o := RestoreRuntimeObservation{VM: refOf("VirtualMachine", vm), VMResourceVersion: vm.ResourceVersion, VMGeneration: vm.Generation, ObservedGeneration: vm.Status.ObservedGeneration, BindingGeneration: c.State.BindingGeneration, Halted: !vmWantsRunning(vm) && !running && clear, Running: vmWantsRunning(vm) && running, WritersClear: clear, NetworkHash: network}
	if request.Action == "halt" || request.Action == "rollback-halt" {
		return o, nil
	}
	wanted, err := r.desired(ctx, c, request, vm)
	if err != nil {
		return o, err
	}
	o.DesiredMatches = RuntimeVMMatches(vm, wanted) && vm.Status.ObservedGeneration >= vm.Generation
	if o.DesiredMatches && vm.Spec.Template.Spec.Domain.Firmware != nil && string(vm.Spec.Template.Spec.Domain.Firmware.UUID) == request.Startup.FirmwareUUID {
		o.IdentityHash, _ = snapshotDigest(request.Startup)
	}
	if o.Running {
		selection, err := r.selection(ctx, c, request)
		if err != nil {
			return o, err
		}
		instances, err := r.Dynamic.Resource(restoreVMIGVR).Namespace(request.Scope.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return o, ErrReferenceUnavailable
		}
		matches := 0
		for _, object := range instances.Items {
			for _, owner := range object.GetOwnerReferences() {
				if owner.Kind != "VirtualMachine" || owner.UID != vm.UID {
					continue
				}
				var instance kv.VirtualMachineInstance
				if runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, &instance) != nil || instance.DeletionTimestamp != nil || instance.Status.Phase != kv.Running || !runtimeInstanceMatches(&instance, wanted, selection) {
					o.Running = false
					return o, nil
				}
				matches++
			}
		}
		o.Running = matches == 1
	}
	return o, nil
}

// Apply requires the already-persisted exact action intent and issued witness.
func (r *KubeRestoreRuntime) Apply(ctx context.Context, request RestoreRuntimeRequest) (RestoreRuntimeObservation, error) {
	c, vm, err := r.load(ctx, request)
	if err != nil {
		return RestoreRuntimeObservation{}, err
	}
	hash, _ := snapshotDigest(request)
	if step := c.Receipt.Steps["runtime-issued-"+request.Action]; !step.Applied || step.RequestHash != hash {
		return RestoreRuntimeObservation{}, ErrConflict
	}
	updated := vm.DeepCopy()
	switch request.Action {
	case "halt", "rollback-halt":
		strategy := kv.RunStrategyHalted
		updated.Spec.Running = nil
		updated.Spec.RunStrategy = &strategy
	case "sync-halted", "rollback-sync-halted":
		_, clear, err := r.writers(ctx, c)
		if err != nil || !clear {
			if err != nil {
				return RestoreRuntimeObservation{}, err
			}
			return RestoreRuntimeObservation{}, ErrRestorePending
		}
		if vmWantsRunning(vm) {
			return RestoreRuntimeObservation{}, ErrRestorePending
		}
		updated, err = r.desired(ctx, c, request, updated)
		if err != nil {
			return RestoreRuntimeObservation{}, err
		}
		strategy := kv.RunStrategyHalted
		updated.Spec.Running = nil
		updated.Spec.RunStrategy = &strategy
	case "resume", "rollback-resume":
		wanted, err := r.desired(ctx, c, request, vm)
		if err != nil {
			return RestoreRuntimeObservation{}, err
		}
		if !RuntimeVMMatches(vm, wanted) || vm.Status.ObservedGeneration < vm.Generation {
			return RestoreRuntimeObservation{}, ErrConflict
		}
		strategy := kv.RunStrategyAlways
		updated.Spec.Running = nil
		updated.Spec.RunStrategy = &strategy
	default:
		return RestoreRuntimeObservation{}, ErrCorrupt
	}
	if updated.ResourceVersion == "" {
		return RestoreRuntimeObservation{}, ErrConflict
	}
	latest, observed, raw, err := r.loadRaw(ctx, request)
	if err != nil {
		return RestoreRuntimeObservation{}, err
	}
	if observed.ResourceVersion != vm.ResourceVersion {
		return RestoreRuntimeObservation{}, ErrConflict
	}
	network, err := r.Store.NetworkIdentityHash(request.Scope)
	if err != nil {
		return RestoreRuntimeObservation{}, err
	}
	if network != latest.Receipt.Execution.NetworkHash {
		return RestoreRuntimeObservation{}, ErrConflict
	}
	body, err := PreserveRuntimeVM(raw, vm, updated)
	if err != nil {
		return RestoreRuntimeObservation{}, err
	}
	guard, err := r.Store.Read(request.Scope, request.OperationID)
	if err != nil {
		return RestoreRuntimeObservation{}, err
	}
	if restoreOwner(guard, request.Scope, Expectation{OperationID: request.OperationID, ManifestHash: request.CoreHash}) != nil || guard.State.ControlRevision != latest.State.ControlRevision || guard.State.BindingGeneration != request.BindingGeneration {
		return RestoreRuntimeObservation{}, ErrConflict
	}
	if _, err = r.Dynamic.Resource(restoreVMGVR).Namespace(request.Scope.Namespace).Update(ctx, body, metav1.UpdateOptions{}); err != nil {
		return RestoreRuntimeObservation{}, ErrRestoreUnknown
	}
	return RestoreRuntimeObservation{}, nil
}
