package server

import (
	"os"
	"sort"
	"github.com/urfave/cli"
	"rainbond/pkg/grctl/cmd"
)

//var App *cli.App=cli.NewApp()
var App *cli.App
func Run() error {
	App.Flags = []cli.Flag {
		cli.StringFlag{
			Name: "config, c",
			Value: "/etc/goodrain/grctl.json",
			Usage: "Load configuration from `FILE`",
		},
	}
	sort.Sort(cli.FlagsByName(App.Flags))
	sort.Sort(cli.CommandsByName(App.Commands))
	App.Commands=cmd.GetCmds()

	return App.Run(os.Args)
}
