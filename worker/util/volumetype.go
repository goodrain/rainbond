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
// 本文件定义了与存储类型和容量验证相关的工具函数，主要用于将 Kubernetes StorageClass 转换为 Rainbond 平台的存储卷类型
// 并验证存储容量的合法性。

// 1. `TransStorageClass2RBDVolumeType` 函数：
//    - 该函数用于将 Kubernetes 的 StorageClass 对象转换为 Rainbond 平台的 TenantServiceVolumeType 对象。
//    - 如果 StorageClass 名称为 Rainbond 的特定值（如 `RainbondStatefuleShareStorageClass` 或 `RainbondStatefuleLocalStorageClass`），
//      则会直接返回对应的共享文件卷类型或本地卷类型。
//    - 对于其他 StorageClass，函数会将其转换为 TenantServiceVolumeType 对象，包括存储类型名称、容量验证、存储类详细信息、
//      配置等信息。
//    - 函数还支持通过 StorageClass 的注解自定义显示名称。

// 2. `ValidateVolumeCapacity` 函数：
//    - 该函数用于验证存储容量是否符合给定的容量验证规则。
//    - 函数从 JSON 格式的验证规则字符串中解析出最小值 (`min`) 和最大值 (`max`) 限制，
//      然后检查给定的容量是否在这个范围内。
//    - 如果容量不符合规则，函数会返回相应的错误信息。

// 总的来说，这些函数帮助 Rainbond 平台与 Kubernetes 的存储系统进行集成，
// 实现了存储类型的转换和容量验证的功能，确保了存储资源的合理使用。

package util

import (
	"encoding/json"
	"fmt"
	"strings"

	dbmodel "github.com/goodrain/rainbond/db/model"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
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
	if sc.GetName() == v1.RainbondStatefuleShareStorageClass {
		return &dbmodel.TenantServiceVolumeType{VolumeType: dbmodel.ShareFileVolumeType.String()}
	}
	if sc.GetName() == v1.RainbondStatefuleLocalStorageClass {
		return &dbmodel.TenantServiceVolumeType{VolumeType: dbmodel.LocalVolumeType.String()}
	}
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
