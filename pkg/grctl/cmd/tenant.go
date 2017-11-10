package cmd
import (
	"github.com/urfave/cli"
	"rainbond/pkg/grctl/clients"
	"fmt"
	"github.com/apcera/termtables"
)
func NewCmdTenant() cli.Command {
	c:=cli.Command{
		Name: "tenant",
		Usage: "获取租户应用（包括未运行）信息。 grctl tenant TENANT_NAME",
		Action: func(c *cli.Context) error {
			Common(c)
			return getTenantInfo(c)
		},
	}
	return c
}
func NewCmdTenantRes() cli.Command {
	c:=cli.Command{
		Name:  "tenantres",
		Usage: "获取租户占用资源信息。 grctl tenantres TENANT_NAME",
		Action: func(c *cli.Context) error {
			Common(c)
			return findTenantResourceUsage(c)
		},
	}
	return c
}

// grctrl tenant TENANT_NAME
func getTenantInfo(c *cli.Context) error {
	tenantID := c.Args().First()

	services:=clients.RegionClient.Tenants().Get(tenantID).Services().List()
	//services, err := db.GetServiceInfoByTenant(c.Args().First())
	//if err != nil {
	//	logrus.Error(err.Error())
	//	return err
	//}
	//fmt.Println()
	table := termtables.CreateTable()
	table.AddHeaders("租户ID", "服务ID", "服务别名", "应用状态", "Deploy版本")
	for _, service := range services {
		table.AddRow(service.TenantID, service.ServiceID, service.ServiceAlias, service.CurStatus, service.DeployVersion)
	}
	fmt.Println(table.Render())
	return nil
}
func findTenantResourceUsage(c *cli.Context) error  {
	tenantID := c.Args().First()
	services:=clients.RegionClient.Tenants().Get(tenantID).Services().List()
	//services, err := db.GetServiceInfoByTenant(tenantId)
	//if err != nil {
	//	logrus.Error("租户无应用(资源):" + tenantId)
	//	return err
	//}
	var cpuUsage int64 =0
	var memoryUsage int64=0
	for _,service:=range services{
		cpuUsage+=int64(service.ContainerCPU)
		memoryUsage+=int64(service.ContainerMemory)
	}
	fmt.Printf("租户 %s 占用CPU : %d 核; 占用Memory : %d M",tenantID, cpuUsage,memoryUsage)
	fmt.Println()
	return nil
}