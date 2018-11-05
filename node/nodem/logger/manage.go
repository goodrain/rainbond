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

package logger

import (
	"fmt"
	"io"

	"github.com/docker/engine-api/types"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/moby/moby/daemon/logger/jsonfilelog"
)

type ContainerLogManage struct {
	conf *option.Conf
}

//CreatContainerLogManage create a container log manage
func CreatContainerLogManage(conf *option.Conf) {

}

//Start start
func (*ContainerLogManage) Start() error {

}

//ContainerLog container log struct
type ContainerLog struct {
	types.Container
}

func (container *ContainerLog) startLogging() error {
	l, err := container.StartLogger()
	if err != nil {
		return fmt.Errorf("failed to initialize logging driver: %v", err)
	}
	copier := NewCopier(map[string]io.Reader{"stdout": container.StdoutPipe(), "stderr": container.StderrPipe()}, l)
	container.LogCopier = copier
	copier.Run()
	container.LogDriver = l
	// set LogPath field only for json-file logdriver
	if jl, ok := l.(*jsonfilelog.JSONFileLogger); ok {
		container.LogPath = jl.LogPath()
	}
	return nil
}

// StartLogger starts a new logger driver for the container.
func (container *ContainerLog) StartLogger() (Logger, error) {
	cfg := container.HostConfig.LogConfig
	initDriver, err := GetLogDriver(cfg.Type)
	if err != nil {
		return nil, fmt.Errorf("failed to get logging factory: %v", err)
	}
	info := Info{
		Config:              cfg.Config,
		ContainerID:         container.ID,
		ContainerName:       container.Name,
		ContainerEntrypoint: container.Path,
		ContainerArgs:       container.Args,
		ContainerImageID:    container.ImageID.String(),
		ContainerImageName:  container.Config.Image,
		ContainerCreated:    container.Created,
		ContainerEnv:        container.Config.Env,
		ContainerLabels:     container.Config.Labels,
		DaemonName:          "docker",
	}
	l, err := initDriver(info)
	if err != nil {
		return nil, err
	}
	return l, nil
}
