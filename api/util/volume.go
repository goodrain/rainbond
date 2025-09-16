package util

import (
	"strings"

	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/google/uuid"
)

// SetVolumeDefaultValue set volume default value
func SetVolumeDefaultValue(info *dbmodel.TenantServiceVolume) {
	if info.VolumeName == "" {
		info.VolumeName = uuid.New().String()
	}

	if info.AccessMode != "" {
		info.AccessMode = strings.ToUpper(info.AccessMode)
	} else {
		info.AccessMode = "RWO"
	}

	if info.SharePolicy == "" {
		info.SharePolicy = "exclusive"
	}

	if info.BackupPolicy == "" {
		info.BackupPolicy = "exclusive"
	}

	if info.ReclaimPolicy == "" {
		info.ReclaimPolicy = "retain"
	}
}
