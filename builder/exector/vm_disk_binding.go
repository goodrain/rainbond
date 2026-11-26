package exector

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/distribution/reference"
	"github.com/goodrain/rainbond/builder/sourceutil"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
)

type vmDiskRootConsumer struct {
	VolumeName string `json:"volume_name"`
	DiskKey    string `json:"disk_key"`
	SourceType string `json:"source_type"`
	Format     string `json:"format"`
	ImageURL   string `json:"image_url"`
}

func invalidVMDiskConsumer() error {
	return fmt.Errorf("vm_disk_runtime_binding_invalid: 当前根盘未绑定本次校验发布的磁盘镜像，请通过磁盘镜像入口重新创建，不能直接导入未经校验的下载地址")
}

func validateVMDiskRuntimeConsumer(raw, image, local string) error {
	if len(raw) == 0 || len(raw) > 1<<20 {
		return invalidVMDiskConsumer()
	}
	var keyed map[string]vmDiskRootConsumer
	var root *vmDiskRootConsumer
	if err := json.Unmarshal([]byte(raw), &keyed); err == nil {
		for key, item := range keyed {
			if item.VolumeName == "" {
				item.VolumeName = key
			}
			if item.VolumeName == "disk" {
				if root != nil {
					return invalidVMDiskConsumer()
				}
				copy := item
				root = &copy
			}
		}
	} else {
		var list []vmDiskRootConsumer
		if err = json.Unmarshal([]byte(raw), &list); err != nil {
			return invalidVMDiskConsumer()
		}
		for _, item := range list {
			name := item.VolumeName
			if name == "" {
				name = item.DiskKey
			}
			if name == "disk" {
				if root != nil {
					return invalidVMDiskConsumer()
				}
				copy := item
				root = &copy
			}
		}
	}
	if root == nil || strings.ToLower(strings.TrimSpace(root.SourceType)) != "registry" {
		return invalidVMDiskConsumer()
	}
	// CDI probes the actual disk, including legacy QCOW2 artifacts. New builds
	// preserve the original format while retaining the server-owned task target.
	if !sourceutil.IsVMPlainDiskFormat(root.Format) {
		return invalidVMDiskConsumer()
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
		return invalidVMDiskConsumer()
	}
	if !qualified(actual) {
		// Worker resolves bare task names against its internal registry, not
		// Docker Hub. Use exact task identity here, never Docker defaults.
		if actual == strip(image) && !qualified(strip(image)) {
			if _, err := reference.ParseNormalizedNamed(actual); err == nil {
				return nil
			}
		}
		return invalidVMDiskConsumer()
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
	return invalidVMDiskConsumer()
}

func (v *VMBuildItem) validateVMDiskRuntimeBinding() error {
	if v.SkipVersionStore {
		return nil
	} // delivery-only artifact, never a VM start.
	reader := v.readVMRootImports
	if reader == nil {
		reader = func() (string, error) {
			manager := db.GetManager()
			if manager == nil || manager.ComponentK8sAttributeDao() == nil {
				return "", invalidVMDiskConsumer()
			}
			attr, err := manager.ComponentK8sAttributeDao().GetByComponentIDAndName(v.ServiceID, dbmodel.K8sAttributeNameVMDiskImports)
			if err != nil || attr == nil {
				return "", invalidVMDiskConsumer()
			}
			return attr.AttributeValue, nil
		}
	}
	raw, err := reader()
	if err != nil {
		return invalidVMDiskConsumer()
	}
	return validateVMDiskRuntimeConsumer(raw, v.Image, v.localImageName())
}
