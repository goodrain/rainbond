package cmd
import (
	"github.com/urfave/cli"
	"github.com/Sirupsen/logrus"
	"rainbond/pkg/grctl/clients"
	"fmt"
	"strings"
	"errors"
)


func NewCmdStartService() cli.Command {
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
func startService(c *cli.Context) error  {
	//GET /v2/tenants/{tenant_name}/services/{service_alias}
	//POST /v2/tenants/{tenant_name}/services/{service_alias}/stop

	// goodrain/gra564a1
	serviceAlias := c.Args().First()
	info := strings.Split(serviceAlias, "/")

	eventID:=c.Args().Get(1)
	service:=clients.RegionClient.Tenants().Get(info[0]).Services().Get(info[1])
	if service==nil {
		return errors.New("应用不存在:"+info[1])
	}
	err:=clients.RegionClient.Tenants().Get(info[0]).Services().Start(info[1],eventID)
	//err = region.StopService(service["service_id"].(string), service["deploy_version"].(string))
	if err != nil {
		logrus.Error("停止应用失败:" + err.Error())
		return err
	}
	return nil
}


func stopService(c *cli.Context) error {
	//POST /v1/services/lifecycle/{service_id}/stop/
	//serviceAlias := c.Args().First()
	//info := strings.Split(serviceAlias, "/")
	//if len(info) == 2 {
	//	service, err := db.FindTenantService(info[0], info[1])
	//	if err != nil {
	//		logrus.Error("应用不存在:" + serviceAlias)
	//		return err
	//	}
	//	err = region.StopService(service["service_id"].(string), service["deploy_version"].(string))
	//	if err != nil {
	//		logrus.Error("停止应用失败:" + err.Error())
	//		return err
	//	}
	//} else {
	//	fmt.Println("命令不正确，例如如下格式: grctl stop TenantName/ServiceAlias ")
	//}
	//return nil
	serviceAlias := c.Args().First()
	info := strings.Split(serviceAlias, "/")

	eventID:=c.Args().Get(1)
	service:=clients.RegionClient.Tenants().Get(info[0]).Services().Get(info[1])
	if service==nil {
		return errors.New("应用不存在:"+info[1])
	}
	err:=clients.RegionClient.Tenants().Get(info[0]).Services().Stop(info[1],eventID)
	if err != nil {
		logrus.Error("停止应用失败:" + err.Error())
		return err
	}
	return nil
}