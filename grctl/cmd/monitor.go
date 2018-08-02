package cmd

import (
	"github.com/urfave/cli"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/grctl/clients"
	"fmt"
	"github.com/ghodss/yaml"
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
					_, err := clients.RegionClient.Monitor().DelRule(name)
					handleErr(err)
					fmt.Println("Delete rule succeeded")
					return nil
				},
			},
			{
				Name:  "add",
				Usage: "add FilePath",
				Action: func(c *cli.Context) error {
					Common(c)
					filePath := c.Args().First()
					if filePath == "" {
						logrus.Errorf("need args")
						return nil
					}
					_, err := clients.RegionClient.Monitor().AddRule(filePath)
					handleErr(err)
					fmt.Println("Add rule successfully")
					return nil

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
						Name:  "RulesPath,rp",
						Value: "",
						Usage: "RulesPath",
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)
					if c.IsSet("RulesName") && c.IsSet("RulesPath") {
						path := c.String("RulesPath")
						ruleName := c.String("RulesName")
						_, err := clients.RegionClient.Monitor().RegRule(ruleName, path)
						handleErr(err)
						fmt.Println("Modify rule successfully")
						return nil
					}
					return errors.New("rule name or rules not null")
				},
			},
		},
	}
	return c
}
