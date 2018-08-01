package cmd

import (
	"github.com/urfave/cli"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/grctl/clients"
	"fmt"
	"github.com/ghodss/yaml"
)



//NewCmdNode NewCmdNode
func NewCmdAlerting() cli.Command {
	c := cli.Command{
		Name:  "alerting",
		Usage: "监控报警。grctl alerting",
		Subcommands: []cli.Command{
			{
				Name:  "get",
				Usage: "get rule_name",
				Action: func(c *cli.Context) error {
					Common(c)
					name := c.Args().First()
					if name == "" {
						logrus.Errorf("need args")
						return nil
					}
					println("======name>",name)
					v, err := clients.RegionClient.Monitor().GetRule(name)
					println("========>", err.Error())
					handleErr(err)
					rule, _ := yaml.Marshal(v)
					//var out bytes.Buffer
					//error := json.Indent(&out, nodeByte, "", "\t")
					//if error != nil {
					//	handleErr(util.CreateAPIHandleError(500, err))
					//}
					fmt.Println(rule)
					return nil
				},
			},
			//{
			//	Name:  "list",
			//	Usage: "list",
			//	Action: func(c *cli.Context) error {
			//		Common(c)
			//		list, err := clients.RegionClient.Nodes().List()
			//		handleErr(err)
			//		serviceTable := termtables.CreateTable()
			//		serviceTable.AddHeaders("Uid", "IP", "HostName", "NodeRole", "NodeMode", "Status", "Alived", "Schedulable", "Ready")
			//		var rest []*client.HostNode
			//		for _, v := range list {
			//			if v.Role.HasRule("manage") {
			//				handleStatus(serviceTable, isNodeReady(v), v)
			//			} else {
			//				rest = append(rest, v)
			//			}
			//		}
			//		if len(rest) > 0 {
			//			serviceTable.AddSeparator()
			//		}
			//		for _, v := range rest {
			//			handleStatus(serviceTable, isNodeReady(v), v)
			//		}
			//		fmt.Println(serviceTable.Render())
			//		return nil
			//	},
			//},

		},
	}
	return c
}
