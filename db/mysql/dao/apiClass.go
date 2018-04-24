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

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

//RegionAPIClassDaoImpl RegionAPIClassDaoImpl
type RegionAPIClassDaoImpl struct {
	DB *gorm.DB
}

//AddModel 添加api分类信息
func (t *RegionAPIClassDaoImpl) AddModel(mo model.Interface) error {
	info := mo.(*model.RegionAPIClass)
	var oldInfo model.RegionAPIClass
	if ok := t.DB.Where("prefix = ? and class_level=?", info.Prefix, info.ClassLevel).Find(&oldInfo).RecordNotFound(); ok {
		if err := t.DB.Create(info).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("prefix %s is exist", info.Prefix)
	}
	return nil
}

//UpdateModel 更新api分类信息
func (t *RegionAPIClassDaoImpl) UpdateModel(mo model.Interface) error {
	info := mo.(*model.RegionAPIClass)
	if info.ID == 0 {
		return fmt.Errorf("region user info id can not be empty when update ")
	}
	if err := t.DB.Save(info).Error; err != nil {
		return err
	}
	return nil
}

//GetPrefixesByClass GetPrefixesByClass
func (t *RegionAPIClassDaoImpl) GetPrefixesByClass(apiClass string) ([]*model.RegionAPIClass, error) {
	var racs []*model.RegionAPIClass
	if err := t.DB.Select("prefix").Where("class_level =?", apiClass).Find(&racs).Error; err != nil {
		return nil, err
	}
	return racs, nil
}

//DeletePrefixInClass DeletePrefixInClass
func (t *RegionAPIClassDaoImpl) DeletePrefixInClass(apiClass, prefix string) error {
	relation := &model.RegionAPIClass{
		ClassLevel: apiClass,
		Prefix:     prefix,
	}
	if err := t.DB.Where("class_level=? and prefix=?", apiClass, prefix).Delete(relation).Error; err != nil {
		return err
	}
	return nil
}
