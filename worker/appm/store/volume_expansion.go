package store

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/retry"
)

// reconcileStatefulVolumeClaim applies the stored capacity to claims created from
// an older, immutable StatefulSet template, including claims added by HPA scaling.
func (a *appRuntimeStore) reconcileStatefulVolumeClaim(ctx context.Context, claim *corev1.PersistentVolumeClaim) error {
	if a.volumeExpansionClient == nil || a.dbmanager == nil || claim.DeletionTimestamp != nil ||
		claim.Labels["creator"] != "Rainbond" || claim.Labels["creater_id"] == "" ||
		claim.Labels["service_id"] == "" || claim.Annotations["volume_name"] == "" {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	service, err := a.dbmanager.TenantServiceDao().GetServiceByID(claim.Labels["service_id"])
	if gorm.IsRecordNotFoundError(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if service == nil || !service.IsState() || service.IsVM() || service.TenantID == "" {
		return nil
	}
	volume, err := a.dbmanager.TenantServiceVolumeDao().GetVolumeByServiceIDAndName(service.ServiceID, claim.Annotations["volume_name"])
	if gorm.IsRecordNotFoundError(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if volume == nil || volume.VolumeCapacity <= 0 || !managedVolumeClaimMatches(claim, service, volume) {
		return nil
	}
	switch volume.VolumeType {
	case dbmodel.ConfigFileVolumeType.String(), dbmodel.MemoryFSVolumeType.String(), dbmodel.VMVolumeType.String():
		return nil
	case dbmodel.ShareFileVolumeType.String():
		if os.Getenv("ENABLE_SUBPATH") == "true" {
			return nil
		}
	}
	target := resource.MustParse(fmt.Sprintf("%dGi", volume.VolumeCapacity))
	if !volumeClaimNeedsExpansion(claim, target) {
		return nil
	}
	tenant, err := a.dbmanager.TenantDao().GetTenantByUUID(service.TenantID)
	if gorm.IsRecordNotFoundError(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if tenant == nil || tenant.Namespace == "" || tenant.Namespace != claim.Namespace {
		return nil
	}
	selector := labels.Set{"service_id": service.ServiceID, "creator": "Rainbond"}.AsSelector().String()
	statefulSets, err := a.volumeExpansionClient.AppsV1().StatefulSets(claim.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return err
	}
	managed := false
	for i := range statefulSets.Items {
		if statefulVolumeClaimMatches(claim, &statefulSets.Items[i], service, volume) {
			managed = true
			break
		}
	}
	if !managed || claim.Spec.StorageClassName == nil || *claim.Spec.StorageClassName == "" {
		return nil
	}
	className := *claim.Spec.StorageClassName
	storageClass, err := a.volumeExpansionClient.StorageV1().StorageClasses().Get(ctx, className, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	provisioner := strings.ToLower(storageClass.Provisioner)
	if storageClass.AllowVolumeExpansion == nil || !*storageClass.AllowVolumeExpansion ||
		strings.Contains(provisioner, "nfs-subdir-external-provisioner") || strings.Contains(provisioner, "nfs-client-provisioner") {
		return nil
	}
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := a.volumeExpansionClient.CoreV1().PersistentVolumeClaims(claim.Namespace).Get(ctx, claim.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if current.UID != claim.UID || current.DeletionTimestamp != nil ||
			current.Labels["creater_id"] != claim.Labels["creater_id"] || !managedVolumeClaimMatches(current, service, volume) ||
			current.Spec.StorageClassName == nil || *current.Spec.StorageClassName != className || !volumeClaimNeedsExpansion(current, target) {
			return nil
		}
		updated := current.DeepCopy()
		if updated.Spec.Resources.Requests == nil {
			updated.Spec.Resources.Requests = corev1.ResourceList{}
		}
		updated.Spec.Resources.Requests[corev1.ResourceStorage] = target.DeepCopy()
		_, err = a.volumeExpansionClient.CoreV1().PersistentVolumeClaims(current.Namespace).Update(ctx, updated, metav1.UpdateOptions{})
		return err
	})
}

func managedVolumeClaimMatches(claim *corev1.PersistentVolumeClaim, service *dbmodel.TenantServices, volume *dbmodel.TenantServiceVolume) bool {
	volumeLabel := claim.Labels["volume_name"]
	return claim.Labels["creator"] == "Rainbond" && claim.Labels["service_id"] == service.ServiceID &&
		claim.Labels["tenant_id"] == service.TenantID && volume.ServiceID == service.ServiceID &&
		claim.Annotations["volume_name"] == volume.VolumeName && volume.ID > 0 &&
		(volumeLabel == volume.VolumeName || volumeLabel == fmt.Sprintf("manual%d", volume.ID))
}

func statefulVolumeClaimMatches(claim *corev1.PersistentVolumeClaim, statefulSet *appsv1.StatefulSet,
	service *dbmodel.TenantServices, volume *dbmodel.TenantServiceVolume,
) bool {
	if statefulSet.DeletionTimestamp != nil || statefulSet.Namespace != claim.Namespace ||
		statefulSet.Labels["creator"] != "Rainbond" || statefulSet.Labels["service_id"] != service.ServiceID ||
		statefulSet.Labels["tenant_id"] != service.TenantID {
		return false
	}
	templateName := fmt.Sprintf("manual%d", volume.ID)
	prefix := templateName + "-" + statefulSet.Name + "-"
	if !strings.HasPrefix(claim.Name, prefix) {
		return false
	}
	ordinal := strings.TrimPrefix(claim.Name, prefix)
	value, err := strconv.ParseUint(ordinal, 10, 32)
	if err != nil || strconv.FormatUint(value, 10) != ordinal {
		return false
	}
	for i := range statefulSet.Spec.VolumeClaimTemplates {
		template := &statefulSet.Spec.VolumeClaimTemplates[i]
		// Upgrades can replace the workload generation, while the immutable
		// template and its future PVCs retain their original generation.
		if template.Name == templateName && template.Labels["creater_id"] == claim.Labels["creater_id"] &&
			managedVolumeClaimMatches(template, service, volume) {
			return true
		}
	}
	return false
}

func volumeClaimNeedsExpansion(claim *corev1.PersistentVolumeClaim, target resource.Quantity) bool {
	requested := claim.Spec.Resources.Requests[corev1.ResourceStorage]
	actual := claim.Status.Capacity[corev1.ResourceStorage]
	return target.Cmp(requested) > 0 && target.Cmp(actual) >= 0
}
