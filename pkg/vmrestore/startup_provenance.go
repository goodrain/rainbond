package vmrestore

import (
	"context"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	kv "kubevirt.io/api/core/v1"
)

const startupProducerAnnotation = "rainbond.io/vm-startup-producer"

// VMStartupHints is emitted by the platform renderer, never from a request body.
// It identifies generated initialization devices without including their data.
type VMStartupHints struct {
	NoCloudVolumeName string
	NoCloudDiskName   string
	ConfigMaps        []VMStartupConfigMap
}

// VMStartupConfigMap identifies an explicit renderer-owned environment device.
type VMStartupConfigMap struct{ VolumeName, DiskName, Name string }

// VMStartupProof binds a successful platform publication to exact live objects.
type VMStartupProof struct {
	VM     ObjectRef            `json:"vm"`
	Policy RestoreStartupPolicy `json:"policy"`
}

func (p VMStartupProof) validate(scope Scope) error {
	if p.VM.Kind != "VirtualMachine" || p.VM.Namespace != scope.Namespace || !validName(p.VM.Name) || !validUID(p.VM.UID) || p.Policy.validate(scope) != nil {
		return ErrCorrupt
	}
	return nil
}

func hasNoCloudSecretReferences(source *kv.CloudInitNoCloudSource) bool {
	return source != nil && (source.UserDataSecretRef != nil || source.NetworkDataSecretRef != nil)
}

// checkStartupDevices checks identities only; it never reads guest or media data.
func checkStartupDevices(policy RestoreStartupPolicy, spec kv.VirtualMachineInstanceSpec) error {
	if len(spec.AccessCredentials) != 0 {
		return ErrConflict
	}
	volumes := map[string]kv.Volume{}
	clouds := 0
	for _, volume := range spec.Volumes {
		if _, exists := volumes[volume.Name]; exists {
			return ErrConflict
		}
		volumes[volume.Name] = volume
		if volume.CloudInitConfigDrive != nil || volume.Sysprep != nil {
			return ErrConflict
		}
		if volume.CloudInitNoCloud != nil {
			// Private startup proofs cover inline NoCloud data only. A SecretRef
			// introduces a different source whose identity was never sealed.
			if hasNoCloudSecretReferences(volume.CloudInitNoCloud) {
				return ErrConflict
			}
			clouds++
			if policy.DataSourceKind != "CloudInitNoCloud" || volume.Name != policy.NoCloudVolumeName {
				return ErrConflict
			}
		}
	}
	disks := map[string]bool{}
	for _, disk := range spec.Domain.Devices.Disks {
		if disks[disk.Name] {
			return ErrConflict
		}
		disks[disk.Name] = true
	}
	if policy.DataSourceKind == "CloudInitNoCloud" {
		if clouds != 1 || policy.NoCloudVolumeName != policy.NoCloudDiskName || !disks[policy.NoCloudDiskName] {
			return ErrConflict
		}
	} else if policy.DataSourceKind != "None" || clouds != 0 {
		return ErrConflict
	}
	for _, media := range policy.GeneratedMedia {
		volume, ok := volumes[media.VolumeName]
		if !ok || media.VolumeName != media.DiskName || !disks[media.DiskName] {
			return ErrConflict
		}
		if media.Source.Kind == "ConfigMap" && volume.ConfigMap != nil && volume.ConfigMap.Name == media.Source.Name {
			continue
		}
		if media.Source.Kind == "Secret" && volume.Secret != nil && volume.Secret.SecretName == media.Source.Name {
			continue
		}
		return ErrConflict
	}
	return nil
}

func (s *Store) validateStartupReceipt(query *gorm.DB, state State, scope Scope) error {
	if state.StartupOperationID == "" {
		return nil
	}
	key, err := ReceiptKey(state.StartupOperationID)
	if err != nil {
		return err
	}
	var rows []dbmodel.ComponentK8sAttributes
	if err := query.Where("component_id = ? AND name = ?", scope.ServiceID, key).Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) != 1 {
		return ErrCorrupt
	}
	receipt, err := decodeStoredReceipt(rows[0], scope, state.StartupOperationID)
	if err != nil {
		return err
	}
	if receipt.Startup == nil || !receipt.Steps["startup-published"].Applied {
		return ErrCorrupt
	}
	return validateSuccessfulSteps(receipt, state)
}

// VMStartupPublication is an in-memory new-VM draft. It carries no guest data.
type VMStartupPublication struct {
	OperationID  string
	Scope        Scope
	Hints        VMStartupHints
	FirmwareUUID string
}

// PrepareNewVMStartup assigns a new guest identity only on a new VM object and
// marks exactly the renderer-declared media before their normal publication.
func PrepareNewVMStartup(scope Scope, vm *kv.VirtualMachine, hints VMStartupHints) (*VMStartupPublication, error) {
	if !validScope(scope) || vm == nil || vm.UID != "" || vm.Spec.Template == nil || vm.Namespace != scope.Namespace || !validName(vm.Name) {
		return nil, ErrConflict
	}
	if (hints.NoCloudVolumeName == "") != (hints.NoCloudDiskName == "") {
		return nil, ErrCorrupt
	}
	for _, volume := range vm.Spec.Template.Spec.Volumes {
		if hasNoCloudSecretReferences(volume.CloudInitNoCloud) {
			return nil, ErrRestoreCapability
		}
	}
	if hints.NoCloudVolumeName != "" {
		found := false
		for _, v := range vm.Spec.Template.Spec.Volumes {
			if v.Name == hints.NoCloudVolumeName && v.CloudInitNoCloud != nil {
				found = true
			}
		}
		if !found {
			return nil, ErrRestoreCapability
		}
	}
	if len(vm.Spec.Template.Spec.AccessCredentials) != 0 {
		return nil, ErrRestoreCapability
	}
	if vm.Spec.Template.Spec.Domain.Firmware == nil {
		vm.Spec.Template.Spec.Domain.Firmware = &kv.Firmware{}
	}
	if vm.Spec.Template.Spec.Domain.Firmware.UUID == "" {
		vm.Spec.Template.Spec.Domain.Firmware.UUID = types.UID(uuid.NewString())
	}
	if _, err := uuid.Parse(string(vm.Spec.Template.Spec.Domain.Firmware.UUID)); err != nil {
		return nil, ErrCorrupt
	}
	op := "startup-" + uuid.NewString()
	if vm.Annotations == nil {
		vm.Annotations = map[string]string{}
	}
	vm.Annotations[startupProducerAnnotation] = op
	return &VMStartupPublication{OperationID: op, Scope: scope, Hints: hints, FirmwareUUID: string(vm.Spec.Template.Spec.Domain.Firmware.UUID)}, nil
}

// ProducerID returns the marker for exactly this in-memory publication.
func (p VMStartupPublication) ProducerID() string { return p.OperationID }

// MarkConfigMap marks only an explicitly renderer-owned publication object.
func (p VMStartupPublication) MarkConfigMap(cm *corev1.ConfigMap) bool {
	for _, media := range p.Hints.ConfigMaps {
		if cm != nil && media.Name == cm.Name {
			if cm.Annotations == nil {
				cm.Annotations = map[string]string{}
			}
			cm.Annotations[startupProducerAnnotation] = p.OperationID
			return true
		}
	}
	return false
}

// PreserveStartupMediaMetadata retains existing producer ownership on updates.
func PreserveStartupMediaMetadata(existing, wanted *corev1.ConfigMap) {
	if existing == nil || wanted == nil || existing.Name != wanted.Name {
		return
	}
	if marker := existing.Annotations[startupProducerAnnotation]; marker != "" {
		if wanted.Annotations == nil {
			wanted.Annotations = map[string]string{}
		}
		wanted.Annotations[startupProducerAnnotation] = marker
	}
}

// SealNewVMStartup records only a successful exact-UID publication in the
// private Region ledger. An annotation alone is never source authority.
func (s *Store) SealNewVMStartup(ctx context.Context, core kubernetes.Interface, vm *kv.VirtualMachine, p VMStartupPublication) error {
	if core == nil || vm == nil || vm.Spec.Template == nil || vm.Spec.Template.Spec.Domain.Firmware == nil || string(vm.Spec.Template.Spec.Domain.Firmware.UUID) != p.FirmwareUUID || vm.Annotations[startupProducerAnnotation] != p.OperationID {
		return ErrConflict
	}
	policy := RestoreStartupPolicy{FirmwareUUID: p.FirmwareUUID, BootstrapMode: "platform", DataSourceKind: "None", ProducerUID: p.OperationID, GeneratedMedia: []RestoreGeneratedMedia{}}
	if p.Hints.NoCloudVolumeName != "" {
		policy.DataSourceKind = "CloudInitNoCloud"
		policy.NoCloudVolumeName = p.Hints.NoCloudVolumeName
		policy.NoCloudDiskName = p.Hints.NoCloudDiskName
	}
	for _, media := range p.Hints.ConfigMaps {
		cm, err := core.CoreV1().ConfigMaps(p.Scope.Namespace).Get(ctx, media.Name, metav1.GetOptions{})
		if err != nil {
			return ErrReferenceUnavailable
		}
		if !validUID(string(cm.UID)) || cm.Annotations[startupProducerAnnotation] != p.OperationID {
			return ErrConflict
		}
		policy.GeneratedMedia = append(policy.GeneratedMedia, RestoreGeneratedMedia{VolumeName: media.VolumeName, DiskName: media.DiskName, Source: refOf("ConfigMap", cm), ProducerUID: p.OperationID})
	}
	proof := VMStartupProof{VM: refOf("VirtualMachine", vm), Policy: policy}
	if proof.validate(p.Scope) != nil {
		return ErrCorrupt
	}
	if err := checkStartupDevices(policy, vm.Spec.Template.Spec); err != nil {
		return err
	}
	c, err := s.Read(p.Scope, p.OperationID)
	if err != nil {
		return err
	}
	hash, _ := snapshotDigest(proof)
	_, err = s.Mutate(p.Scope, Expectation{ControlRevision: c.State.ControlRevision, BindingGeneration: c.State.BindingGeneration, OperationID: p.OperationID, ManifestHash: hash, RequestHash: hash, Step: "startup-published"}, func(c Current) (Mutation, error) {
		if c.State.Fence != nil || c.State.Admission != nil || c.State.StartupOperationID != "" || c.Receipt != nil {
			return Mutation{}, ErrBusy
		}
		r := NewReceipt(p.Scope, p.OperationID, hash)
		r.Startup = &proof
		r.Resources = []ResourceRef{{ObjectRef: proof.VM, Purpose: "startup-vm"}}
		for _, media := range policy.GeneratedMedia {
			r.Resources = append(r.Resources, ResourceRef{ObjectRef: media.Source, Purpose: "startup-media"})
		}
		c.State.StartupOperationID = p.OperationID
		return Mutation{State: c.State, Receipt: &r}, nil
	})
	return err
}

// ReadVMStartup requires the private publication receipt and exact live UID.
// It never adopts a caller marker, copied label, old name, or unknown producer.
func (s *Store) ReadVMStartup(ctx context.Context, core kubernetes.Interface, scope Scope, vm *kv.VirtualMachine) (RestoreStartupPolicy, error) {
	c, err := s.Read(scope, "")
	if err != nil {
		return RestoreStartupPolicy{}, err
	}
	if c.State.StartupOperationID == "" {
		return RestoreStartupPolicy{}, ErrRestoreCapability
	}
	c, err = s.Read(scope, c.State.StartupOperationID)
	if err != nil {
		return RestoreStartupPolicy{}, err
	}
	if c.Receipt == nil || c.Receipt.Startup == nil || core == nil || vm == nil || !sameObject(c.Receipt.Startup.VM, refOf("VirtualMachine", vm)) {
		return RestoreStartupPolicy{}, ErrConflict
	}
	proof := c.Receipt.Startup
	policy := proof.Policy
	if vm.Annotations[startupProducerAnnotation] != policy.ProducerUID || vm.Spec.Template == nil || vm.Spec.Template.Spec.Domain.Firmware == nil || string(vm.Spec.Template.Spec.Domain.Firmware.UUID) != policy.FirmwareUUID || len(vm.Spec.Template.Spec.AccessCredentials) != 0 {
		return RestoreStartupPolicy{}, ErrConflict
	}
	if err := checkStartupDevices(policy, vm.Spec.Template.Spec); err != nil {
		return RestoreStartupPolicy{}, err
	}
	for _, media := range policy.GeneratedMedia {
		cm, err := core.CoreV1().ConfigMaps(scope.Namespace).Get(ctx, media.Source.Name, metav1.GetOptions{})
		if err != nil {
			return RestoreStartupPolicy{}, ErrReferenceUnavailable
		}
		if !sameObject(media.Source, refOf("ConfigMap", cm)) || cm.Annotations[startupProducerAnnotation] != media.ProducerUID {
			return RestoreStartupPolicy{}, ErrConflict
		}
	}
	return policy, nil
}
