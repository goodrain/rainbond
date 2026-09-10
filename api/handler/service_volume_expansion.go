package handler

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/goodrain/rainbond/api/util/bcode"
	dbmodel "github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/retry"
)

const (
	volumeExpansionUnsupported             = "unsupported"
	volumeExpansionUnbound                 = "unbound"
	volumeExpansionReady                   = "ready"
	volumeExpansionResizing                = "resizing"
	volumeExpansionFileSystemResizePending = "filesystem_resize_pending"
	volumeExpansionFailed                  = "failed"
)

const gibibyte = int64(1024 * 1024 * 1024)

type volumeExpansionRuntime struct {
	AllowExpansion    bool
	ActualCapacity    int64
	RequestedCapacity int64
	Status            string
	Message           string
	PVCCount          int
}

func unsupportedVolumeExpansionReason(service *dbmodel.TenantServices, volume *dbmodel.TenantServiceVolume) string {
	if service != nil && service.ExtendMethod == "vm" {
		return "virtual machine guest disk expansion is not supported"
	}
	if volume == nil {
		return "volume does not exist"
	}
	switch volume.VolumeType {
	case dbmodel.ConfigFileVolumeType.String():
		return "configuration file volumes do not use PersistentVolumeClaims"
	case dbmodel.MemoryFSVolumeType.String():
		return "memory filesystem volumes do not use PersistentVolumeClaims"
	case dbmodel.ShareFileVolumeType.String():
		if os.Getenv("ENABLE_SUBPATH") == "true" {
			return "subpath volumes share one PersistentVolumeClaim and cannot be expanded independently"
		}
	}
	return ""
}

func matchesServiceVolumeClaim(pvc *corev1.PersistentVolumeClaim, volume *dbmodel.TenantServiceVolume) bool {
	if pvc == nil || volume == nil {
		return false
	}
	if pvc.Annotations["volume_name"] == volume.VolumeName || pvc.Labels["volume_name"] == volume.VolumeName {
		return true
	}
	manualName := fmt.Sprintf("manual%d", volume.ID)
	return pvc.Labels["volume_name"] == manualName || pvc.Name == manualName || strings.HasPrefix(pvc.Name, manualName+"-")
}

func capacityInGi(quantity resource.Quantity) int64 {
	value := quantity.Value()
	if value <= 0 {
		return 0
	}
	return (value + gibibyte - 1) / gibibyte
}

func expansionCondition(pvc *corev1.PersistentVolumeClaim) (status, message string) {
	for _, condition := range pvc.Status.Conditions {
		if condition.Status != corev1.ConditionTrue {
			continue
		}
		switch condition.Type {
		case corev1.PersistentVolumeClaimControllerResizeError, corev1.PersistentVolumeClaimNodeResizeError:
			return volumeExpansionFailed, condition.Message
		}
	}
	for _, condition := range pvc.Status.Conditions {
		if condition.Status == corev1.ConditionTrue && condition.Type == corev1.PersistentVolumeClaimFileSystemResizePending {
			return volumeExpansionFileSystemResizePending, condition.Message
		}
	}
	for _, condition := range pvc.Status.Conditions {
		if condition.Status == corev1.ConditionTrue && condition.Type == corev1.PersistentVolumeClaimResizing {
			return volumeExpansionResizing, condition.Message
		}
	}
	return "", ""
}

func (s *ServiceAction) inspectVolumeExpansion(ctx context.Context, service *dbmodel.TenantServices,
	volume *dbmodel.TenantServiceVolume,
) (volumeExpansionRuntime, []*corev1.PersistentVolumeClaim, error) {
	runtimeStatus := volumeExpansionRuntime{Status: volumeExpansionUnsupported}
	if reason := unsupportedVolumeExpansionReason(service, volume); reason != "" {
		runtimeStatus.Message = reason
		return runtimeStatus, nil, nil
	}
	if s.kubeClient == nil {
		runtimeStatus.Message = "Kubernetes client is unavailable"
		return runtimeStatus, nil, nil
	}
	namespace, err := s.resolveVolumeExpansionNamespace(service)
	if err != nil {
		return runtimeStatus, nil, err
	}
	if namespace == "" {
		runtimeStatus.Message = "component namespace is unavailable"
		return runtimeStatus, nil, nil
	}

	selector := labels.Set{"service_id": service.ServiceID}.AsSelector().String()
	list, err := s.kubeClient.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return runtimeStatus, nil, fmt.Errorf("list component PersistentVolumeClaims: %w", err)
	}
	claims := make([]*corev1.PersistentVolumeClaim, 0)
	for i := range list.Items {
		if matchesServiceVolumeClaim(&list.Items[i], volume) {
			claims = append(claims, &list.Items[i])
		}
	}
	if len(claims) == 0 {
		runtimeStatus.AllowExpansion = true
		runtimeStatus.Status = volumeExpansionUnbound
		return runtimeStatus, claims, nil
	}

	runtimeStatus.PVCCount = len(claims)
	runtimeStatus.ActualCapacity = math.MaxInt64
	classCache := make(map[string]bool)
	conditionStatus := ""
	conditionMessage := ""
	for _, pvc := range claims {
		requested := capacityInGi(pvc.Spec.Resources.Requests[corev1.ResourceStorage])
		actual := capacityInGi(pvc.Status.Capacity[corev1.ResourceStorage])
		if requested > runtimeStatus.RequestedCapacity {
			runtimeStatus.RequestedCapacity = requested
		}
		if actual < runtimeStatus.ActualCapacity {
			runtimeStatus.ActualCapacity = actual
		}

		if pvc.Spec.StorageClassName == nil || strings.TrimSpace(*pvc.Spec.StorageClassName) == "" {
			runtimeStatus.Message = fmt.Sprintf("PersistentVolumeClaim %s has no StorageClass", pvc.Name)
			runtimeStatus.ActualCapacity = normalizedActualCapacity(runtimeStatus.ActualCapacity)
			return runtimeStatus, claims, nil
		}
		className := *pvc.Spec.StorageClassName
		allowed, ok := classCache[className]
		if !ok {
			storageClass, getErr := s.kubeClient.StorageV1().StorageClasses().Get(ctx, className, metav1.GetOptions{})
			if getErr != nil {
				if apierrors.IsNotFound(getErr) {
					runtimeStatus.Message = fmt.Sprintf("StorageClass %s does not exist", className)
					runtimeStatus.ActualCapacity = normalizedActualCapacity(runtimeStatus.ActualCapacity)
					return runtimeStatus, claims, nil
				}
				return runtimeStatus, claims, fmt.Errorf("get StorageClass %s: %w", className, getErr)
			}
			allowed = storageClass.AllowVolumeExpansion != nil && *storageClass.AllowVolumeExpansion
			classCache[className] = allowed
		}
		if !allowed {
			runtimeStatus.Message = fmt.Sprintf("StorageClass %s does not allow volume expansion", className)
			runtimeStatus.ActualCapacity = normalizedActualCapacity(runtimeStatus.ActualCapacity)
			return runtimeStatus, claims, nil
		}

		status, message := expansionCondition(pvc)
		if status == volumeExpansionFailed || conditionStatus == "" ||
			(status == volumeExpansionFileSystemResizePending && conditionStatus == volumeExpansionResizing) {
			conditionStatus = status
			conditionMessage = message
		}
	}

	runtimeStatus.AllowExpansion = true
	runtimeStatus.ActualCapacity = normalizedActualCapacity(runtimeStatus.ActualCapacity)
	runtimeStatus.Status = volumeExpansionReady
	if conditionStatus != "" {
		runtimeStatus.Status = conditionStatus
		runtimeStatus.Message = conditionMessage
	} else if runtimeStatus.ActualCapacity < runtimeStatus.RequestedCapacity {
		runtimeStatus.Status = volumeExpansionResizing
	}
	return runtimeStatus, claims, nil
}

func normalizedActualCapacity(capacity int64) int64 {
	if capacity == math.MaxInt64 {
		return 0
	}
	return capacity
}

func (s *ServiceAction) resolveVolumeExpansionNamespace(service *dbmodel.TenantServices) (string, error) {
	if service == nil {
		return "", nil
	}
	tenantNamespace := ""
	if (service.Namespace == "" || service.Namespace == service.TenantID) && s.resolveTenantNamespaceHook != nil {
		var err error
		tenantNamespace, err = s.resolveTenantNamespaceHook(service.TenantID)
		if err != nil {
			return "", fmt.Errorf("resolve tenant namespace: %w", err)
		}
	}
	return resolvePodMetricsNamespace(service.Namespace, service.TenantID, tenantNamespace), nil
}

func (s *ServiceAction) expandVolumeClaims(ctx context.Context, service *dbmodel.TenantServices,
	volume *dbmodel.TenantServiceVolume, targetGi int64,
) error {
	if targetGi <= 0 {
		return bcode.NewBadRequest("volume capacity must be a positive integer")
	}
	if targetGi < volume.VolumeCapacity {
		return bcode.NewBadRequest("volume capacity can only be expanded, not reduced")
	}
	if targetGi == volume.VolumeCapacity {
		return nil
	}
	if reason := unsupportedVolumeExpansionReason(service, volume); reason != "" {
		return bcode.NewBadRequest(reason)
	}
	if s.kubeClient == nil {
		return nil
	}

	runtimeStatus, claims, err := s.inspectVolumeExpansion(ctx, service, volume)
	if err != nil {
		return err
	}
	if runtimeStatus.Status == volumeExpansionUnsupported {
		return bcode.NewBadRequest(runtimeStatus.Message)
	}
	if len(claims) == 0 {
		return nil
	}

	target := resource.MustParse(fmt.Sprintf("%dGi", targetGi))
	for _, pvc := range claims {
		requested := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		actual := pvc.Status.Capacity[corev1.ResourceStorage]
		if target.Cmp(requested) < 0 || target.Cmp(actual) < 0 {
			return bcode.NewBadRequest(fmt.Sprintf(
				"volume capacity can only be expanded; PersistentVolumeClaim %s already requests or provides more than %dGi",
				pvc.Name, targetGi,
			))
		}
	}

	updated := make([]string, 0, len(claims))
	for _, pvc := range claims {
		requested := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		if target.Cmp(requested) == 0 {
			continue
		}
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			current, getErr := s.kubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).
				Get(ctx, pvc.Name, metav1.GetOptions{})
			if getErr != nil {
				return getErr
			}
			currentRequested := current.Spec.Resources.Requests[corev1.ResourceStorage]
			if currentRequested.Cmp(target) > 0 {
				return bcode.NewBadRequest(fmt.Sprintf(
					"PersistentVolumeClaim %s already requests more than %dGi", current.Name, targetGi,
				))
			}
			if currentRequested.Cmp(target) == 0 {
				return nil
			}
			updatedPVC := current.DeepCopy()
			if updatedPVC.Spec.Resources.Requests == nil {
				updatedPVC.Spec.Resources.Requests = corev1.ResourceList{}
			}
			updatedPVC.Spec.Resources.Requests[corev1.ResourceStorage] = target.DeepCopy()
			_, updateErr := s.kubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).
				Update(ctx, updatedPVC, metav1.UpdateOptions{})
			return updateErr
		})
		if err != nil {
			return fmt.Errorf("expand PersistentVolumeClaim %s after updating [%s]: %w", pvc.Name, strings.Join(updated, ", "), err)
		}
		updated = append(updated, pvc.Name)
	}
	return nil
}
