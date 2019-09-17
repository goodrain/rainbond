package cmd

import (
	"fmt"

	licutil "github.com/goodrain/rainbond/util/license"
	"github.com/gosuri/uitable"
	"github.com/urfave/cli"
)

//NewCmdLicense -
func NewCmdLicense() cli.Command {
	c := cli.Command{
		Name:  "license",
		Usage: "rainbond license manage cmd",
		Subcommands: []cli.Command{
			{
				Name:  "show",
				Usage: "show license information",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "lic-path, lp",
						Usage: "license file path",
						Value: "/opt/rainbond/etc/license/license.yb",
					},
					cli.StringFlag{
						Name:  "lic-so-path, lsp",
						Usage: "license.so file path",
						Value: "/opt/rainbond/etc/license/license.so",
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)
					licPath := c.String("lic-path")
					licSoPath := c.String("lic-so-path")
					licInfo, err := licutil.GetLicInfo(licPath, licSoPath)
					if err != nil {
						showError(err.Error())
					}

					if licInfo == nil {
						fmt.Println("non-enterprise version, no license information")
						return nil
					}

					table := uitable.New()
					table.AddRow("授权公司名称:", licInfo.Company)
					table.AddRow("授权公司代码:", licInfo.Code)
					table.AddRow("授权单数据中心节点数:", licInfo.Node)
					table.AddRow("授权开始时间:", licInfo.StartTime)
					table.AddRow("授权到期时间:", licInfo.EndTime)
					table.AddRow("授权key:", licInfo.LicKey)
					fmt.Println(table)
					return nil
				},
			},
			{
				Name:  "genkey",
				Usage: "generate a license key for the machine",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "lic-so-path, lsp",
						Usage: "license.so file path",
						Value: "/opt/rainbond/etc/license/license.so",
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)
					licSoPath := c.String("lic-so-path")
					licKey, err := licutil.GenLicKey(licSoPath)
					if err != nil {
						showError(err.Error())
					}

					if licKey == "" {
						fmt.Println("non-enterprise version, no license key")
						return nil
					}
					fmt.Println(licKey)
					return nil
				},
			},
		},
	}
	return c
}
