// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package util

import (
	"encoding/json"
	"fmt"
	"strings"

	dbmodel "github.com/goodrain/rainbond/db/model"
	storagev1 "k8s.io/api/storage/v1"
)

var defaultcapacityValidation map[string]interface{}
var defaultAccessMode = []string{"RWO"}
var defaultBackupPolicy = []string{"exclusive"}
var defaultSharePolicy = []string{"exclusive"}

func init() {
	defaultcapacityValidation = make(map[string]interface{})
	defaultcapacityValidation["min"] = 0
	defaultcapacityValidation["required"] = true
	defaultcapacityValidation["max"] = 999999999
}

// TransStorageClass2RBDVolumeType transfer k8s storageclass 2 rbd volumeType
func TransStorageClass2RBDVolumeType(sc *storagev1.StorageClass) *dbmodel.TenantServiceVolumeType {
	scbs, _ := json.Marshal(sc)
	cvbs, _ := json.Marshal(defaultcapacityValidation)

	volumeType := &dbmodel.TenantServiceVolumeType{
		VolumeType:         sc.GetName(),
		NameShow:           sc.GetName(),
		CapacityValidation: string(cvbs),
		StorageClassDetail: string(scbs),
		Provisioner:        sc.Provisioner,
		AccessMode:         strings.Join(defaultAccessMode, ","),
		BackupPolicy:       strings.Join(defaultBackupPolicy, ","),
		SharePolicy:        strings.Join(defaultSharePolicy, ","),
		Sort:               999,
		Enable:             true,
	}
	volumeType.ReclaimPolicy = "Retain"
	if sc.ReclaimPolicy != nil {
		volumeType.ReclaimPolicy = fmt.Sprintf("%v", *sc.ReclaimPolicy)
	}
	if sc.Annotations != nil {
		if name, ok := sc.Annotations["rbd_volume_name"]; ok {
			volumeType.NameShow = name
		}
	}
	return volumeType
}

// ValidateVolumeCapacity validate volume capacity
func ValidateVolumeCapacity(validation string, capacity int64) error {
	validator := make(map[string]interface{})
	if err := json.Unmarshal([]byte(validation), &validator); err != nil {
		return err
	}

	if min, ok := validator["min"].(int64); ok {
		if capacity < min {
			return fmt.Errorf("volume capacity %v less than min value %v", capacity, min)
		}
	}

	if max, ok := validator["max"].(int64); ok {
		if capacity > max {
			return fmt.Errorf("volume capacity %v more than max value %v", capacity, max)
		}
	}

	return nil
}
