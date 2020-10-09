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
	"strings"
	"time"

	"github.com/docker/docker/client"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/util/termtables"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//NewSourceBuildCmd cmd for source build test
func NewSourceBuildCmd() cli.Command {
	c := cli.Command{
		Subcommands: []cli.Command{
			cli.Command{
				Name:  "test",
				Usage: "build test source code, If it can be build, you can build in rainbond",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "dir",
						Usage: "source code dir,default is current dir.",
						Value: "",
					},
					cli.StringFlag{
						Name:  "lang",
						Usage: "source code lang type, if not specified, will automatic identify",
						Value: "",
					},
					cli.StringFlag{
						Name:  "image",
						Usage: "builder image name",
						Value: builder.BUILDERIMAGENAME,
					},
					cli.StringSliceFlag{
						Name:  "env",
						Usage: "Build the required environment variables",
					},
				},
				Action: build,
			},
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
							cms, err := clients.K8SClient.CoreV1().ConfigMaps(namespace).List(metav1.ListOptions{
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
							cm, err := clients.K8SClient.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
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
							cm, err := clients.K8SClient.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
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
							_, err = clients.K8SClient.CoreV1().ConfigMaps(namespace).Update(cm)
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
							_, err = clients.K8SClient.CoreV1().ConfigMaps(namespace).Create(config)
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
							err := clients.K8SClient.CoreV1().ConfigMaps(namespace).Delete(name, &metav1.DeleteOptions{})
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

func build(c *cli.Context) error {
	dir := c.String("dir")
	if dir == "" {
		dir = util.GetCurrentDir()
	}
	fmt.Printf("Start test build code:%s \n", dir)
	envs := c.StringSlice("env")
	var kvenv []*sources.KeyValue
	for _, e := range envs {
		if strings.Contains(e, "=") {
			info := strings.Split(e, "=")
			kvenv = append(kvenv, &sources.KeyValue{Key: info[0], Value: info[1]})
		}
	}
	lang := c.String("lang")
	if lang == "" {
		var err error
		lang, err = getLang(dir)
		if err != nil {
			fatal("automatic identify failure."+err.Error(), 1)
		}
	}
	prepare(dir)
	kvenv = append(kvenv, &sources.KeyValue{Key: "LANGUAGE", Value: lang})
	containerConfig := &sources.ContainerConfig{
		Metadata: &sources.ContainerMetadata{
			Name: "buildcontainer",
		},
		Image: &sources.ImageSpec{
			Image: c.String("image"),
		},
		Mounts: []*sources.Mount{
			&sources.Mount{
				ContainerPath: "/tmp/cache",
				HostPath:      path.Join(dir, ".cache"),
				Readonly:      false,
			},
			&sources.Mount{
				ContainerPath: "/tmp/slug",
				HostPath:      path.Join(dir, ".release"),
				Readonly:      false,
			},
		},
		Envs:         kvenv,
		Stdin:        true,
		StdinOnce:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		NetworkConfig: &sources.NetworkConfig{
			NetworkMode: "host",
		},
		Args: []string{"local"},
	}
	reader, err := getSourceCodeTarFile(dir)
	if err != nil {
		fatal("tar code failure."+err.Error(), 1)
	}
	defer func() {
		reader.Close()
		clear()
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	containerService := sources.CreateDockerService(ctx, createDockerCli())
	containerID, err := containerService.CreateContainer(containerConfig)
	if err != nil {
		return fmt.Errorf("create builder container error:%s", err.Error())
	}
	closed := make(chan struct{})
	defer close(closed)
	errchan := make(chan error, 1)
	close, err := containerService.AttachContainer(containerID, true, true, true, reader, os.Stdout, os.Stderr, &errchan)
	if err != nil {
		containerService.RemoveContainer(containerID)
		return fmt.Errorf("attach builder container error:%s", err.Error())
	}
	defer close()
	statuschan := containerService.WaitExitOrRemoved(containerID, true)
	//start the container
	if err := containerService.StartContainer(containerID); err != nil {
		containerService.RemoveContainer(containerID)
		return fmt.Errorf("start builder container error:%s", err.Error())
	}
	if err := <-errchan; err != nil {
		logrus.Debugf("Error hijack: %s", err)
	}
	status := <-statuschan
	if status != 0 {
		fatal("build source code error", 1)
	}
	fmt.Println("BUILD SUCCESS")
	return nil
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
