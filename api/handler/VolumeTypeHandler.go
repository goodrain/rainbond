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

	"github.com/Sirupsen/logrus"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/worker/client"
	"github.com/goodrain/rainbond/worker/server/pb"
	// pb "github.com/goodrain/rainibond/worker/server/pb"
)

//VolumeTypeHandler LicenseAction
type VolumeTypeHandler interface {
	VolumeTypeVar(action string, vtm *dbmodel.TenantServiceVolumeType) error
	GetAllVolumeTypes() ([]*api_model.VolumeTypeStruct, error)
	GetVolumeTypeByType(volumeType string) (*dbmodel.TenantServiceVolumeType, error)
	GetAllStorageClasses() ([]*pb.StorageClassDetail, error)
	VolumeTypeAction(action, volumeTypeID string) error
	DeleteVolumeType(volumeTypeID string) error
	SetVolumeType(vtm *api_model.VolumeTypeStruct) error
}

var defaultVolumeTypeHandler VolumeTypeHandler

//CreateVolumeTypeManger create VolumeType manager
func CreateVolumeTypeManger(statusCli *client.AppRuntimeSyncClient) *VolumeTypeAction {
	return &VolumeTypeAction{statusCli: statusCli}
}

//GetVolumeTypeHandler get volumeType handler
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
func (vta *VolumeTypeAction) GetAllVolumeTypes() ([]*api_model.VolumeTypeStruct, error) {
	storageClasses, err := vta.GetAllStorageClasses()
	if err != nil {
		return nil, err
	}
	var optionList []*api_model.VolumeTypeStruct
	volumeTypeMap := make(map[string]*dbmodel.TenantServiceVolumeType)
	volumeTypes, err := db.GetManager().VolumeTypeDao().GetAllVolumeTypes()
	if err != nil {
		logrus.Errorf("get all volumeTypes error: %s", err.Error())
		return nil, err
	}

	for _, vt := range volumeTypes {
		volumeTypeMap[vt.VolumeType] = vt
	}

	for _, sc := range storageClasses {
		vt := util.ParseVolumeTypeOption(sc)
		opt := &api_model.VolumeTypeStruct{}
		opt.VolumeType = vt // volumeType is storageclass's name, but share-file/memoryfs/local
		if dbvt, ok := volumeTypeMap[opt.VolumeType]; ok {
			util.HackVolumeOptionDetailFromDB(opt, dbvt)
		} else {
			util.HackVolumeOptionDetail(vt, opt, sc.GetName(), sc.GetReclaimPolicy(), sc.VolumeBindingMode, sc.AllowVolumeExpansion)
		}

		optionList = append(optionList, opt)
	}
	// TODO 管理后台支持自定义StorageClass，则内容与db中的数据进行融合，进行更多的业务逻辑
	memoryVolumeType := &api_model.VolumeTypeStruct{VolumeType: dbmodel.MemoryFSVolumeType.String(), NameShow: "内存文件存储"}
	util.HackVolumeOptionDetailFromDB(memoryVolumeType, volumeTypeMap["memoryfs"])
	optionList = append(optionList, memoryVolumeType)
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
	// TODO 开启驱动或者关闭驱动，关闭之前需要确定该驱动是否可以因为已经绑定了存储而不能直接关闭
	return nil
}

// DeleteVolumeType delte volume type
func (vta *VolumeTypeAction) DeleteVolumeType(volumeType string) error {
	db.GetManager().VolumeTypeDao().DeleteModelByVolumeTypes(volumeType)
	return nil
}

// SetVolumeType set volume type
func (vta *VolumeTypeAction) SetVolumeType(vol *api_model.VolumeTypeStruct) error {
	var accessMode []string
	var sharePolicy []string
	var backupPolicy []string
	jsonStr, _ := json.Marshal(vol.CapacityValidation)
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
	dbVolume.CapacityValidation = string(jsonStr)
	dbVolume.Description = vol.Description
	dbVolume.AccessMode = strings.Join(accessMode, ",")
	dbVolume.SharePolicy = strings.Join(sharePolicy, ",")
	dbVolume.BackupPolicy = strings.Join(backupPolicy, ",")
	dbVolume.ReclaimPolicy = vol.ReclaimPolicy
	dbVolume.Sort = vol.Sort
	dbVolume.Enable = vol.Enable

	err := db.GetManager().VolumeTypeDao().AddModel(&dbVolume)
	return err
}
