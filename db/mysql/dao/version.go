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

package dao

import (
	"github.com/goodrain/rainbond/db/model"

	"fmt"

	"time"

	"github.com/jinzhu/gorm"
)

//DeleteVersionByEventID DeleteVersionByEventID
func (c *VersionInfoDaoImpl) DeleteVersionByEventID(eventID string) error {
	version := &model.VersionInfo{
		EventID: eventID,
	}
	if err := c.DB.Where("event_id = ? ", eventID).Delete(version).Error; err != nil {
		return err
	}
	return nil
}

//DeleteVersionByServiceID DeleteVersionByServiceID
func (c *VersionInfoDaoImpl) DeleteVersionByServiceID(serviceID string) error {
	var version model.VersionInfo
	if err := c.DB.Where("service_id = ? ", serviceID).Delete(&version).Error; err != nil {
		return err
	}
	return nil
}

//AddModel AddModel
func (c *VersionInfoDaoImpl) AddModel(mo model.Interface) error {
	result := mo.(*model.VersionInfo)
	var oldResult model.VersionInfo
	if ok := c.DB.Where("build_version=? and service_id=?", result.BuildVersion, result.ServiceID).Find(&oldResult).RecordNotFound(); ok {
		if err := c.DB.Create(result).Error; err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("service %s build version %s is exist", result.ServiceID, result.BuildVersion)
}

//UpdateModel UpdateModel
func (c *VersionInfoDaoImpl) UpdateModel(mo model.Interface) error {
	result := mo.(*model.VersionInfo)
	if err := c.DB.Save(result).Error; err != nil {
		return err
	}
	return nil
}

//VersionInfoDaoImpl VersionInfoDaoImpl
type VersionInfoDaoImpl struct {
	DB *gorm.DB
}

//GetVersionByEventID get version by event id
func (c *VersionInfoDaoImpl) GetVersionByEventID(eventID string) (*model.VersionInfo, error) {
	var result model.VersionInfo
	if err := c.DB.Where("event_id=?", eventID).Find(&result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			//return messageRaw, nil
		}
		return nil, err
	}
	return &result, nil
}

//GetVersionByDeployVersion get version by deploy version
func (c *VersionInfoDaoImpl) GetVersionByDeployVersion(version, serviceID string) (*model.VersionInfo, error) {
	var result model.VersionInfo
	if err := c.DB.Where("build_version =? and service_id = ?", version, serviceID).Find(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

//GetVersionByServiceID get versions by service id
//only return success version info
func (c *VersionInfoDaoImpl) GetVersionByServiceID(serviceID string) ([]*model.VersionInfo, error) {
	var result []*model.VersionInfo
	if err := c.DB.Where("service_id=? and final_status=?", serviceID, "success").Find(&result).Error; err != nil {
		return nil, err
	}
	return result, nil
}

//GetVersionInfo get version info by service ids
func (c *VersionInfoDaoImpl) GetVersionInfo(timePoint time.Time, serviceIDs []string) ([]*model.VersionInfo, error) {
	var result []*model.VersionInfo

	if err := c.DB.Where("service_id in (?) and create_time  < ?", serviceIDs, timePoint).Find(&result).Order("create_time asc").Error; err != nil {
		return nil, err
	}
	return result, nil

}

//DeleteVersionInfo delete version
func (c *VersionInfoDaoImpl) DeleteVersionInfo(obj *model.VersionInfo) error {
	if err := c.DB.Delete(obj).Error; err != nil {
		return err
	}
	return nil
}

//DeleteFailureVersionInfo delete failure version
func (c *VersionInfoDaoImpl) DeleteFailureVersionInfo(timePoint time.Time, status string, serviceIDs []string) error {
	if err := c.DB.Where("service_id in (?) and create_time  < ? and final_status = ?", serviceIDs, timePoint, status).Delete(&model.VersionInfo{}).Error; err != nil {
		return err
	}
	return nil
}

//SearchVersionInfo query version count >5
func (c *VersionInfoDaoImpl) SearchVersionInfo() ([]*model.VersionInfo, error) {
	var result []*model.VersionInfo
	if err := c.DB.Table("version_info").Select("service_id").Group("service_id").Having("count(ID) > ?", 5).Scan(&result).Error; err != nil {
		return nil, err
	}
	return result, nil
}
