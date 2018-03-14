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

package exector

import (
	"os"
	"github.com/akkuman/parseConfig"
	"github.com/Sirupsen/logrus"
)

const confPath = "plugins/config.json"

//BuildConfig 构建参数
var BuildConfig parseConfig.Config

func init() {
	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		logrus.Errorf("config.json is not exist")
	}
	BuildConfig = parseConfig.New(confPath)
	logrus.Debugf("format config json success")
}

//GetBuilderConfig 获取builder配置
func GetBuilderConfig() parseConfig.Config {
	return BuildConfig
}