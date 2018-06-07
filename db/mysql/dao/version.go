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

	"github.com/jinzhu/gorm"
	"time"
	"os"
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
	if ok := c.DB.Where("event_id=?", result.EventID).Find(&oldResult).RecordNotFound(); ok {
		if err := c.DB.Create(result).Error; err != nil {
			return err
		}
	} else {
		fmt.Errorf("version is exist")
		return nil
	}
	return nil
}

//UpdateModel UpdateModel
func (c *VersionInfoDaoImpl) UpdateModel(mo model.Interface) error {
	result := mo.(*model.VersionInfo)
	if err := c.DB.Save(result).Error; err != nil {
		return err
	}
	return nil
}

//EventLogMessageDaoImpl EventLogMessageDaoImpl
type VersionInfoDaoImpl struct {
	DB *gorm.DB
}

//GetEventLogMessages get event log message
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

//GetEventLogMessages get event log message
func (c *VersionInfoDaoImpl) GetVersionByDeployVersion(version, serviceID string) (*model.VersionInfo, error) {
	var result model.VersionInfo
	if err := c.DB.Where("build_version =? and service_id = ?", version, serviceID).Find(&result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, err
		}
		return nil, err
	}
	return &result, nil
}

//GetEventLogMessages get event log message
func (c *VersionInfoDaoImpl) GetVersionByServiceID(serviceID string) ([]*model.VersionInfo, error) {
	var result []*model.VersionInfo
	if err := c.DB.Where("service_id=?", serviceID).Find(&result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			//return messageRaw, nil
		}
		return nil, err
	}
	return result, nil
}

func (c *VersionInfoDaoImpl) CheanViesion() {
	var result []*model.VersionInfo
	timestamp := time.Now().Unix()
	c.DB.Where("create_time < ? AND delivered_type = ?", timestamp,"slug").Find(&result)
	fmt.Println(len(result),"源码查询数量")
	for _,v := range result {
		path := v.DeliveredPath
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			fmt.Println("源码文件不存在")
			continue
		}
		if err != nil {
			continue
		}
		//os.Remove(path) //删除文件
		fmt.Println(path, "源码文件删除成功")

	}
	var image_result []*model.VersionInfo
	c.DB.Where("create_time < ? AND delivered_type = ?", timestamp,"image").Find(&image_result)
	fmt.Println(len(image_result),"镜像查询数量")
	//for _,v := range result {
	//	image_path := v.DeliveredPath
	//	dc, _ := client.NewEnvClient()
	//	err := sources.ImageRemove(dc,image_path)
	//	if err!= nil{
	//		fmt.Println("错误",err)
	//	}else{
	//		fmt.Println("删除镜像成功")
	//	}
	//
	//}

}
