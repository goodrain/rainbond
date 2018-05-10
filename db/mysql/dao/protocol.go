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
	"fmt"

	"github.com/jinzhu/gorm"

	"github.com/goodrain/rainbond/db/model"
)

//RegionProcotolsDaoImpl RegionProcotolsDaoImpl
type RegionProcotolsDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加cloud信息
func (t *RegionProcotolsDaoImpl) AddModel(mo model.Interface) error {
	info := mo.(*model.RegionProcotols)
	var oldInfo model.RegionProcotols
	if ok := t.DB.Where("protocol_group = ? and protocol_child = ?", info.ProtocolGroup, info.ProtocolChild).Find(&oldInfo).RecordNotFound(); ok {
		if err := t.DB.Create(info).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("prococol group  %s or child %s is exist", info.ProtocolGroup, info.ProtocolChild)
	}
	return nil
}

//UpdateModel 更新cloud信息
func (t *RegionProcotolsDaoImpl) UpdateModel(mo model.Interface) error {
	info := mo.(*model.RegionProcotols)
	if info.ID == 0 {
		return fmt.Errorf("region protocol id can not be empty when update ")
	}
	if err := t.DB.Save(info).Error; err != nil {
		return err
	}
	return nil
}

//GetAllSupportProtocol 获取当前数据中心支持的所有协议
func (t *RegionProcotolsDaoImpl) GetAllSupportProtocol(version string) ([]*model.RegionProcotols, error) {
	var rpss []*model.RegionProcotols
	if err := t.DB.Where("api_version= ? and is_support = ?", version, true).Find(&rpss).Error; err != nil {
		return nil, err
	}
	return rpss, nil
}

//GetProtocolGroupByProtocolChild 获取协议族名称
func (t *RegionProcotolsDaoImpl) GetProtocolGroupByProtocolChild(
	version,
	protocolChild string) (*model.RegionProcotols, error) {
	var rps model.RegionProcotols
	if err := t.DB.Where("api_version=? and protocol_child = ?", version, protocolChild).Find(&rps).Error; err != nil {
		return nil, err
	}
	return &rps, nil
}
