package handler

import (
	"context"
	"fmt"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/vmrestore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/api/core/v1"
)

const vmExpansionGuestMessage = "PVC容量状态；来宾块设备容量、分区和文件系统扩容仍需确认"

func (s *ServiceAction) vmDiskExpansionCapability(ctx context.Context) (string, error) {
	if s.kubevirtClient == nil {
		return "KubeVirt capability client is unavailable", nil
	}
	list, err := s.kubevirtClient.KubeVirt("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("read KubeVirt disk expansion capability: %w", err)
	}
	if len(list.Items) != 1 {
		return "KubeVirt configuration could not be uniquely identified", nil
	}
	kv := list.Items[0]
	if kv.Status.Phase != v1.KubeVirtPhaseDeployed {
		return "KubeVirt is not fully deployed", nil
	}
	// v1.8.1 still registers ExpandDisks as opt-in. A version string cannot
	// establish that virt-launcher will resize the image and notify the guest.
	if config := kv.Spec.Configuration.DeveloperConfiguration; config != nil {
		for _, gate := range config.FeatureGates {
			if gate == "ExpandDisks" {
				return "", nil
			}
		}
	}
	return "KubeVirt ExpandDisks feature gate is not enabled", nil
}

const vmVolumeObservationBusy = "vm-disk-operation-in-progress"

func (s *ServiceAction) inspectVMExpansionClaims(ctx context.Context, service *dbmodel.TenantServices, volume *dbmodel.TenantServiceVolume, namespace string) ([]*corev1.PersistentVolumeClaim, string, error) {
	if reason, err := s.vmDiskExpansionCapability(ctx); reason != "" || err != nil {
		return nil, reason, err
	}
	consumer, err := s.vmVolumes(service)
	if err != nil {
		return nil, "", err
	}
	if consumer.scope.Namespace != namespace {
		return nil, "", vmrestore.ErrConflict
	}
	resolved, err := consumer.resolver.ResolveVolume(ctx, consumer.scope, volume.ID)
	if err != nil {
		return nil, "", err
	}
	if resolved.Role != "data" || resolved.ReadOnly {
		return nil, "只支持可写虚拟机数据盘扩容", nil
	}
	if resolved.ActivePVC == nil {
		return nil, "数据盘尚未准备完成", nil
	}
	if _, err := s.checkVMDataDevice(ctx, resolved); err != nil {
		return nil, "", err
	}
	pvc, err := s.kubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, resolved.ActivePVC.Name, metav1.GetOptions{})
	if err != nil {
		return nil, "", err
	}
	if vmrestore.CheckLivePVC(*resolved.ActivePVC, pvc.Namespace, pvc.Name, string(pvc.UID)) != nil {
		return nil, "", vmrestore.ErrConflict
	}
	refs, err := consumer.resolver.VolumeReferences(ctx, consumer.scope, volume.ID)
	if err != nil {
		return nil, "", err
	}
	if !refs.CanExpand {
		for _, reason := range refs.BlockingReasons {
			if reason == "component_operation_in_progress" {
				return []*corev1.PersistentVolumeClaim{pvc}, vmVolumeObservationBusy, nil
			}
		}
		return []*corev1.PersistentVolumeClaim{pvc}, "磁盘当前引用不允许扩容", nil
	}
	return []*corev1.PersistentVolumeClaim{pvc}, "", nil
}
