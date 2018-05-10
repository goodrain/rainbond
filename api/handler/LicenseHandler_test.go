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
	"github.com/goodrain/rainbond/cmd/api/option"
	"github.com/goodrain/rainbond/api/db"
	"fmt"
	"testing"
)

func TestLicenseInfo(t *testing.T) {
	conf := option.Config{
		DBType:           "mysql",
		DBConnectionInfo: "admin:admin@tcp(127.0.0.1:3306)/region",
	}
	//创建db manager
	if err := db.CreateDBManager(conf); err != nil {
		fmt.Printf("create db manager error, %v", err)

	}
	//创建license验证 manager
	if err := CreateLicensesInfoManager(); err != nil {
		fmt.Printf("create license check manager error, %v", err)
	}
	lists, err := GetLicensesInfosHandler().ShowInfos()
	if err != nil {
		fmt.Printf("get list error, %v", err)
	}
	for _, v := range lists {
		fmt.Printf("license value is %v", v)
	}
}
