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

package db

import "github.com/goodrain/rainbond/eventlog/conf"
import "github.com/Sirupsen/logrus"

type Manager interface {
	SaveMessage([]*EventLogMessage) error
	Close() error
}

//NewManager 创建存储管理器
func NewManager(conf conf.DBConf, log *logrus.Entry) (Manager, error) {
	switch conf.Type {
	case "file":
		return &filePlugin{
			homePath: conf.HomePath,
		}, nil
	case "cockroachdb":
		cdb := &cockroachPlugin{
			conf: conf,
			url:  conf.URL,
			log:  log,
		}
		if err := cdb.open(); err != nil {
			return nil, err
		}
		return cdb, nil
	case "eventfile":
		return &EventFilePlugin{
			HomePath: conf.HomePath,
		}, nil
	default:
		my := &mysqlPlugin{
			conf: conf,
			url:  conf.URL,
			log:  log,
		}
		if err := my.open(); err != nil {
			return nil, err
		}
		return my, nil
	}
}
