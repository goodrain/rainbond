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

package option

import (
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	yaml "gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path"
	//"strings"
)

var config Config

// Config Config
type Config struct {
	Kubernets     Kubernets `yaml:"kube"`
	DockerLogPath string    `yaml:"docker_log_path"`
}

// RegionMysql RegionMysql
type RegionMysql struct {
	URL      string `yaml:"url"`
	Pass     string `yaml:"pass"`
	User     string `yaml:"user"`
	Database string `yaml:"database"`
}

// Kubernets Kubernets
type Kubernets struct {
	KubeConf string `yaml:"kube-conf"`
}

// LoadConfig 加载配置
func LoadConfig(ctx *cli.Context) (Config, error) {
	configfile := ctx.GlobalString("config")
	if configfile == "" {
		home, _ := sources.Home()
		configfile = path.Join(home, ".rbd", "grctl.yaml")
	}
	_, err := os.Stat(configfile)
	if err != nil {
		return config, nil
	}
	data, err := ioutil.ReadFile(configfile)
	if err != nil {
		logrus.Warning("Read config file error ,will get config from region.", err.Error())
		return config, err
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		logrus.Warning("Read config file error ,will get config from region.", err.Error())
		return config, err
	}
	return config, nil
}

// GetConfig GetConfig
func GetConfig() Config {
	return config
}

// Get TenantNamePath
func GetTenantNamePath() (tenantnamepath string, err error) {
	home, err := sources.Home()
	if err != nil {
		logrus.Warn("Get Home Dir error.", err.Error())
		return tenantnamepath, err
	}
	tenantnamepath = path.Join(home, ".rbd", "tenant.txt")
	return tenantnamepath, err
}
