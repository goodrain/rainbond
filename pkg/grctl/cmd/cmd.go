package cmd
import (
	"github.com/urfave/cli"
	"github.com/Sirupsen/logrus"
	"os"
	conf "rainbond/cmd/grctl/option"
	"rainbond/pkg/grctl/clients"
)

func GetCmds() []cli.Command {
	cmds:=[]cli.Command{}
	cmds=append(cmds,NewCmdBatchStop())
	cmds=append(cmds,NewCmdStartService())
	cmds=append(cmds,NewCmdStopService())
	cmds=append(cmds,NewCmdTenant())
	cmds=append(cmds,NewCmdTenantRes())
	//todo
	return cmds
}
func Common(c *cli.Context) {
	config, err := conf.LoadConfig(c)
	if err != nil {
		logrus.Error("Load config file error.", err.Error())
		os.Exit(1)
	}
	//if err := db.InitDB(*config.RegionMysql); err != nil {
	//	os.Exit(1)
	//}
	if err := clients.InitClient(*config.Kubernets); err != nil {
		os.Exit(1)
	}
	//clients.SetInfo(config.RegionAPI.URL, config.RegionAPI.Token)
	clients.InitRegionClient(*config.RegionAPI)
}