// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.
 
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
	"github.com/urfave/cli"
	"os"
	"github.com/Sirupsen/logrus"
	"io/ioutil"
	"encoding/json"
	//"strings"

)
var config Config
type Config struct {
	RegionMysql   *RegionMysql `json:"RegionMysql"`
	Kubernets     *Kubernets   `json:"Kubernets"`
	RegionAPI     *RegionAPI   `json:"RegionAPI"`
	DockerLogPath string       `json:"DockerLogPath"`
}
type RegionMysql struct {
	URL      string `json:"URL"`
	Pass     string `json:"Pass"`
	User     string `json:"User"`
	Database string `json:"Database"`
}
type Kubernets struct {
	Master string
}
type RegionAPI struct {
	URL   string
	Token string
	Type  string
}


func LoadConfig(ctx *cli.Context) (Config, error) {
	var c Config
	_, err := os.Stat(ctx.GlobalString("config"))
	if err != nil {
		//return LoadConfigByRegion(c, ctx)
		return c,err
	}
	data, err := ioutil.ReadFile(ctx.GlobalString("config"))
	if err != nil {
		logrus.Warning("Read config file error ,will get config from region.", err.Error())
		//return LoadConfigByRegion(c, ctx)
		return c,err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		logrus.Warning("Read config file error ,will get config from region.", err.Error())
		//return LoadConfigByRegion(c, ctx)
		return c,err
	}
	//if c.Kubernets == nil  {
	//	return LoadConfigByRegion(c, ctx)
	//}
	config = c
	return c, nil
}

//LoadConfigByRegion 通过regionAPI获取配置
func LoadConfigByRegion(c Config, ctx *cli.Context) (Config, error) {
	if c.RegionAPI == nil {
		c.RegionAPI = &RegionAPI{
			URL:   ctx.GlobalString("region.url"),
			Token: "",
		}
	}
	//data, err := region.LoadConfig(c.RegionAPI.URL, c.RegionAPI.Token)
	//if err != nil {
	//	logrus.Error("Get config from region error.", err.Error())
	//	return c,err
	//	//os.Exit(1)
	//}
	//if c.Kubernets == nil {
	//	c.Kubernets = &Kubernets{
	//		Master: strings.Replace(data["k8s"]["url"].(string), "/api/v1", "", -1),
	//	}
	//}
	config = c
	return c, nil
}

func GetConfig() Config {
	return config
}
