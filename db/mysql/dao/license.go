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
)

//LicenseDaoImpl license model 管理
type LicenseDaoImpl struct {
	DB *gorm.DB
}

//AddModel AddModel
func (l *LicenseDaoImpl) AddModel(mo model.Interface) error {
	license := mo.(*model.LicenseInfo)
	var oldLicense model.LicenseInfo
	if ok := l.DB.Where("license=?", license.License).Find(&oldLicense).RecordNotFound(); ok {
		if err := l.DB.Create(license).Error; err != nil {
			return err
		}
	} else {
		return fmt.Errorf("license is exist")
	}
	return nil
}

//UpdateModel UpdateModel
func (l *LicenseDaoImpl) UpdateModel(mo model.Interface) error {
	return nil
}

//DeleteLicense DeleteLicense
func (l *LicenseDaoImpl) DeleteLicense(token string) error {
	return nil
}

//ListLicenses ListLicenses
func (l *LicenseDaoImpl) ListLicenses() ([]*model.LicenseInfo, error) {
	var licenses []*model.LicenseInfo
	if err := l.DB.Find(&licenses).Error; err != nil {
		return nil, err
	}
	return licenses, nil
}
