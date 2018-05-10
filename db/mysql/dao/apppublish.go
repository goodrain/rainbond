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

	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
)

//AddModel AddModel
func (c *AppPublishDaoImpl) AddModel(mo model.Interface) error {
	result := mo.(*model.AppPublish)
	var oldResult model.AppPublish
	if ok := c.DB.Where("service_key=? and app_version=?", result.ServiceKey, result.AppVersion).Find(&oldResult).RecordNotFound(); ok {
		if err := c.DB.Create(result).Error; err != nil {
			if err != nil {
				logrus.Errorf("error save app publish,details %s", err.Error())
			}
			return err
		}
	} else {

		oldResult.Status = result.Status
		if err := c.DB.Save(&oldResult).Error; err != nil {
			return err
		}
		return nil
	}
	return nil
}

//UpdateModel UpdateModel
func (c *AppPublishDaoImpl) UpdateModel(mo model.Interface) error {
	return nil
}

//AppPublishDaoImpl EventLogMessageDaoImpl
type AppPublishDaoImpl struct {
	DB *gorm.DB
}

//GetAppPublish get event log message
func (c *AppPublishDaoImpl) GetAppPublish(serviceKey, appVersion string) (*model.AppPublish, error) {
	var result model.AppPublish
	if err := c.DB.Where("service_key = ? and app_version =?", serviceKey, appVersion).Find(&result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			//return messageRaw, nil
			return &model.AppPublish{
				Status: "failure",
			}, nil
		}
		return nil, err
	}
	return &result, nil
}
