package handler

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	"github.com/goodrain/rainbond/pkg/vmstorage"
	appmvolume "github.com/goodrain/rainbond/worker/appm/volume"
	"github.com/jinzhu/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	v1 "kubevirt.io/api/core/v1"
)

// ExistingSnapshotDisk is an authoritative logical participant and its storage result.
type ExistingSnapshotDisk struct {
	vmstorage.VolumeCapability
	VolumeID uint   `json:"volume_id,omitempty"`
	DiskKey  string `json:"disk_key,omitempty"`
	Role     string `json:"role,omitempty"`
	ReadOnly bool   `json:"read_only"`
}

// ExcludedSnapshotVolume makes retained/detached and installation-media exclusions explicit.
type ExcludedSnapshotVolume struct {
	Key        string `json:"key"`
	Role       string `json:"role"`
	ReasonCode string `json:"reason_code"`
}

// ExistingVMSnapshotCapability is only for creating a new snapshot of the current VM.
// Restore-plan capability must assess that snapshot's participants/target plan separately.
type ExistingVMSnapshotCapability struct {
	vmstorage.Capability
	Operation         string                   `json:"operation"`
	Complete          bool                     `json:"complete"`
	Volumes           []ExistingSnapshotDisk   `json:"volumes"`
	ExcludedVolumes   []ExcludedSnapshotVolume `json:"excluded_volumes"`
	ControlRevision   uint64                   `json:"control_revision"`
	BindingGeneration uint64                   `json:"binding_generation"`
}

func existingSnapshotUnknown(code, message string) vmstorage.Capability {
	return vmstorage.Capability{Status: "unknown", ReasonCode: code, Message: message}
}

// GetExistingVMSnapshotCapability reads the complete current binding set in the authenticated component scope.
// It never creates bindings, selects retained B, or accepts namespace/claim overrides.
func (s *ServiceAction) GetExistingVMSnapshotCapability(ctx context.Context, tenantID, serviceID string) (*ExistingVMSnapshotCapability, error) {
	out := &ExistingVMSnapshotCapability{Capability: existingSnapshotUnknown("binding_view_unavailable", "无法核验当前完整磁盘集合"), Operation: "create_snapshot", Volumes: []ExistingSnapshotDisk{}, ExcludedVolumes: []ExcludedSnapshotVolume{}}
	if tenantID == "" || serviceID == "" {
		return nil, bcode.NewBadRequest("缺少组件鉴权上下文")
	}
	if s == nil || s.getDBManager() == nil {
		return out, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	service, err := s.getDBManager().TenantServiceDao().GetServiceByID(serviceID)
	if gorm.IsRecordNotFoundError(err) || (err == nil && (service == nil || service.TenantID != tenantID)) {
		return nil, &bcode.Code{Status: 404, Code: 404, Message: "组件不存在"}
	}
	if err != nil {
		return nil, &bcode.Code{Status: 503, Code: 503, Message: "无法核验组件归属"}
	}
	if !service.IsVM() {
		return nil, bcode.NewBadRequest("此组件不是虚拟机")
	}
	consumer, err := s.vmVolumes(service)
	if err != nil {
		return out, nil
	}
	view, err := consumer.resolver.ReadBindingView(ctx, consumer.scope)
	if err != nil {
		return out, nil
	}
	out.ControlRevision = view.ControlRevision
	out.BindingGeneration = view.BindingGeneration
	dyn := vmHotplugDynamicClient()
	objects, err := dyn.Resource(vmDiskVMGVR).Namespace(consumer.scope.Namespace).List(ctx, metav1.ListOptions{LabelSelector: "service_id=" + serviceID})
	if err != nil || len(objects.Items) != 1 {
		out.Capability = existingSnapshotUnknown("vm_identity_unavailable", "未能唯一读取该组件的虚拟机身份")
		return out, nil
	}
	var vm v1.VirtualMachine
	if runtime.DefaultUnstructuredConverter.FromUnstructured(objects.Items[0].Object, &vm) != nil || vm.UID == "" || vm.Spec.Template == nil || vm.Namespace != consumer.scope.Namespace || vm.Labels["service_id"] != serviceID || vm.DeletionTimestamp != nil {
		out.Capability = existingSnapshotUnknown("vm_identity_unavailable", "虚拟机身份或规格无法核验")
		return out, nil
	}
	mounted := map[string]v1.Volume{}
	for _, v := range vm.Spec.Template.Spec.Volumes {
		if _, duplicate := mounted[v.Name]; duplicate || v.Name == "" {
			out.Capability = existingSnapshotUnknown("volume_set_invalid", "虚拟机磁盘集合身份缺失或重复")
			return out, nil
		}
		mounted[v.Name] = v
	}
	inspector := vmstorage.NewInspector(s.kubeClient, dyn)
	seen := map[string]bool{}
	out.Complete = true
	for _, resolved := range view.Volumes {
		key := resolved.CanonicalName
		actual, present := mounted[key]
		seen[key] = true
		disk := ExistingSnapshotDisk{VolumeCapability: vmstorage.VolumeCapability{Key: key, Capability: existingSnapshotUnknown("participant_not_attached", "当前磁盘未在虚拟机规格中就绪")}, VolumeID: resolved.VolumeID, DiskKey: resolved.DiskKey, Role: resolved.Role, ReadOnly: resolved.ReadOnly}
		if !resolved.Attached {
			if present {
				disk.Capability = existingSnapshotUnknown("detached_volume_still_mounted", "已脱离磁盘仍出现在当前虚拟机规格")
				out.Volumes = append(out.Volumes, disk)
				out.Complete = false
			} else {
				out.ExcludedVolumes = append(out.ExcludedVolumes, ExcludedSnapshotVolume{Key: key, Role: resolved.Role, ReasonCode: "detached_retained_volume"})
			}
			continue
		}
		if !present {
			out.Volumes = append(out.Volumes, disk)
			out.Complete = false
			continue
		}
		row, rowErr := s.getDBManager().TenantServiceVolumeDao().GetVolumeByServiceIDAndName(serviceID, resolved.DiskKey)
		if rowErr != nil || row == nil || row.ID != resolved.VolumeID {
			disk.Capability = existingSnapshotUnknown("volume_metadata_unavailable", "无法核验磁盘的设备角色")
			out.Volumes = append(out.Volumes, disk)
			out.Complete = false
			continue
		}
		if resolved.ActivePVC == nil {
			disk.Capability = existingSnapshotUnknown("pvc_not_ready", "当前绑定PVC尚未就绪")
			out.Volumes = append(out.Volumes, disk)
			continue
		}
		mountMatches := actual.PersistentVolumeClaim != nil && actual.PersistentVolumeClaim.ClaimName == resolved.ActivePVC.Name
		if actual.DataVolume != nil && resolved.ActiveDV != nil && actual.DataVolume.Name == resolved.ActiveDV.Name {
			mountMatches = true
		}
		if !mountMatches {
			disk.Capability = existingSnapshotUnknown("binding_mount_mismatch", "实际挂载与权威当前绑定不一致")
			out.Volumes = append(out.Volumes, disk)
			out.Complete = false
			continue
		}
		pvc, readErr := s.kubeClient.CoreV1().PersistentVolumeClaims(consumer.scope.Namespace).Get(ctx, resolved.ActivePVC.Name, metav1.GetOptions{})
		if readErr != nil || vmrestore.CheckLivePVC(*resolved.ActivePVC, pvcNamespace(pvc), pvcName(pvc), pvcUID(pvc)) != nil {
			disk.Capability = existingSnapshotUnknown("pvc_identity_unverified", "无法核验当前绑定PVC的实际UID")
		} else if resolved.Role != "root" && appmvolume.VMVolumeDeviceType(row.VolumePath) == "cdrom" && vmstorage.ReadOnlyCDROM(&vm, key) {
			out.ExcludedVolumes = append(out.ExcludedVolumes, ExcludedSnapshotVolume{Key: key, Role: "installation_media", ReasonCode: "read_only_installation_media"})
			continue
		} else {
			disk.Capability = vmstorage.ManagedSnapshotCapability(inspector.PersistentVolumeClaim(ctx, pvc))
		}
		out.Volumes = append(out.Volumes, disk)
	}
	for _, v := range vm.Spec.Template.Spec.Volumes {
		if seen[v.Name] {
			continue
		}
		if vmstorage.AuxiliaryVolume(v) {
			out.ExcludedVolumes = append(out.ExcludedVolumes, ExcludedSnapshotVolume{Key: v.Name, Role: "auxiliary", ReasonCode: "auxiliary_media"})
			continue
		}
		if vmstorage.ReadOnlyInstallationMedia(&vm, v) {
			out.ExcludedVolumes = append(out.ExcludedVolumes, ExcludedSnapshotVolume{Key: v.Name, Role: "installation_media", ReasonCode: "read_only_installation_media"})
			continue
		}
		out.Volumes = append(out.Volumes, ExistingSnapshotDisk{VolumeCapability: vmstorage.VolumeCapability{Key: v.Name, Capability: existingSnapshotUnknown("participant_unregistered", "该实际参与盘不在权威完整绑定集合中")}})
		out.Complete = false
	}
	caps := make([]vmstorage.VolumeCapability, 0, len(out.Volumes))
	for _, v := range out.Volumes {
		caps = append(caps, v.VolumeCapability)
	}
	out.Capability = vmstorage.Aggregate(caps).Capability
	for _, disk := range out.Volumes {
		if disk.ReasonCode == "managed_snapshot_driver_unsupported" {
			out.ReasonCode, out.Message = disk.ReasonCode, disk.Message
			break
		}
	}
	current, err := consumer.store.ReadOnly(consumer.scope, "")
	if err != nil || current.State.ControlRevision != view.ControlRevision || current.State.BindingGeneration != view.BindingGeneration {
		out.Capability = existingSnapshotUnknown("binding_view_changed", "绑定代际在查询中改变，请刷新核验")
		out.Complete = false
		return out, nil
	}
	latest, err := dyn.Resource(vmDiskVMGVR).Namespace(consumer.scope.Namespace).Get(ctx, vm.Name, metav1.GetOptions{})
	if err != nil || latest.GetUID() != vm.UID || latest.GetResourceVersion() != vm.ResourceVersion {
		out.Capability = existingSnapshotUnknown("vm_changed_during_check", "虚拟机身份或规格在查询中改变")
		out.Complete = false
		return out, nil
	}
	if view.Fence != nil || view.MutationInProgress {
		out.Capability = existingSnapshotUnknown("component_operation_in_progress", "组件正在执行其它受控操作，当前不能确认新快照可用")
	}
	return out, nil
}

func pvcNamespace(p *corev1.PersistentVolumeClaim) string {
	if p == nil {
		return ""
	}
	return p.Namespace
}
func pvcName(p *corev1.PersistentVolumeClaim) string {
	if p == nil {
		return ""
	}
	return p.Name
}
func pvcUID(p *corev1.PersistentVolumeClaim) string {
	if p == nil {
		return ""
	}
	return string(p.UID)
}
