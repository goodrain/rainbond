// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/docker/docker/client"
	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/util/termtables"
	"github.com/urfave/cli"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//NewSourceBuildCmd cmd for source build test
func NewSourceBuildCmd() cli.Command {
	c := cli.Command{
		Subcommands: []cli.Command{
			cli.Command{
				Name:  "list",
				Usage: "Lists the building tasks pod currently being performed",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "namespace,ns",
						Usage: "rainbond default namespace",
						Value: "rbd-system",
					},
				},
				Action: func(ctx *cli.Context) {
					namespace := ctx.String("namespace")
					cmd := exec.Command("kubectl", "get", "pod", "-l", "job=codebuild", "-o", "wide", "-n", namespace)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					cmd.Run()
				},
			},
			cli.Command{
				Name:  "log",
				Usage: "Displays a log of the build task",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "namespace,ns",
						Usage: "rainbond default namespace",
						Value: "rbd-system",
					},
				},
				Action: func(ctx *cli.Context) {
					name := ctx.Args().First()
					if name == "" {
						showError("Please specify the task pod name")
					}

					namespace := ctx.String("namespace")
					cmd := exec.Command("kubectl", "logs", "-f", name, "-n", namespace)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					cmd.Run()
				},
			},
			cli.Command{
				Name:  "maven-setting",
				Usage: "maven setting config file manage",
				Subcommands: []cli.Command{
					cli.Command{
						Name: "list",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "namespace,ns",
								Usage: "rainbond default namespace",
								Value: "rbd-system",
							},
						},
						Usage: "list maven setting config file manage",
						Action: func(ctx *cli.Context) {
							Common(ctx)
							namespace := ctx.String("namespace")
							cms, err := clients.K8SClient.CoreV1().ConfigMaps(namespace).List(context.Background(), metav1.ListOptions{
								LabelSelector: "configtype=mavensetting",
							})
							if err != nil {
								showError(err.Error())
							}
							runtable := termtables.CreateTable()
							runtable.AddHeaders("Name", "CreateTime", "UpdateTime", "Default")
							for _, cm := range cms.Items {
								var updateTime = "-"
								if cm.Annotations != nil {
									updateTime = cm.Annotations["updateTime"]
								}
								var def bool
								if cm.Labels["default"] == "true" {
									def = true
								}
								runtable.AddRow(cm.Name, cm.CreationTimestamp.Format(time.RFC3339), updateTime, def)
							}
							fmt.Println(runtable.Render())
						},
					},
					cli.Command{
						Name: "get",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "namespace,ns",
								Usage: "rainbond default namespace",
								Value: "rbd-system",
							},
						},
						Usage: "get maven setting config file manage",
						Action: func(ctx *cli.Context) {
							Common(ctx)
							name := ctx.Args().First()
							if name == "" {
								showError("Please specify the task pod name")
							}
							namespace := ctx.String("namespace")
							cm, err := clients.K8SClient.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, metav1.GetOptions{})
							if err != nil {
								showError(err.Error())
							}
							fmt.Println(cm.Data["mavensetting"])
						},
					},
					cli.Command{
						Name:  "update",
						Usage: "update maven setting config file manage",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "file,f",
								Usage: "define maven setting file",
								Value: "./setting.xml",
							},
							cli.StringFlag{
								Name:  "namespace,ns",
								Usage: "rainbond default namespace",
								Value: "rbd-system",
							},
						},
						Action: func(ctx *cli.Context) {
							Common(ctx)
							name := ctx.Args().First()
							if name == "" {
								showError("Please specify the task pod name")
							}
							namespace := ctx.String("namespace")
							cm, err := clients.K8SClient.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, metav1.GetOptions{})
							if err != nil {
								showError(err.Error())
							}
							body, err := ioutil.ReadFile(ctx.String("f"))
							if err != nil {
								showError(err.Error())
							}
							if cm.Data == nil {
								cm.Data = make(map[string]string)
							}
							if cm.Annotations == nil {
								cm.Annotations = make(map[string]string)
							}
							cm.Data["mavensetting"] = string(body)
							cm.Annotations["updateTime"] = time.Now().Format(time.RFC3339)
							_, err = clients.K8SClient.CoreV1().ConfigMaps(namespace).Update(context.Background(), cm, metav1.UpdateOptions{})
							if err != nil {
								showError(err.Error())
							}
							fmt.Println("Update Success")
						},
					},
					cli.Command{
						Name:  "add",
						Usage: "add maven setting config file manage",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "file,f",
								Usage: "define maven setting file",
								Value: "./setting.xml",
							},
							cli.BoolFlag{
								Name:  "default,d",
								Usage: "default maven setting file",
							},
							cli.StringFlag{
								Name:  "namespace,ns",
								Usage: "rainbond default namespace",
								Value: "rbd-system",
							},
						},
						Action: func(ctx *cli.Context) {
							Common(ctx)
							name := ctx.Args().First()
							if name == "" {
								showError("Please specify the task pod name")
							}
							namespace := ctx.String("namespace")
							body, err := ioutil.ReadFile(ctx.String("f"))
							if err != nil {
								showError(err.Error())
							}
							config := &corev1.ConfigMap{}
							config.Name = name
							config.Namespace = namespace
							config.Labels = map[string]string{
								"creator":    "Rainbond",
								"configtype": "mavensetting",
								"laguage":    code.JavaMaven.String(),
							}
							if ctx.Bool("default") {
								config.Labels["default"] = "true"
							}
							config.Annotations = map[string]string{
								"updateTime": time.Now().Format(time.RFC3339),
							}
							config.Data = map[string]string{
								"mavensetting": string(body),
							}
							_, err = clients.K8SClient.CoreV1().ConfigMaps(namespace).Create(context.Background(), config, metav1.CreateOptions{})
							if err != nil {
								showError(err.Error())
							}
							fmt.Println("Add Success")
						},
					},
					cli.Command{
						Name:  "delete",
						Usage: "delete maven setting config file manage",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "namespace,ns",
								Usage: "rainbond default namespace",
								Value: "rbd-system",
							},
						},
						Action: func(ctx *cli.Context) {
							Common(ctx)
							name := ctx.Args().First()
							if name == "" {
								showError("Please specify the task pod name")
							}
							namespace := ctx.String("namespace")
							err := clients.K8SClient.CoreV1().ConfigMaps(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
							if err != nil {
								showError(err.Error())
							}
							fmt.Println("Delete Success")
						},
					},
				},
			},
		},
		Name:  "build",
		Usage: "Commands related to building source code",
	}
	return c
}

func getLang(dir string) (string, error) {
	lang, err := code.GetLangType(dir)
	if err != nil {
		return "", err
	}
	return lang.String(), nil
}

func getSourceCodeTarFile(dir string) (*os.File, error) {
	util.CheckAndCreateDir("/tmp/.grctl/")
	var cmd []string
	cmd = append(cmd, "tar", "-cf", "/tmp/.grctl/sourcebuild.tar", "--exclude=.svn", "--exclude=.git", "./")
	source := exec.Command(cmd[0], cmd[1:]...)
	source.Dir = dir
	if err := source.Run(); err != nil {
		return nil, err
	}
	return os.OpenFile("/tmp/.grctl/sourcebuild.tar", os.O_RDONLY, 0755)
}

func clear() {
	os.RemoveAll("/tmp/.grctl/sourcebuild.tar")
}

func createDockerCli() *client.Client {
	cli, err := client.NewEnvClient()
	if err != nil {
		fatal("docker client create failure:"+err.Error(), 1)
	}
	return cli
}

func prepare(dir string) {
	util.CheckAndCreateDir(path.Join(dir, ".cache"))
	util.CheckAndCreateDir(path.Join(dir, ".release"))
	os.Chown(path.Join(dir, ".cache"), 200, 200)
	os.Chown(path.Join(dir, ".release"), 200, 200)
}
