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
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/engine-api/client"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/util"
	"github.com/urfave/cli"
)

//NewSourceBuildCmd cmd for source build test
func NewSourceBuildCmd() cli.Command {
	c := cli.Command{
		Name:  "buildtest",
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
