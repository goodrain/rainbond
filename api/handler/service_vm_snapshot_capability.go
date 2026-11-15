package handler

import (
	"context"
	"errors"
	"time"

	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/goodrain/rainbond/pkg/vmstorage"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	v1 "kubevirt.io/api/core/v1"
)

// ExistingSnapshotDisk is one actual VM disk and its current storage result.
type ExistingSnapshotDisk struct {
	vmstorage.VolumeCapability
	DiskKey  string `json:"disk_key,omitempty"`
	Role     string `json:"role,omitempty"`
	ReadOnly bool   `json:"read_only"`
}

// ExcludedSnapshotVolume makes non-persistent media exclusions explicit.
type ExcludedSnapshotVolume struct {
	Key        string `json:"key"`
	Role       string `json:"role"`
	ReasonCode string `json:"reason_code"`
}

// ExistingVMSnapshotCapability is a read-only view of the VM's current disks.
type ExistingVMSnapshotCapability struct {
	vmstorage.Capability
	Operation       string                   `json:"operation"`
	Complete        bool                     `json:"complete"`
	Volumes         []ExistingSnapshotDisk   `json:"volumes"`
	ExcludedVolumes []ExcludedSnapshotVolume `json:"excluded_volumes"`
}

func existingSnapshotUnknown(code, message string) vmstorage.Capability {
	return vmstorage.Capability{Status: "unknown", ReasonCode: code, Message: message}
}

// GetExistingVMSnapshotCapability inspects only the authenticated component's
// actual VM and mounted disk sources. It never creates resources or chooses a
// replacement PVC.
func (s *ServiceAction) GetExistingVMSnapshotCapability(ctx context.Context, tenantID, serviceID string) (*ExistingVMSnapshotCapability, error) {
	out := &ExistingVMSnapshotCapability{Capability: existingSnapshotUnknown("vm_identity_unavailable", "无法核验当前虚拟机身份"), Operation: "create_snapshot", Volumes: []ExistingSnapshotDisk{}, ExcludedVolumes: []ExcludedSnapshotVolume{}}
	if tenantID == "" || serviceID == "" {
		return nil, bcode.NewBadRequest("缺少组件鉴权上下文")
	}
	if s == nil || s.getDBManager() == nil {
		return out, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	scope, err := s.snapshotScope(ctx, serviceID)
	if err != nil || scope.TenantID != tenantID {
		if errors.Is(err, vmrestore.ErrNotFound) || errors.Is(err, vmrestore.ErrInternalAttribute) || scope.TenantID != tenantID {
			return nil, &bcode.Code{Status: 404, Code: 404, Message: "组件不存在"}
		}
		return out, nil
	}
	if s.kubeClient == nil {
		out.Capability = existingSnapshotUnknown("capability_client_unavailable", "存储能力查询客户端不可用")
		return out, nil
	}
	dynamicClient := vmHotplugDynamicClient()
	if dynamicClient == nil {
		out.Capability = existingSnapshotUnknown("capability_client_unavailable", "存储能力查询客户端不可用")
		return out, nil
	}
	objects, err := dynamicClient.Resource(vmDiskVMGVR).Namespace(scope.Namespace).List(ctx, metav1.ListOptions{LabelSelector: "service_id=" + serviceID})
	if err != nil || len(objects.Items) != 1 {
		return out, nil
	}
	var vm v1.VirtualMachine
	if runtime.DefaultUnstructuredConverter.FromUnstructured(objects.Items[0].Object, &vm) != nil || vm.UID == "" || vm.Spec.Template == nil || vm.Namespace != scope.Namespace || vm.Labels["service_id"] != serviceID || vm.DeletionTimestamp != nil {
		return out, nil
	}

	for _, volume := range vm.Spec.Template.Spec.Volumes {
		switch {
		case vmstorage.ReadOnlyInstallationMedia(&vm, volume):
			out.ExcludedVolumes = append(out.ExcludedVolumes, ExcludedSnapshotVolume{Key: volume.Name, Role: "installation_media", ReasonCode: "read_only_installation_media"})
		case vmstorage.AuxiliaryVolume(volume):
			out.ExcludedVolumes = append(out.ExcludedVolumes, ExcludedSnapshotVolume{Key: volume.Name, Role: "auxiliary", ReasonCode: "auxiliary_media"})
		}
	}

	native := vmstorage.NewInspector(s.kubeClient, dynamicClient).VirtualMachine(ctx, &vm)
	managed := make([]vmstorage.VolumeCapability, 0, len(native.Volumes))
	for _, volume := range native.Volumes {
		capability := vmstorage.ManagedSnapshotCapability(volume.Capability)
		managed = append(managed, vmstorage.VolumeCapability{Key: volume.Key, Capability: capability})
		out.Volumes = append(out.Volumes, ExistingSnapshotDisk{VolumeCapability: vmstorage.VolumeCapability{Key: volume.Key, Capability: capability}, DiskKey: volume.Key})
	}
	out.Capability = vmstorage.Aggregate(managed).Capability
	out.Complete = true

	latest, err := dynamicClient.Resource(vmDiskVMGVR).Namespace(scope.Namespace).Get(ctx, vm.Name, metav1.GetOptions{})
	if err != nil || latest.GetUID() != vm.UID || latest.GetResourceVersion() != vm.ResourceVersion {
		out.Capability = existingSnapshotUnknown("vm_changed_during_check", "虚拟机身份或规格在查询中改变")
		out.Complete = false
	}
	return out, nil
}
