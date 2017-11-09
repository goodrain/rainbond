package cmd

import (
	"github.com/urfave/cli"
	"github.com/Sirupsen/logrus"
	"rainbond/pkg/grctl/clients"
)


func NewCmdBatchStop() cli.Command {
	c:=cli.Command{
		Name: "get",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "url",
				Value: "",
				Usage: "URL of the app. eg. https://user.goodrain.com/apps/goodrain/dev-debug/detail/",
			},
		},
		Usage: "获取应用运行详细信息。grctl get PATH",
		Action: func(c *cli.Context) error {
			Common(c)
			return stopTenantService(c)
		},
	}
	return c
}
func stopTenantService(c *cli.Context) error  {
	//GET /v2/tenants/{tenant_name}/services/{service_alias}
	//POST /v2/tenants/{tenant_name}/services/{service_alias}/stop

	tenantID := c.Args().First()
	eventID:=c.Args().Get(1)
	services:=clients.RegionClient.Tenants().Get(tenantID).Services().List()

	for _,service:=range services{
		err:=clients.RegionClient.Tenants().Get(tenantID).Services().Stop(service.ServiceAlias,eventID)
		//err = region.StopService(service["service_id"].(string), service["deploy_version"].(string))
		if err != nil {
			logrus.Error("停止应用失败:" + err.Error())
			return err
		}
	}
	return nil
}


