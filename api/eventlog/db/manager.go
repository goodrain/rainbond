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
)

type Manager interface {
	SaveMessage([]*EventLogMessage) error
	Close() error
	GetMessages(id, level string, length int) (interface{}, error)
}

// NewManager 创建存储管理器
func NewManager(storageType, storageHomePath string) (Manager, error) {
	switch storageType {
	case "file":
		return &filePlugin{
			homePath: storageHomePath,
		}, nil
	case "eventfile":
		return &EventFilePlugin{
			HomePath: storageHomePath,
		}, nil
	default:
		return nil, fmt.Errorf("do not support plugin")
	}
}
