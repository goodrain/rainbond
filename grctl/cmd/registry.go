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
	"context"
	"fmt"
	"github.com/goodrain/rainbond-operator/util/constants"
	"github.com/goodrain/rainbond/monitor/utils"

	rainbondv1alpha1 "github.com/goodrain/rainbond-operator/api/v1alpha1"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/goodrain/rainbond/grctl/registry"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// NewCmdRegistry registry cmd
func NewCmdRegistry() cli.Command {
	c := cli.Command{
		Name:  "registry",
		Usage: "grctl registry [command]",
		Subcommands: []cli.Command{
			{
				Name: "cleanup",
				Usage: `Clean up free images in the registry.
	The command 'grctl registry cleanup' will delete the index of free images in registry.
	Then you have to exec the command below to remove blobs from the filesystem:
		bin/registry garbage-collect [--dry-run] /path/to/config.yml
	More Detail: https://docs.docker.com/registry/garbage-collection/#run-garbage-collection.
				`,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:   "namespace, ns",
						Usage:  "rainbond namespace",
						EnvVar: "RBDNamespace",
						Value:  utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace),
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)

					namespace := c.String("namespace")
					var cluster rainbondv1alpha1.RainbondCluster
					if err := clients.RainbondKubeClient.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: "rainbondcluster"}, &cluster); err != nil {
						return errors.Wrap(err, "get configuration from rainbond cluster")
					}

					dsn, err := databaseDSN(&cluster)
					if err != nil {
						return errors.Wrap(err, "get database dsn")
					}

					dbCfg := config.Config{
						MysqlConnectionInfo: dsn,
						DBType:              "mysql",
					}
					if err := db.CreateManager(dbCfg); err != nil {
						return errors.Wrap(err, "create database manager")
					}

					registryConfig := cluster.Spec.ImageHub
					cleaner, err := registry.NewRegistryCleaner(registryConfig.Domain, registryConfig.Username, registryConfig.Password)
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

func databaseDSN(rainbondcluster *rainbondv1alpha1.RainbondCluster) (string, error) {
	database := rainbondcluster.Spec.RegionDatabase
	if database != nil {
		return fmt.Sprintf("%s:%s@tcp(%s)/%s", database.Username, database.Password, database.Host, database.Name), nil
	}
	// default name of rbd-db pod is rbd-db-0
	pod, err := clients.K8SClient.CoreV1().Pods(rainbondcluster.Namespace).Get(context.Background(), "rbd-db-0", metav1.GetOptions{})
	if err != nil {
		return "", errors.Wrap(err, "get pod rbd-db-0")
	}
	host := pod.Status.PodIP
	name := "region"
	for _, ct := range pod.Spec.Containers {
		if ct.Name != "rbd-db" {
			continue
		}
		for _, env := range ct.Env {
			if env.Name == "MYSQL_DATABASE" {
				name = env.Value
			}
		}
	}

	secret, err := clients.K8SClient.CoreV1().Secrets(rainbondcluster.Namespace).Get(context.Background(), "rbd-db", metav1.GetOptions{})
	if err != nil {
		return "", errors.Wrap(err, "get secret rbd-db")
	}
	username := string(secret.Data["mysql-user"])
	password := string(secret.Data["mysql-password"])

	return fmt.Sprintf("%s:%s@tcp(%s:3306)/%s", username, password, host, name), nil
}
