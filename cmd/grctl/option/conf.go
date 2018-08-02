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
	"io/ioutil"
	"os"
	"path"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/api/region"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/urfave/cli"
	yaml "gopkg.in/yaml.v2"
	//"strings"
)

var config Config

//Config Config
type Config struct {
	RegionMysql   RegionMysql    `yaml:"region_db"`
	Kubernets     Kubernets      `yaml:"kube"`
	RegionAPI     region.APIConf `yaml:"region_api"`
	DockerLogPath string         `yaml:"docker_log_path"`
}

//RegionMysql RegionMysql
type RegionMysql struct {
	URL      string `yaml:"url"`
	Pass     string `yaml:"pass"`
	User     string `yaml:"user"`
	Database string `yaml:"database"`
}

//Kubernets Kubernets
type Kubernets struct {
	Master string `yaml:"master"`
}

//LoadConfig 加载配置
func LoadConfig(ctx *cli.Context) (Config, error) {
	config = Config{
		RegionAPI: region.APIConf{
			Endpoints: []string{"http://127.0.0.1:8888"},
		},
		RegionMysql: RegionMysql{
			User:     os.Getenv("MYSQL_USER"),
			Pass:     os.Getenv("MYSQL_PASS"),
			URL:      os.Getenv("MYSQL_URL"),
			Database: os.Getenv("MYSQL_DB"),
		},
	}
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

//GetConfig GetConfig
func GetConfig() Config {
	return config
}
