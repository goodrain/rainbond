package handler

import (
	"bytes"
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/goodrain/rainbond/pkg/component/k8s"
)

// StorageClassInfo is the API response struct for a StorageClass
type StorageClassInfo struct {
	Name                 string `json:"name"`
	Provisioner          string `json:"provisioner"`
	IsDefault            bool   `json:"is_default"`
	ReclaimPolicy        string `json:"reclaim_policy"`
	VolumeBindingMode    string `json:"volume_binding_mode"`
	AllowVolumeExpansion bool   `json:"allow_volume_expansion"`
	PVCount              int    `json:"pv_count"`
}

// StorageHandler handles StorageClass and PersistentVolume operations
type StorageHandler struct{}

// ListStorageClasses returns all StorageClasses with PV counts
func (h *StorageHandler) ListStorageClasses() ([]StorageClassInfo, error) {
	client := k8s.Default().Clientset
	scList, err := client.StorageV1().StorageClasses().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	pvList, err := client.CoreV1().PersistentVolumes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	pvCount := make(map[string]int)
	for _, pv := range pvList.Items {
		if pv.Spec.StorageClassName != "" {
			pvCount[pv.Spec.StorageClassName]++
		}
	}
	var result []StorageClassInfo
	for _, sc := range scList.Items {
		isDefault := sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true"
		reclaimPolicy := ""
		if sc.ReclaimPolicy != nil {
			reclaimPolicy = string(*sc.ReclaimPolicy)
		}
		volumeBindingMode := ""
		if sc.VolumeBindingMode != nil {
			volumeBindingMode = string(*sc.VolumeBindingMode)
		}
		allowExpansion := sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion
		result = append(result, StorageClassInfo{
			Name:                 sc.Name,
			Provisioner:          sc.Provisioner,
			IsDefault:            isDefault,
			ReclaimPolicy:        reclaimPolicy,
			VolumeBindingMode:    volumeBindingMode,
			AllowVolumeExpansion: allowExpansion,
			PVCount:              pvCount[sc.Name],
		})
	}
	return result, nil
}

// CreateStorageClass decodes YAML and creates a StorageClass
func (h *StorageHandler) CreateStorageClass(yamlBody []byte) (*storagev1.StorageClass, error) {
	sc := &storagev1.StorageClass{}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlBody), 4096)
	if err := decoder.Decode(sc); err != nil {
		return nil, fmt.Errorf("invalid StorageClass YAML: %v", err)
	}
	return k8s.Default().Clientset.StorageV1().StorageClasses().Create(context.Background(), sc, metav1.CreateOptions{})
}

// DeleteStorageClass deletes a StorageClass by name (no-op if not found)
func (h *StorageHandler) DeleteStorageClass(name string) error {
	err := k8s.Default().Clientset.StorageV1().StorageClasses().Delete(context.Background(), name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

// PVInfo is the flat API response struct for a PersistentVolume
type PVInfo struct {
	Name          string   `json:"name"`
	Capacity      string   `json:"capacity"`
	AccessModes   []string `json:"access_modes"`
	StorageClass  string   `json:"storage_class"`
	Status        string   `json:"status"`
	ReclaimPolicy string   `json:"reclaim_policy"`
	Claim         string   `json:"claim"`
}

// ListPersistentVolumes returns all PersistentVolumes as flat PVInfo structs
func (h *StorageHandler) ListPersistentVolumes() ([]PVInfo, error) {
	list, err := k8s.Default().Clientset.CoreV1().PersistentVolumes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]PVInfo, 0, len(list.Items))
	for _, pv := range list.Items {
		capacity := ""
		if q, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
			capacity = q.String()
		}
		modes := make([]string, 0, len(pv.Spec.AccessModes))
		for _, m := range pv.Spec.AccessModes {
			modes = append(modes, string(m))
		}
		claim := ""
		if pv.Spec.ClaimRef != nil {
			claim = pv.Spec.ClaimRef.Namespace + "/" + pv.Spec.ClaimRef.Name
		}
		reclaimPolicy := ""
		if pv.Spec.PersistentVolumeReclaimPolicy != "" {
			reclaimPolicy = string(pv.Spec.PersistentVolumeReclaimPolicy)
		}
		result = append(result, PVInfo{
			Name:          pv.Name,
			Capacity:      capacity,
			AccessModes:   modes,
			StorageClass:  pv.Spec.StorageClassName,
			Status:        string(pv.Status.Phase),
			ReclaimPolicy: reclaimPolicy,
			Claim:         claim,
		})
	}
	return result, nil
}

// CreatePersistentVolume decodes YAML and creates a PersistentVolume
func (h *StorageHandler) CreatePersistentVolume(yamlBody []byte) (*corev1.PersistentVolume, error) {
	pv := &corev1.PersistentVolume{}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlBody), 4096)
	if err := decoder.Decode(pv); err != nil {
		return nil, fmt.Errorf("invalid PersistentVolume YAML: %v", err)
	}
	return k8s.Default().Clientset.CoreV1().PersistentVolumes().Create(context.Background(), pv, metav1.CreateOptions{})
}

// DeletePersistentVolume deletes a PV by name (no-op if not found)
func (h *StorageHandler) DeletePersistentVolume(name string) error {
	err := k8s.Default().Clientset.CoreV1().PersistentVolumes().Delete(context.Background(), name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

var storageHandler *StorageHandler

// GetStorageHandler returns the singleton StorageHandler
func GetStorageHandler() *StorageHandler {
	if storageHandler == nil {
		storageHandler = &StorageHandler{}
	}
	return storageHandler
}
