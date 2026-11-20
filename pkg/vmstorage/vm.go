package vmstorage

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	v1 "kubevirt.io/api/core/v1"
)

// UnavailableError rejects snapshot submission while preserving machine-readable per-disk reasons.
type UnavailableError struct{ Capability SnapshotSet }

func (e *UnavailableError) Error() string {
	return fmt.Sprintf("%s: %s", e.Capability.ReasonCode, e.Capability.Message)
}

// VirtualMachine inspects every persistent volume actually declared on a VM.
// It observes backing metadata only; it never creates/falls back to a new disk or publishes a binding.
// Managed producers must also validate their authoritative Resolver view and fence before mutation.
func (i *Inspector) VirtualMachine(ctx context.Context, vm *v1.VirtualMachine) SnapshotSet {
	out := SnapshotSet{Capability: unknown("volume_set_empty", "没有可核验的快照参与磁盘"), Volumes: []VolumeCapability{}}
	if vm == nil || vm.Spec.Template == nil || vm.Namespace == "" || vm.Name == "" || vm.UID == "" {
		out.Capability = unknown("vm_identity_unavailable", "虚拟机身份或规格不可用")
		return out
	}
	if i == nil || i.core == nil || i.dynamic == nil {
		out.Capability = unknown("capability_client_unavailable", "存储能力查询客户端不可用")
		return out
	}
	for _, v := range vm.Spec.Template.Spec.Volumes {
		if ReadOnlyInstallationMedia(vm, v) {
			continue
		}
		var cap Capability
		claim := ""
		dvUID := ""
		switch {
		case v.PersistentVolumeClaim != nil:
			claim = v.PersistentVolumeClaim.ClaimName
		case v.DataVolume != nil:
			dv, err := i.dynamic.Resource(schema.GroupVersionResource{Group: "cdi.kubevirt.io", Version: "v1beta1", Resource: "datavolumes"}).Namespace(vm.Namespace).Get(ctx, v.DataVolume.Name, metav1.GetOptions{})
			if err != nil {
				cap = unknown("data_volume_read_failed", "无法核验数据卷对应的实际PVC")
				break
			}
			if dv.GetDeletionTimestamp() != nil || dv.GetUID() == "" {
				cap = unknown("data_volume_not_ready", "数据卷未就绪")
				break
			}
			dvUID = string(dv.GetUID())
			var errField error
			claim, _, errField = unstructured.NestedString(dv.Object, "status", "claimName")
			if errField != nil {
				cap = unknown("data_volume_invalid", "数据卷PVC引用格式不正确")
				break
			}
			if claim == "" {
				claim = dv.GetName()
			}
		case v.ContainerDisk != nil || v.EmptyDisk != nil || v.Ephemeral != nil || v.HostDisk != nil:
			cap = decision("unsupported", "ephemeral_disk_not_snapshotable", "该磁盘不是可持久化CSI快照盘，不能静默从整机快照排除")
		case AuxiliaryVolume(v):
			continue // Explicit auxiliary media, not persistent user disks.
		default:
			cap = unknown("volume_source_unverified", "无法核验该磁盘来源，不能静默从整机快照排除")
		}
		if claim != "" {
			pvc, err := i.core.CoreV1().PersistentVolumeClaims(vm.Namespace).Get(ctx, claim, metav1.GetOptions{})
			switch {
			case err != nil:
				cap = unknown("pvc_read_failed", "无法读取参与磁盘的实际PVC")
			case dvUID != "" && !ownedByDV(pvc, dvUID):
				cap = unknown("data_volume_claim_unverified", "无法核验数据卷与PVC的UID归属关系")
			default:
				cap = i.PersistentVolumeClaim(ctx, pvc)
			}
		} else if cap.Status == "" {
			cap = unknown("pvc_reference_missing", "参与磁盘没有可核验的PVC引用")
		}
		out.Volumes = append(out.Volumes, VolumeCapability{Key: v.Name, Capability: cap})
	}
	return Aggregate(out.Volumes)
}

func ownedByDV(pvc *corev1.PersistentVolumeClaim, uid string) bool {
	for _, o := range pvc.OwnerReferences {
		if o.Kind == "DataVolume" && string(o.UID) == uid {
			return true
		}
	}
	return false
}

// PersistentVolumeClaim assesses one fresh caller-observed PVC. The caller must
// validate authoritative scope/expected UID; this function does not choose a claim.
func (i *Inspector) PersistentVolumeClaim(ctx context.Context, pvc *corev1.PersistentVolumeClaim) Capability {
	if i == nil || i.core == nil || i.dynamic == nil {
		return unknown("capability_client_unavailable", "存储能力查询客户端不可用")
	}
	if pvc == nil || pvc.DeletionTimestamp != nil || pvc.Status.Phase != corev1.ClaimBound || pvc.UID == "" {
		return unknown("pvc_not_ready", "参与磁盘PVC尚未绑定或正在删除")
	}
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName == "" {
		return unknown("storage_class_missing", "实际PVC尚无可核验存储类，不能用当前默认类替代")
	}
	cap := i.StorageClass(ctx, *pvc.Spec.StorageClassName)
	if !cap.Supported {
		return cap
	}
	pv, e := i.core.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
	switch {
	case e != nil:
		return unknown("pv_read_failed", "无法核验参与盘的实际PV")
	case pv.DeletionTimestamp != nil || pv.Spec.ClaimRef == nil || pv.Spec.ClaimRef.UID != pvc.UID || pv.Spec.ClaimRef.Name != pvc.Name || pv.Spec.ClaimRef.Namespace != pvc.Namespace:
		return unknown("pv_claim_mismatch", "PV与PVC的实际UID归属不一致")
	case pv.Spec.CSI == nil:
		return decision("unsupported", "pv_not_csi", "实际PV不使用可核验的CSI快照驱动")
	case pv.Spec.CSI.Driver != cap.Driver:
		return unknown("pv_driver_mismatch", "实际PV驱动与当前存储类不一致")
	case pv.Spec.CSI.VolumeHandle == "":
		return unknown("pv_handle_missing", "实际CSI卷身份不可用")
	}
	return cap
}

// Aggregate preserves all participant results and never hides an unknown disk.
func Aggregate(volumes []VolumeCapability) SnapshotSet {
	out := SnapshotSet{Capability: decision("supported", "ready", "所有参与磁盘均具备快照配置与就绪证据"), Volumes: append([]VolumeCapability{}, volumes...)}
	if len(volumes) == 0 {
		out.Capability = unknown("volume_set_empty", "没有可核验的快照参与磁盘")
		return out
	}
	seen := map[string]bool{}
	for n, v := range out.Volumes {
		if v.Key == "" || seen[v.Key] {
			out.Volumes[n].Capability = unknown("volume_set_invalid", "参与磁盘身份重复或缺失")
			v = out.Volumes[n]
		}
		seen[v.Key] = true
		if v.Status == "unsupported" {
			out.Capability = decision("unsupported", "participating_volume_unsupported", "至少一块参与磁盘不支持快照，请查看逐盘原因")
		} else if (!v.Supported || v.Status != "supported") && out.Status != "unsupported" {
			out.Capability = unknown("participating_volume_unverified", "至少一块参与磁盘无法核验，请查看逐盘原因")
		}
	}
	return out
}

// ReadOnlyCDROM checks the actual device rather than the volume's readOnly flag.
func ReadOnlyCDROM(vm *v1.VirtualMachine, key string) bool {
	if vm == nil || vm.Spec.Template == nil {
		return false
	}
	for _, d := range vm.Spec.Template.Spec.Domain.Devices.Disks {
		if d.Name == key && d.CDRom != nil {
			return d.CDRom.ReadOnly == nil || *d.CDRom.ReadOnly
		}
	}
	return false
}

// ReadOnlyInstallationMedia recognizes immutable container-backed CD-ROM media.
// A regular read-only data disk is not installation media.
func ReadOnlyInstallationMedia(vm *v1.VirtualMachine, v v1.Volume) bool {
	return v.ContainerDisk != nil && ReadOnlyCDROM(vm, v.Name)
}

// AuxiliaryVolume enumerates known non-user-disk media; unknown sources are not auxiliary.
func AuxiliaryVolume(v v1.Volume) bool {
	return v.ConfigMap != nil || v.Secret != nil || v.CloudInitNoCloud != nil || v.CloudInitConfigDrive != nil || v.DownwardAPI != nil || v.ServiceAccount != nil || v.DownwardMetrics != nil || v.Sysprep != nil
}
