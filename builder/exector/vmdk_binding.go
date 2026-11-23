package exector

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/distribution/reference"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
)

type vmdkRootConsumer struct {
	VolumeName string `json:"volume_name"`
	DiskKey    string `json:"disk_key"`
	SourceType string `json:"source_type"`
	Format     string `json:"format"`
	ImageURL   string `json:"image_url"`
}

func invalidVMDKConsumer() error {
	return fmt.Errorf("vm_vmdk_runtime_binding_invalid: 当前根盘未绑定本次转换的QCOW2镜像，请通过VMDK入口重新创建，不能直接导入原VMDK下载地址")
}

func validateVMDKRuntimeConsumer(raw, image, local string) error {
	if len(raw) == 0 || len(raw) > 1<<20 {
		return invalidVMDKConsumer()
	}
	var keyed map[string]vmdkRootConsumer
	var root *vmdkRootConsumer
	if err := json.Unmarshal([]byte(raw), &keyed); err == nil {
		for key, item := range keyed {
			if item.VolumeName == "" {
				item.VolumeName = key
			}
			if item.VolumeName == "disk" {
				if root != nil {
					return invalidVMDKConsumer()
				}
				copy := item
				root = &copy
			}
		}
	} else {
		var list []vmdkRootConsumer
		if err = json.Unmarshal([]byte(raw), &list); err != nil {
			return invalidVMDKConsumer()
		}
		for _, item := range list {
			name := item.VolumeName
			if name == "" {
				name = item.DiskKey
			}
			if name == "disk" {
				if root != nil {
					return invalidVMDKConsumer()
				}
				copy := item
				root = &copy
			}
		}
	}
	if root == nil || strings.ToLower(strings.TrimSpace(root.SourceType)) != "registry" || strings.ToLower(strings.TrimSpace(root.Format)) != "qcow2" {
		return invalidVMDKConsumer()
	}
	strip := func(value string) string {
		value = strings.TrimSpace(value)
		value = strings.TrimPrefix(value, "docker://")
		if strings.Contains(value, "://") {
			return ""
		}
		return value
	}
	qualified := func(value string) bool {
		parts := strings.SplitN(value, "/", 2)
		return len(parts) == 2 && (parts[0] == "localhost" || strings.ContainsAny(parts[0], ".:"))
	}
	actual := strip(root.ImageURL)
	if actual == "" {
		return invalidVMDKConsumer()
	}
	if !qualified(actual) {
		// Worker resolves bare task names against its internal registry, not
		// Docker Hub. Use exact task identity here, never Docker defaults.
		if actual == strip(image) && !qualified(strip(image)) {
			if _, err := reference.ParseNormalizedNamed(actual); err == nil {
				return nil
			}
		}
		return invalidVMDKConsumer()
	}
	normalize := func(value string) string {
		value = strip(value)
		if !qualified(value) {
			return ""
		}
		parsed, err := reference.ParseNormalizedNamed(value)
		if err != nil {
			return ""
		}
		return parsed.String()
	}
	if expected := normalize(local); expected != "" && normalize(actual) == expected {
		return nil
	}
	return invalidVMDKConsumer()
}

func (v *VMBuildItem) validateVMDKRuntimeBinding() error {
	if v.SkipVersionStore {
		return nil
	} // delivery-only artifact, never a VM start.
	reader := v.readVMRootImports
	if reader == nil {
		reader = func() (string, error) {
			manager := db.GetManager()
			if manager == nil || manager.ComponentK8sAttributeDao() == nil {
				return "", invalidVMDKConsumer()
			}
			attr, err := manager.ComponentK8sAttributeDao().GetByComponentIDAndName(v.ServiceID, dbmodel.K8sAttributeNameVMDiskImports)
			if err != nil || attr == nil {
				return "", invalidVMDKConsumer()
			}
			return attr.AttributeValue, nil
		}
	}
	raw, err := reader()
	if err != nil {
		return invalidVMDKConsumer()
	}
	return validateVMDKRuntimeConsumer(raw, v.Image, v.localImageName())
}
