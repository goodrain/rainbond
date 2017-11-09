package option

import (
	"github.com/urfave/cli"
	"os"
	"github.com/Sirupsen/logrus"
	"io/ioutil"
	"encoding/json"
	"strings"
	"rainbond/pkg/grctl/clients"
)
var config Config
type Config struct {
	//RegionMysql   *RegionMysql `json:"RegionMysql"`
	Kubernets     *Kubernets   `json:"Kubernets"`
	RegionAPI     *RegionAPI   `json:"RegionAPI"`
	//DockerLogPath string       `json:"DockerLogPath"`
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
		return LoadConfigByRegion(c, ctx)
	}
	data, err := ioutil.ReadFile(ctx.GlobalString("config"))
	if err != nil {
		logrus.Warning("Read config file error ,will get config from region.", err.Error())
		return LoadConfigByRegion(c, ctx)
	}
	if err := json.Unmarshal(data, &c); err != nil {
		logrus.Warning("Read config file error ,will get config from region.", err.Error())
		return LoadConfigByRegion(c, ctx)
	}
	if c.Kubernets == nil  {
		return LoadConfigByRegion(c, ctx)
	}
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
	data, err := clients.LoadConfig(c.RegionAPI.URL, c.RegionAPI.Token)
	if err != nil {
		logrus.Error("Get config from region error.", err.Error())
		os.Exit(1)
	}
	//if c.RegionMysql == nil {
	//	c.RegionMysql = &RegionMysql{
	//		URL:      fmt.Sprintf("%s:%s", data["db"]["HOST"], data["db"]["PORT"]),
	//		User:     data["db"]["USER"].(string),
	//		Pass:     data["db"]["PASSWORD"].(string),
	//		Database: data["db"]["NAME"].(string),
	//	}
	//}
	if c.Kubernets == nil {
		c.Kubernets = &Kubernets{
			Master: strings.Replace(data["k8s"]["url"].(string), "/api/v1", "", -1),
		}
	}
	config = c
	return c, nil
}

func GetConfig() Config {
	return config
}
