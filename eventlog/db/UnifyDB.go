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

// 文件: UnifyDB.go
// 说明: 该文件实现了与数据库交互的统一接口。文件中定义了各种操作数据库的方法，
// 用于处理应用管理平台中的数据存储和检索操作。通过这些方法，Rainbond 平台能够统一地
// 处理数据库相关的任务，确保数据操作的稳定和一致性。

package db

import (
	"time"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/eventlog/conf"
	"github.com/sirupsen/logrus"
)

// CreateDBManager -
func CreateDBManager(conf conf.DBConf) error {
	logrus.Infof("creating dbmanager ,details %v", conf)
	var tryTime time.Duration
	tryTime = 0
	var err error
	for tryTime < 4 {
		tryTime++
		if err = db.CreateManager(config.Config{
			MysqlConnectionInfo: conf.URL,
			DBType:              conf.Type,
		}); err != nil {
			logrus.Errorf("get db manager failed, try time is %v,%s", tryTime, err.Error())
			time.Sleep((5 + tryTime*10) * time.Second)
		} else {
			break
		}
	}
	if err != nil {
		logrus.Errorf("get db manager failed,%s", err.Error())
		return err
	}
	logrus.Debugf("init db manager success")
	return nil
}
