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

package config

import "context"

//Operation 实例操作类型
type Operation int

const (
	ADD Operation = iota
	DELETE
	UPDATE
	SYNC
)

//DiscoverConfig discover config
type DiscoverConfig struct {
	Ctx                  context.Context
	EtcdClusterEndpoints []string
}

type Endpoint struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Weight int    `json:"weight"`
	Mode   int    `json:"-"` //0 表示URL变化，1表示Weight变化 ,2表示全变化
}
