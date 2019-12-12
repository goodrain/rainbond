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
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/worker/client"
	"github.com/goodrain/rainbond/worker/server/pb"
	// pb "github.com/goodrain/rainibond/worker/server/pb"
)

//VolumeTypeHandler LicenseAction
type VolumeTypeHandler interface {
	VolumeTypeVar(action string, vtm *dbmodel.TenantServiceVolumeType) error
	GetAllVolumeTypes() ([]*dbmodel.TenantServiceVolumeType, error)
	GetVolumeTypeByType(volumeType string) (*dbmodel.TenantServiceVolumeType, error)
	GetAllStorageClasses() ([]*pb.StorageClassDetail, error)
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
func (vta *VolumeTypeAction) GetAllVolumeTypes() ([]*dbmodel.TenantServiceVolumeType, error) {
	return db.GetManager().VolumeTypeDao().GetAllVolumeTypes()
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
