package volume

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

type vmDiskImportConfig struct {
	VolumeName    string   `json:"volume_name"`
	DiskKey       string   `json:"disk_key"`
	DiskName      string   `json:"disk_name"`
	ImageURL      string   `json:"image_url"`
	SourceURI     string   `json:"source_uri"`
	Format        string   `json:"format"`
	Checksum      string   `json:"checksum"`
	SourceType    string   `json:"source_type"`
	CertConfigMap string   `json:"-"`
	ExtraHeaders  []string `json:"-"`
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
		if cfg.SourceType == "" {
			cfg.SourceType = inferVMDiskImportSourceType(cfg.ImageURL, cfg.SourceURI)
		}
		normalized[volumeName] = cfg
	}
	return normalized
}

func inferVMDiskImportSourceType(imageURL, sourceURI string) string {
	for _, candidate := range []string{imageURL, sourceURI} {
		value := strings.ToLower(strings.TrimSpace(candidate))
		switch {
		case strings.HasPrefix(value, "docker://"):
			return "registry"
		case strings.HasPrefix(value, "http://"), strings.HasPrefix(value, "https://"):
			return "http"
		}
	}
	return ""
}

func normalizeVMRegistryImportURL(imageURL string) string {
	url := strings.TrimSpace(imageURL)
	if url == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(url), "docker://") {
		return url
	}
	return "docker://" + url
}

func buildVMVolumeSource(claim *corev1.PersistentVolumeClaim, labels, annotations map[string]string, volumePath string,
	importConfig *vmDiskImportConfig) (kubevirtv1.Volume, *kubevirtv1.DataVolumeTemplateSpec, bool) {
	serviceID := labels["service_id"]
	if importConfig != nil {
		template := buildVMDiskImportDataVolumeTemplate(claim, labels, annotations, *importConfig)
		mode := importConfig.SourceType
		if mode == "" {
			mode = "http-import"
		}
		logrus.Infof(
			"vm volume source resolved: service_id=%s claim=%s volume_name=%s path=%s mode=%s image_url=%s format=%s",
			serviceID,
			claim.Name,
			annotations["volume_name"],
			volumePath,
			mode,
			importConfig.ImageURL,
			importConfig.Format,
		)
		return kubevirtv1.Volume{
			Name: claim.Name,
			VolumeSource: kubevirtv1.VolumeSource{
				DataVolume: &kubevirtv1.DataVolumeSource{
					Name: claim.Name,
				},
			},
		}, &template, false
	}
	if shouldUseVMBlankDataVolume(volumePath) {
		template := buildVMBlankDataVolumeTemplate(claim, labels, annotations)
		logrus.Infof(
			"vm volume source resolved: service_id=%s claim=%s volume_name=%s path=%s mode=blank-datavolume",
			serviceID,
			claim.Name,
			annotations["volume_name"],
			volumePath,
		)
		return kubevirtv1.Volume{
			Name: claim.Name,
			VolumeSource: kubevirtv1.VolumeSource{
				DataVolume: &kubevirtv1.DataVolumeSource{
					Name: claim.Name,
				},
			},
		}, &template, false
	}
	logrus.Infof(
		"vm volume source resolved: service_id=%s claim=%s volume_name=%s path=%s mode=manual-pvc",
		serviceID,
		claim.Name,
		annotations["volume_name"],
		volumePath,
	)
	return kubevirtv1.Volume{
		Name: claim.Name,
		VolumeSource: kubevirtv1.VolumeSource{
			PersistentVolumeClaim: &kubevirtv1.PersistentVolumeClaimVolumeSource{
				PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: claim.Name,
				},
				Hotpluggable: false,
			},
		},
	}, nil, true
}

func shouldUseVMBlankDataVolume(volumePath string) bool {
	switch vmVolumeDeviceType(volumePath) {
	case "disk", "lun":
		return true
	default:
		return false
	}
}

func vmVolumeDeviceType(volumePath string) string {
	switch {
	case volumePath == "/disk" || strings.HasPrefix(volumePath, "/disk-"):
		return "disk"
	case volumePath == "/lun" || strings.HasPrefix(volumePath, "/lun-"):
		return "lun"
	case volumePath == "/cdrom" || strings.HasPrefix(volumePath, "/cdrom-"):
		return "cdrom"
	default:
		return ""
	}
}

func buildVMBlankDataVolumeTemplate(claim *corev1.PersistentVolumeClaim, labels, annotations map[string]string) kubevirtv1.DataVolumeTemplateSpec {
	return kubevirtv1.DataVolumeTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:        claim.Name,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				Blank: &cdiv1.DataVolumeBlankImage{},
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

func buildVMDiskImportDataVolumeTemplate(claim *corev1.PersistentVolumeClaim, labels, annotations map[string]string, cfg vmDiskImportConfig) kubevirtv1.DataVolumeTemplateSpec {
	logrus.Infof(
		"vm disk import template build: claim=%s service_id=%s volume_name=%s source_type=%s image_url=%s cert_configmap=%s extra_headers=%d",
		claim.Name,
		labels["service_id"],
		annotations["volume_name"],
		cfg.SourceType,
		cfg.ImageURL,
		cfg.CertConfigMap,
		len(cfg.ExtraHeaders),
	)
	source := &cdiv1.DataVolumeSource{
		HTTP: &cdiv1.DataVolumeSourceHTTP{
			URL:           cfg.ImageURL,
			CertConfigMap: cfg.CertConfigMap,
			ExtraHeaders:  cfg.ExtraHeaders,
		},
	}
	if cfg.SourceType == "registry" {
		url := normalizeVMRegistryImportURL(cfg.ImageURL)
		pullMethod := cdiv1.RegistryPullPod
		source = &cdiv1.DataVolumeSource{
			Registry: &cdiv1.DataVolumeSourceRegistry{
				URL:        &url,
				PullMethod: &pullMethod,
			},
		}
	}
	return kubevirtv1.DataVolumeTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:        claim.Name,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: source,
			Storage: &cdiv1.StorageSpec{
				AccessModes:      claim.Spec.AccessModes,
				Resources:        claim.Spec.Resources,
				StorageClassName: claim.Spec.StorageClassName,
				VolumeMode:       claim.Spec.VolumeMode,
			},
		},
	}
}
