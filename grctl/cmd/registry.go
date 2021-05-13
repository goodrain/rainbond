// Copyright (C) 2014-2021 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package cmd

import (
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/grctl/registry"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// NewCmdRegistry registry cmd
func NewCmdRegistry() cli.Command {
	c := cli.Command{
		Name:  "registry",
		Usage: "grctl registry [command]",
		Subcommands: []cli.Command{
			{
				Name:  "cleanup",
				Usage: `Clean up free images in the registry.
	The command 'grctl registry cleanup' will delete the index of free images in registry.
	Then you have to exec the command below to remove blobs from the filesystem:
		bin/registry garbage-collect [--dry-run] /path/to/config.yml
	More Detail: https://docs.docker.com/registry/garbage-collection/#run-garbage-collection.
				`,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "dsn",
						Usage: "The DSN string to connect the database",
					},
					cli.StringFlag{
						Name:  "url",
						Value: "goodrain.me",
						Usage: "The url of registry",
					},
					cli.StringFlag{
						Name:  "username",
						Value: "admin",
						Usage: "The username of registry",
					},
					cli.StringFlag{
						Name:  "password",
						Usage: "The password of registry",
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)

					dbCfg := config.Config{
						MysqlConnectionInfo: c.String("dsn"),
						DBType:              "mysql",
					}
					if err := db.CreateManager(dbCfg); err != nil {
						return errors.Wrap(err, "create database manager")
					}

					url := c.String("url")
					username := c.String("username")
					password := c.String("password")
					cleaner, err := registry.NewRegistryCleaner(url, username, password)
					if err != nil {
						return errors.WithMessage(err, "create registry cleaner")
					}

					cleaner.Cleanup()

					return nil
				},
			},
		},
	}
	return c
}
