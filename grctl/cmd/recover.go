package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/urfave/cli"
	"io/ioutil"
	"net/http"
)

// NewCmdRecover -
func NewCmdRecover() cli.Command {
	c := cli.Command{
		Name:  "recover",
		Usage: "this command is used to restore the rainbond platform\n",
		Subcommands: []cli.Command{
			{
				Name: "region",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:     "region_name",
						Value:    "",
						Usage:    "use region_name",
						FilePath: GetTenantNamePath(),
						Required: true,
					},
					cli.StringFlag{
						Name:     "recover_range",
						Value:    "",
						Usage:    "recover range [all、component、resource]",
						FilePath: GetTenantNamePath(),
						Required: true,
					},
					cli.StringFlag{
						Name:     "console_host",
						Value:    "",
						Usage:    "use console svc host",
						FilePath: GetTenantNamePath(),
					},
				},
				Usage: "recover region resource. example<grctl recover region --region_name rainbond --range all>",
				Action: func(c *cli.Context) error {
					Common(c)
					return recoverRegion(c)
				},
			},
		},
	}
	return c
}

func recoverRegion(ctx *cli.Context) error {
	regionName := ctx.String("region_name")
	fmt.Println(regionName)
	recoverRange := ctx.String("recover_range")
	fmt.Println(recoverRange)
	consoleHost := ctx.String("console_host")
	fmt.Println(consoleHost)
	recoverUrl := fmt.Sprintf("%v/console/regions_recover", consoleHost)

	requestBody, err := json.Marshal(map[string]string{
		"region_name":   regionName,
		"recover_range": recoverRange,
	})
	if err != nil {
		showError(fmt.Sprintf("failed to marshal request body: %v", err))
	}
	resp, err := http.Post(recoverUrl, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		showError(fmt.Sprintf("failed to make request: %v", err))
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		showError(fmt.Sprintf("failed to read response body: %v", err))
	}
	fmt.Printf("Response Body: %s\n", body)
	return nil
}

type Bean struct {
	ResourceCount  int `json:"resource_count"`
	ComponentCount int `json:"component_count"`
}
