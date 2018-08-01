package cmd

import (
	"github.com/urfave/cli"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/grctl/clients"
	"fmt"
	"github.com/ghodss/yaml"
	"encoding/json"
	"github.com/goodrain/rainbond/node/api/model"
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
					v, err := clients.RegionClient.Monitor().GetRule(name)
					handleErr(err)
					rule, _ := yaml.Marshal(v)
					fmt.Println(string(rule))
					return nil
				},
			},
			{
				Name:  "list",
				Usage: "list",
				Action: func(c *cli.Context) error {
					Common(c)
					list, err := clients.RegionClient.Monitor().GetAllRule()
					handleErr(err)
					ruleList, _ := yaml.Marshal(list)
					fmt.Println(string(ruleList))
					return nil
				},
			},
			{
				Name:  "del",
				Usage: "del rule_name",
				Action: func(c *cli.Context) error {
					Common(c)
					name := c.Args().First()
					if name == "" {
						logrus.Errorf("need args")
						return nil
					}
					v, err := clients.RegionClient.Monitor().DelRule(name)
					handleErr(err)
					result, _ := json.Marshal(v.Bean)
					fmt.Println(string(result))
					return nil
				},
			},
			{
				Name:  "add",
				Usage: "add rules",
				Action: func(c *cli.Context) error {
					Common(c)
					rules := c.Args().First()
					if rules == "" {
						logrus.Errorf("need args")
						return nil
					}
					println("====>", rules)
					var rulesConfig model.AlertingNameConfig
					yaml.Unmarshal([]byte(rules), &rulesConfig)
					v, err := clients.RegionClient.Monitor().AddRule(&rulesConfig)
					handleErr(err)
					result, _ := json.Marshal(v.Bean)
					fmt.Println(string(result))
					return nil
				},
			},
		},
	}
	return c
}
