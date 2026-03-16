package handler

import (
	"context"

	"github.com/goodrain/rainbond/pkg/component/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// StorageHandler defines handler methods for storage resource operations
type StorageHandler interface {
	GetStorageOverview() (*StorageOverview, error)
	ListStorageClasses() ([]StorageClassInfo, error)
	ListPersistentVolumes() ([]corev1.PersistentVolume, error)
}

// NewStorageHandler creates a new StorageHandler
func NewStorageHandler() StorageHandler {
	k8sComp := k8s.Default()
	var client kubernetes.Interface
	if k8sComp.TestClientset != nil {
		client = k8sComp.TestClientset
	} else {
		client = k8sComp.Clientset
	}
	return &storageAction{
		k8sClient: client,
	}
}

// storageAction implements StorageHandler
type storageAction struct {
	k8sClient kubernetes.Interface
}

// StorageOverview represents aggregated storage information
type StorageOverview struct {
	TotalPVs          int                  `json:"total_pvs"`
	AvailablePVs      int                  `json:"available_pvs"`
	BoundPVs          int                  `json:"bound_pvs"`
	TotalCapacity     string               `json:"total_capacity"`
	StorageClasses    []StorageClassInfo   `json:"storage_classes"`
}

// StorageClassInfo represents storage class information with PV count
type StorageClassInfo struct {
	Name              string `json:"name"`
	Provisioner       string `json:"provisioner"`
	ReclaimPolicy     string `json:"reclaim_policy"`
	VolumeBindingMode string `json:"volume_binding_mode"`
	PVCount           int    `json:"pv_count"`
	IsDefault         bool   `json:"is_default"`
}

// GetStorageOverview aggregates PV and StorageClass information
func (s *storageAction) GetStorageOverview() (*StorageOverview, error) {
	ctx := context.Background()

	// List all PersistentVolumes
	pvList, err := s.k8sClient.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// List all StorageClasses
	scList, err := s.k8sClient.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Count PV statuses
	availableCount := 0
	boundCount := 0
	for _, pv := range pvList.Items {
		switch pv.Status.Phase {
		case corev1.VolumeAvailable:
			availableCount++
		case corev1.VolumeBound:
			boundCount++
		}
	}

	// Build StorageClass info with PV counts
	scInfoList := make([]StorageClassInfo, 0, len(scList.Items))
	for _, sc := range scList.Items {
		// Count PVs using this StorageClass
		pvCount := 0
		for _, pv := range pvList.Items {
			if pv.Spec.StorageClassName == sc.Name {
				pvCount++
			}
		}

		// Check if this is the default StorageClass
		isDefault := false
		if sc.Annotations != nil {
			if val, ok := sc.Annotations["storageclass.kubernetes.io/is-default-class"]; ok && val == "true" {
				isDefault = true
			}
		}

		reclaimPolicy := ""
		if sc.ReclaimPolicy != nil {
			reclaimPolicy = string(*sc.ReclaimPolicy)
		}

		volumeBindingMode := ""
		if sc.VolumeBindingMode != nil {
			volumeBindingMode = string(*sc.VolumeBindingMode)
		}

		scInfoList = append(scInfoList, StorageClassInfo{
			Name:              sc.Name,
			Provisioner:       sc.Provisioner,
			ReclaimPolicy:     reclaimPolicy,
			VolumeBindingMode: volumeBindingMode,
			PVCount:           pvCount,
			IsDefault:         isDefault,
		})
	}

	return &StorageOverview{
		TotalPVs:       len(pvList.Items),
		AvailablePVs:   availableCount,
		BoundPVs:       boundCount,
		TotalCapacity:  "", // Can be calculated if needed
		StorageClasses: scInfoList,
	}, nil
}

// ListStorageClasses lists all StorageClasses in the cluster
func (s *storageAction) ListStorageClasses() ([]StorageClassInfo, error) {
	ctx := context.Background()

	scList, err := s.k8sClient.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Get PV list to count PVs per StorageClass
	pvList, err := s.k8sClient.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	scInfoList := make([]StorageClassInfo, 0, len(scList.Items))
	for _, sc := range scList.Items {
		// Count PVs using this StorageClass
		pvCount := 0
		for _, pv := range pvList.Items {
			if pv.Spec.StorageClassName == sc.Name {
				pvCount++
			}
		}

		// Check if this is the default StorageClass
		isDefault := false
		if sc.Annotations != nil {
			if val, ok := sc.Annotations["storageclass.kubernetes.io/is-default-class"]; ok && val == "true" {
				isDefault = true
			}
		}

		reclaimPolicy := ""
		if sc.ReclaimPolicy != nil {
			reclaimPolicy = string(*sc.ReclaimPolicy)
		}

		volumeBindingMode := ""
		if sc.VolumeBindingMode != nil {
			volumeBindingMode = string(*sc.VolumeBindingMode)
		}

		scInfoList = append(scInfoList, StorageClassInfo{
			Name:              sc.Name,
			Provisioner:       sc.Provisioner,
			ReclaimPolicy:     reclaimPolicy,
			VolumeBindingMode: volumeBindingMode,
			PVCount:           pvCount,
			IsDefault:         isDefault,
		})
	}

	return scInfoList, nil
}

// ListPersistentVolumes lists all PersistentVolumes in the cluster
func (s *storageAction) ListPersistentVolumes() ([]corev1.PersistentVolume, error) {
	ctx := context.Background()

	pvList, err := s.k8sClient.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return pvList.Items, nil
}
