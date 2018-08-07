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

package code

import (
	"fmt"
	"io/ioutil"
	"path"

	"github.com/Sirupsen/logrus"

	"github.com/goodrain/rainbond/util"
	"gopkg.in/yaml.v2"
)

//RainbondFileConfig 云帮源码配置文件
type RainbondFileConfig struct {
	Language  string            `yaml:"language"`
	BuildPath string            `yaml:"buildpath"`
	Ports     []Port            `yaml:"ports"`
	Envs      map[string]string `yaml:"envs"`
	Cmd       string            `yaml:"cmd"`
}

//Port Port
type Port struct {
	Port     int    `yaml:"port"`
	Protocol string `yaml:"procotol"`
}

//ReadRainbondFile 读取云帮代码配置
func ReadRainbondFile(homepath string) (*RainbondFileConfig, error) {
	if ok, _ := util.FileExists(path.Join(homepath, "rainbondfile")); !ok {
		return nil, ErrRainbondFileNotFound
	}
	body, err := ioutil.ReadFile(path.Join(homepath, "rainbondfile"))
	if err != nil {
		logrus.Error("read rainbond file error,", err.Error())
		return nil, fmt.Errorf("read rainbond file error")
	}
	var rbdfile RainbondFileConfig
	if err := yaml.Unmarshal(body, &rbdfile); err != nil {
		logrus.Error("marshal rainbond file error,", err.Error())
		return nil, fmt.Errorf("marshal rainbond file error")
	}
	return &rbdfile, nil
}
