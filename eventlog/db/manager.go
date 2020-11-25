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

import (
	"fmt"

	"github.com/goodrain/rainbond/eventlog/conf"
	"github.com/sirupsen/logrus"
)

type Manager interface {
	SaveMessage([]*EventLogMessage) error
	Close() error
	GetMessages(id, level string, length int) (interface{}, error)
}

//NewManager 创建存储管理器
func NewManager(conf conf.DBConf, log *logrus.Entry) (Manager, error) {
	switch conf.Type {
	case "file":
		return &filePlugin{
			homePath: conf.HomePath,
		}, nil
	case "eventfile":
		return &EventFilePlugin{
			HomePath: conf.HomePath,
		}, nil
	default:
		return nil, fmt.Errorf("do not support plugin")
	}
}
