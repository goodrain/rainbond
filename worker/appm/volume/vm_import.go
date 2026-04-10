package volume

import (
	"encoding/json"
	"fmt"

	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

type vmDiskImportConfig struct {
	VolumeName string `json:"volume_name"`
	DiskKey    string `json:"disk_key"`
	DiskName   string `json:"disk_name"`
	ImageURL   string `json:"image_url"`
	SourceURI  string `json:"source_uri"`
	Format     string `json:"format"`
	Checksum   string `json:"checksum"`
}

func loadVMDiskImportConfigs(serviceID string, dbmanager db.Manager) (map[string]vmDiskImportConfig, error) {
	attr, err := dbmanager.ComponentK8sAttributeDao().GetByComponentIDAndName(serviceID, dbmodel.K8sAttributeNameVMDiskImports)
	if err != nil {
		return nil, err
	}
	if attr == nil || attr.AttributeValue == "" {
		return map[string]vmDiskImportConfig{}, nil
	}
	return parseVMDiskImportConfigs(attr.AttributeValue)
}

func parseVMDiskImportConfigs(raw string) (map[string]vmDiskImportConfig, error) {
	if raw == "" {
		return map[string]vmDiskImportConfig{}, nil
	}

	var keyed map[string]vmDiskImportConfig
	if err := json.Unmarshal([]byte(raw), &keyed); err == nil {
		return normalizeVMDiskImportConfigs(keyed), nil
	}

	var listed []vmDiskImportConfig
	if err := json.Unmarshal([]byte(raw), &listed); err == nil {
		keyed = make(map[string]vmDiskImportConfig, len(listed))
		for _, item := range listed {
			key := item.VolumeName
			if key == "" {
				key = item.DiskKey
			}
			if key == "" || item.ImageURL == "" {
				continue
			}
			keyed[key] = item
		}
		return normalizeVMDiskImportConfigs(keyed), nil
	}

	return nil, fmt.Errorf("invalid vm disk imports json")
}

func normalizeVMDiskImportConfigs(configs map[string]vmDiskImportConfig) map[string]vmDiskImportConfig {
	normalized := make(map[string]vmDiskImportConfig, len(configs))
	for key, cfg := range configs {
		volumeName := cfg.VolumeName
		if volumeName == "" {
			volumeName = key
		}
		if volumeName == "" || cfg.ImageURL == "" {
			continue
		}
		cfg.VolumeName = volumeName
		if cfg.DiskKey == "" {
			cfg.DiskKey = volumeName
		}
		if cfg.DiskName == "" {
			cfg.DiskName = volumeName
		}
		normalized[volumeName] = cfg
	}
	return normalized
}

func buildVMDiskImportDataVolumeTemplate(claim *corev1.PersistentVolumeClaim, labels, annotations map[string]string, cfg vmDiskImportConfig) kubevirtv1.DataVolumeTemplateSpec {
	return kubevirtv1.DataVolumeTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:        claim.Name,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				HTTP: &cdiv1.DataVolumeSourceHTTP{
					URL: cfg.ImageURL,
				},
			},
			Storage: &cdiv1.StorageSpec{
				AccessModes:      claim.Spec.AccessModes,
				Resources:        claim.Spec.Resources,
				StorageClassName: claim.Spec.StorageClassName,
				VolumeMode:       claim.Spec.VolumeMode,
			},
		},
	}
}
