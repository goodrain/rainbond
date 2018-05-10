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
	"time"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

//RegionUserInfoDaoImpl CloudDaoImpl
type RegionUserInfoDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加cloud信息
func (t *RegionUserInfoDaoImpl) AddModel(mo model.Interface) error {
	info := mo.(*model.RegionUserInfo)
	var oldInfo model.RegionUserInfo
	if ok := t.DB.Where("eid = ?", info.EID).Find(&oldInfo).RecordNotFound(); ok {
		if err := t.DB.Create(info).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("eid %s is exist", info.EID)
	}
	return nil
}

//UpdateModel 更新cloud信息
func (t *RegionUserInfoDaoImpl) UpdateModel(mo model.Interface) error {
	info := mo.(*model.RegionUserInfo)
	if info.ID == 0 {
		return fmt.Errorf("region user info id can not be empty when update ")
	}
	if err := t.DB.Save(info).Error; err != nil {
		return err
	}
	return nil
}

//GetTokenByEid GetTokenByEid
func (t *RegionUserInfoDaoImpl) GetTokenByEid(eid string) (*model.RegionUserInfo, error) {
	var rui model.RegionUserInfo
	if err := t.DB.Where("eid=?", eid).Find(&rui).Error; err != nil {
		return nil, err
	}
	return &rui, nil
}

//GetTokenByTokenID GetTokenByTokenID
func (t *RegionUserInfoDaoImpl) GetTokenByTokenID(token string) (*model.RegionUserInfo, error) {
	var rui model.RegionUserInfo
	if err := t.DB.Where("token=?", token).Find(&rui).Error; err != nil {
		return nil, err
	}
	return &rui, nil
}

//GetALLTokenInValidityPeriod GetALLTokenInValidityPeriod
func (t *RegionUserInfoDaoImpl) GetALLTokenInValidityPeriod() ([]*model.RegionUserInfo, error) {
	var ruis []*model.RegionUserInfo
	timestamp := int(time.Now().Unix())
	if err := t.DB.Select("api_range, validity_period, token").Where("validity_period > ?", timestamp).Find(&ruis).Error; err != nil {
		return nil, err
	}
	return ruis, nil
}
