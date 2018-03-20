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
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	//"strings"
)

var config Config

//Config Config
type Config struct {
	RegionMysql   RegionMysql `json:"RegionMysql"`
	Kubernets     Kubernets   `json:"Kubernets"`
	RegionAPI     RegionAPI   `json:"RegionAPI"`
	DockerLogPath string      `json:"DockerLogPath"`
}

//RegionMysql RegionMysql
type RegionMysql struct {
	URL      string `json:"URL"`
	Pass     string `json:"Pass"`
	User     string `json:"User"`
	Database string `json:"Database"`
}

//Kubernets Kubernets
type Kubernets struct {
	Master string
}

//RegionAPI RegionAPI
type RegionAPI struct {
	URL   string
	Token string
	Type  string
}

//LoadConfig 加载配置
func LoadConfig(ctx *cli.Context) (Config, error) {
	config = Config{
		RegionAPI: RegionAPI{
			URL: "http://127.0.0.1:8888",
		},
		RegionMysql: RegionMysql{
			User:     os.Getenv("MYSQL_USER"),
			Pass:     os.Getenv("MYSQL_PASS"),
			URL:      os.Getenv("MYSQL_URL"),
			Database: os.Getenv("MYSQL_DB"),
		},
	}
	_, err := os.Stat(ctx.GlobalString("config"))
	if err != nil {
		return config, err
	}
	data, err := ioutil.ReadFile(ctx.GlobalString("config"))
	if err != nil {
		logrus.Warning("Read config file error ,will get config from region.", err.Error())
		return config, err
	}
	if err := json.Unmarshal(data, &config); err != nil {
		logrus.Warning("Read config file error ,will get config from region.", err.Error())
		return config, err
	}
	return config, nil
}

//GetConfig GetConfig
func GetConfig() Config {
	return config
}
