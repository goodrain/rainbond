package cmd

import (
	"github.com/urfave/cli"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/grctl/clients"
	"fmt"
	"github.com/ghodss/yaml"
	"encoding/json"
	"github.com/goodrain/rainbond/node/api/model"
	"errors"
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
				Usage: "add 添加规则",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "Rules,r",
						Value: "",
						Usage: "Rules",
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)
					if c.IsSet("Rules") {
						rules := c.String("Rules")

						println("====>", rules)
						var rulesConfig model.AlertingNameConfig
						yaml.Unmarshal([]byte(rules), &rulesConfig)
						v, err := clients.RegionClient.Monitor().AddRule(&rulesConfig)
						handleErr(err)
						result, _ := json.Marshal(v.Bean)
						fmt.Println(string(result))
						return nil
					}
					return errors.New("rules not null")
				},
			},
			{
				Name:  "modify",
				Usage: "modify 修改规则",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "RulesName,rn",
						Value: "",
						Usage: "RulesName",
					},
					cli.StringFlag{
						Name:  "Rules,r",
						Value: "",
						Usage: "Rules",
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)
					if c.IsSet("RulesName") && c.IsSet("Rules") {
						rules := c.String("Rules")
						ruleName := c.String("RulesName")
						println("====>", rules)
						var rulesConfig model.AlertingNameConfig
						yaml.Unmarshal([]byte(rules), &rulesConfig)
						v, err := clients.RegionClient.Monitor().RegRule(ruleName, &rulesConfig)
						handleErr(err)
						result, _ := json.Marshal(v.Bean)
						fmt.Println(string(result))
						return nil
					}
					return errors.New("rule name or rules not null")
				},
			},
		},
	}
	return c
}
