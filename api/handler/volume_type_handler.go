// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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

package handler

import (
	"encoding/json"
	"strings"

	"fmt"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/worker/client"
	"github.com/goodrain/rainbond/worker/server/pb"
	"github.com/sirupsen/logrus"
	// pb "github.com/goodrain/rainibond/worker/server/pb"
)

// VolumeTypeHandler LicenseAction
type VolumeTypeHandler interface {
	VolumeTypeVar(action string, vtm *dbmodel.TenantServiceVolumeType) error
	GetAllVolumeTypes() ([]*apimodel.VolumeTypeStruct, error)
	GetAllVolumeTypesByPage(page int, pageSize int) ([]*apimodel.VolumeTypeStruct, error)
	GetVolumeTypeByType(volumeType string) (*dbmodel.TenantServiceVolumeType, error)
	GetAllStorageClasses() ([]*pb.StorageClassDetail, error)
	VolumeTypeAction(action, volumeTypeID string) error
	DeleteVolumeType(volumeTypeID string) error
	SetVolumeType(vtm *apimodel.VolumeTypeStruct) error
	UpdateVolumeType(dbVolume *dbmodel.TenantServiceVolumeType, vol *apimodel.VolumeTypeStruct) error
}

var defaultVolumeTypeHandler VolumeTypeHandler

// CreateVolumeTypeManger create VolumeType manager
func CreateVolumeTypeManger(statusCli *client.AppRuntimeSyncClient) *VolumeTypeAction {
	return &VolumeTypeAction{statusCli: statusCli}
}

// GetVolumeTypeHandler get volumeType handler
func GetVolumeTypeHandler() VolumeTypeHandler {
	return defaultVolumeTypeHandler
}

// VolumeTypeAction action
type VolumeTypeAction struct {
	statusCli *client.AppRuntimeSyncClient
}

// VolumeTypeVar volume type crud
func (vta *VolumeTypeAction) VolumeTypeVar(action string, vtm *dbmodel.TenantServiceVolumeType) error {
	switch action {
	case "add":
		logrus.Debug("add volumeType")
	case "update":
		logrus.Debug("update volumeType")
	}
	return nil
}

// GetAllVolumeTypes get all volume types
func (vta *VolumeTypeAction) GetAllVolumeTypes() ([]*apimodel.VolumeTypeStruct, error) {
	var optionList []*apimodel.VolumeTypeStruct
	volumeTypeMap := make(map[string]*dbmodel.TenantServiceVolumeType)
	volumeTypes, err := db.GetManager().VolumeTypeDao().GetAllVolumeTypes()
	if err != nil {
		logrus.Errorf("get all volumeTypes error: %s", err.Error())
		return nil, err
	}

	for _, vt := range volumeTypes {
		volumeTypeMap[vt.VolumeType] = vt
		capacityValidation := make(map[string]interface{})
		if vt.CapacityValidation != "" {
			err := json.Unmarshal([]byte(vt.CapacityValidation), &capacityValidation)
			if err != nil {
				logrus.Error(err.Error())
				return nil, fmt.Errorf("format volume type capacity validation error")
			}
		}

		storageClassDetail := make(map[string]interface{})
		if vt.StorageClassDetail != "" {
			err := json.Unmarshal([]byte(vt.StorageClassDetail), &storageClassDetail)
			if err != nil {
				logrus.Error(err.Error())
				return nil, fmt.Errorf("format storageclass detail error")
			}
		}
		accessMode := strings.Split(vt.AccessMode, ",")
		sharePolicy := strings.Split(vt.SharePolicy, ",")
		backupPolicy := strings.Split(vt.BackupPolicy, ",")
		optionList = append(optionList, &apimodel.VolumeTypeStruct{
			VolumeType:         vt.VolumeType,
			NameShow:           vt.NameShow,
			Provisioner:        vt.Provisioner,
			CapacityValidation: capacityValidation,
			Description:        vt.Description,
			AccessMode:         accessMode,
			SharePolicy:        sharePolicy,
			BackupPolicy:       backupPolicy,
			ReclaimPolicy:      vt.ReclaimPolicy,
			StorageClassDetail: storageClassDetail,
			Sort:               vt.Sort,
			Enable:             vt.Enable,
		})
	}

	return optionList, nil
}

// GetAllVolumeTypesByPage get all volume types by page
func (vta *VolumeTypeAction) GetAllVolumeTypesByPage(page int, pageSize int) ([]*apimodel.VolumeTypeStruct, error) {

	var optionList []*apimodel.VolumeTypeStruct
	volumeTypeMap := make(map[string]*dbmodel.TenantServiceVolumeType)
	volumeTypes, err := db.GetManager().VolumeTypeDao().GetAllVolumeTypesByPage(page, pageSize)
	if err != nil {
		logrus.Errorf("get all volumeTypes error: %s", err.Error())
		return nil, err
	}

	for _, vt := range volumeTypes {
		volumeTypeMap[vt.VolumeType] = vt
		capacityValidation := make(map[string]interface{})
		if vt.CapacityValidation != "" {
			err := json.Unmarshal([]byte(vt.CapacityValidation), &capacityValidation)
			if err != nil {
				logrus.Error(err.Error())
				return nil, fmt.Errorf("format volume type capacity validation error")
			}
		}

		storageClassDetail := make(map[string]interface{})
		if vt.StorageClassDetail != "" {
			err := json.Unmarshal([]byte(vt.StorageClassDetail), &storageClassDetail)
			if err != nil {
				logrus.Error(err.Error())
				return nil, fmt.Errorf("format storageclass detail error")
			}
		}
		accessMode := strings.Split(vt.AccessMode, ",")
		sharePolicy := strings.Split(vt.SharePolicy, ",")
		backupPolicy := strings.Split(vt.BackupPolicy, ",")
		optionList = append(optionList, &apimodel.VolumeTypeStruct{
			VolumeType:         vt.VolumeType,
			NameShow:           vt.NameShow,
			CapacityValidation: capacityValidation,
			Description:        vt.Description,
			AccessMode:         accessMode,
			SharePolicy:        sharePolicy,
			BackupPolicy:       backupPolicy,
			ReclaimPolicy:      vt.ReclaimPolicy,
			StorageClassDetail: storageClassDetail,
			Sort:               vt.Sort,
			Enable:             vt.Enable,
		})
	}

	return optionList, nil
}

// GetVolumeTypeByType get volume type by type
func (vta *VolumeTypeAction) GetVolumeTypeByType(volumtType string) (*dbmodel.TenantServiceVolumeType, error) {
	return db.GetManager().VolumeTypeDao().GetVolumeTypeByType(volumtType)
}

// GetAllStorageClasses get all storage class
func (vta *VolumeTypeAction) GetAllStorageClasses() ([]*pb.StorageClassDetail, error) {
	sces, err := vta.statusCli.GetStorageClasses()
	if err != nil {
		return nil, err
	}
	return sces.List, nil
}

// VolumeTypeAction open volme type or close it
func (vta *VolumeTypeAction) VolumeTypeAction(action, volumeTypeID string) error {
	return nil
}

// DeleteVolumeType delte volume type
func (vta *VolumeTypeAction) DeleteVolumeType(volumeType string) error {
	db.GetManager().VolumeTypeDao().DeleteModelByVolumeTypes(volumeType)
	return nil
}

// SetVolumeType set volume type
func (vta *VolumeTypeAction) SetVolumeType(vol *apimodel.VolumeTypeStruct) error {
	var accessMode []string
	var sharePolicy []string
	var backupPolicy []string
	jsonCapacityValidationStr, _ := json.Marshal(vol.CapacityValidation)
	jsonStorageClassDetailStr, _ := json.Marshal(vol.StorageClassDetail)
	if vol.AccessMode == nil {
		accessMode[1] = "RWO"
	} else {
		accessMode = vol.AccessMode
	}
	if vol.SharePolicy == nil {
		sharePolicy[1] = "exclusive"
	} else {
		sharePolicy = vol.SharePolicy
	}

	if vol.BackupPolicy == nil {
		backupPolicy[1] = "exclusive"
	} else {
		backupPolicy = vol.BackupPolicy
	}

	dbVolume := dbmodel.TenantServiceVolumeType{}
	dbVolume.VolumeType = vol.VolumeType
	dbVolume.NameShow = vol.NameShow
	dbVolume.CapacityValidation = string(jsonCapacityValidationStr)
	dbVolume.Description = vol.Description
	dbVolume.AccessMode = strings.Join(accessMode, ",")
	dbVolume.SharePolicy = strings.Join(sharePolicy, ",")
	dbVolume.BackupPolicy = strings.Join(backupPolicy, ",")
	dbVolume.ReclaimPolicy = vol.ReclaimPolicy
	dbVolume.StorageClassDetail = string(jsonStorageClassDetailStr) // TODO fanyangyang StorageClass规范性校验， 并返回正确的结构，将结构中的provisoner赋值
	dbVolume.Provisioner = "provisioner"                            // TODO fanyangyang 根据StorageClass获取
	dbVolume.Sort = vol.Sort
	dbVolume.Enable = vol.Enable

	err := db.GetManager().VolumeTypeDao().AddModel(&dbVolume)
	return err
}

// UpdateVolumeType update volume type
func (vta *VolumeTypeAction) UpdateVolumeType(dbVolume *dbmodel.TenantServiceVolumeType, vol *apimodel.VolumeTypeStruct) error {
	var accessMode []string
	var sharePolicy []string
	var backupPolicy []string
	jsonCapacityValidationStr, _ := json.Marshal(vol.CapacityValidation)
	jsonStorageClassDetailStr, _ := json.Marshal(vol.StorageClassDetail)
	if vol.AccessMode == nil {
		accessMode[1] = "RWO"
	} else {
		accessMode = vol.AccessMode
	}
	if vol.SharePolicy == nil {
		sharePolicy[1] = "exclusive"
	} else {
		sharePolicy = vol.SharePolicy
	}

	if vol.BackupPolicy == nil {
		backupPolicy[1] = "exclusive"
	} else {
		backupPolicy = vol.BackupPolicy
	}

	dbVolume.VolumeType = vol.VolumeType
	dbVolume.NameShow = vol.NameShow
	dbVolume.CapacityValidation = string(jsonCapacityValidationStr)
	dbVolume.Description = vol.Description
	dbVolume.AccessMode = strings.Join(accessMode, ",")
	dbVolume.SharePolicy = strings.Join(sharePolicy, ",")
	dbVolume.BackupPolicy = strings.Join(backupPolicy, ",")
	dbVolume.ReclaimPolicy = vol.ReclaimPolicy
	dbVolume.StorageClassDetail = string(jsonStorageClassDetailStr)
	dbVolume.Sort = vol.Sort
	dbVolume.Enable = vol.Enable

	err := db.GetManager().VolumeTypeDao().UpdateModel(dbVolume)
	return err
}
