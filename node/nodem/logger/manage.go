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
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/goodrain/rainbond/cmd/node/option"
)

//RFC3339NanoFixed time format
var RFC3339NanoFixed = "2006-01-02T15:04:05.000000000Z07:00"

//ContainerLogManage conatiner log manage
type ContainerLogManage struct {
	ctx           context.Context
	cancel        context.CancelFunc
	conf          *option.Conf
	cchan         chan ContainerEvent
	containerLogs sync.Map
}

//CreatContainerLogManage create a container log manage
func CreatContainerLogManage(conf *option.Conf) *ContainerLogManage {
	ctx, cancel := context.WithCancel(context.Background())
	return &ContainerLogManage{
		ctx:    ctx,
		cancel: cancel,
		conf:   conf,
		cchan:  make(chan ContainerEvent, 100),
	}
}

//Start start
func (c *ContainerLogManage) Start() error {
	errchan := make(chan error)
	go c.handleLogger(errchan)
	go c.listAndWatchContainer(errchan)
	return nil
}

//Stop stop all logger
func (c *ContainerLogManage) Stop() {
	c.containerLogs.Range(func(k, v interface{}) bool {
		cl := v.(*ContainerLog)
		cl.Stop()
		return true
	})
}
func (c *ContainerLogManage) getContainerLogReader(ctx context.Context, containerID string) (io.ReadCloser, io.ReadCloser, error) {
	stderr, err := c.conf.DockerCli.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
		ShowStderr: true,
		Since:      time.Now().Format(RFC3339NanoFixed),
		Follow:     true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("Open container stderr output failure,%s", err.Error())
	}
	stdout, err := c.conf.DockerCli.ContainerLogs(ctx, containerID, types.ContainerLogsOptions{
		ShowStdout: true,
		Since:      time.Now().Format(RFC3339NanoFixed),
		Follow:     true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("Open container stdout output failure,%s", err.Error())
	}
	return stdout, stderr, nil
}
func (c *ContainerLogManage) handleLogger(errchan chan error) {
	for {
		select {
		case <-c.ctx.Done():
			return
		case cevent := <-c.cchan:
			switch cevent.Action {
			case "create", "start":
				loggerType := cevent.Container.HostConfig.LogConfig.Type
				if loggerType != "json-file" && loggerType != "syslog" {
					continue
				}
				if _, ok := c.containerLogs.Load(cevent.Container.ID); !ok {
					clog := createContainerLog(c.ctx, cevent.Container)
					stdout, stderr, err := c.getContainerLogReader(clog.ctx, cevent.Container.ID)
					if err != nil {
						logrus.Errorf("start container logger failure %s", err.Error())
						continue
					}
					err = clog.startLogging(stdout, stderr)
					if err != nil {
						if err == ErrNeglectedContainer {
							clog.Stop()
							continue
						}
						logrus.Errorf("start container logger failure %s", err.Error())
						continue
					}
					logrus.Infof("start copy container log for container %s", cevent.Container.Name)
					c.containerLogs.Store(cevent.Container.ID, clog)
				}
			case "stop", "die":
				if logger, ok := c.containerLogs.Load(cevent.Container.ID); ok {
					clog, okf := logger.(*ContainerLog)
					if okf {
						clog.Stop()
					}
					c.containerLogs.Delete(cevent.Container.ID)
				}
			}
		}
	}
}

//ContainerEvent container event
type ContainerEvent struct {
	Action    string
	Container types.ContainerJSON
}

func (c *ContainerLogManage) cacheContainer(cs ...ContainerEvent) {
	for _, container := range cs {
		logrus.Debugf("found a container %s %s", container.Container.Name, container.Action)
		c.cchan <- container
	}
}
func (c *ContainerLogManage) listAndWatchContainer(errchan chan error) {
	lictctx, cancel := context.WithTimeout(c.ctx, time.Second*20)
	containers, err := c.conf.DockerCli.ContainerList(lictctx, types.ContainerListOptions{})
	if err != nil {
		cancel()
		errchan <- fmt.Errorf("list containers failure.%s", err.Error())
	}
	cancel()
	for _, con := range containers {
		container, err := c.getContainer(con.ID)
		if err != nil {
			if !strings.Contains(err.Error(), "No such container") {
				logrus.Errorf("get container detail info failure %s", err.Error())
			}
			container.ID = con.ID
		}
		c.cacheContainer(ContainerEvent{Action: "start", Container: container})
	}
	for {
		if err := c.watchContainer(); err != nil {
			logrus.Errorf("watch container error %s, will retry", err.Error())
		}
		time.Sleep(time.Second * 1)
	}
}

func (c *ContainerLogManage) watchContainer() error {
	containerFileter := filters.NewArgs()
	containerFileter.Add("type", "container")
	eventchan, eventerrchan := c.conf.DockerCli.Events(c.ctx, types.EventsOptions{
		Filters: containerFileter,
	})
	for {
		select {
		case <-c.ctx.Done():
			return nil
		case err := <-eventerrchan:
			return err
		case event, ok := <-eventchan:
			if !ok {
				return fmt.Errorf("event chan is closed")
			}
			if event.Type == events.ContainerEventType && checkEventAction(event.Action) {
				container, err := c.getContainer(event.ID)
				if err != nil {
					if !strings.Contains(err.Error(), "No such container") {
						logrus.Errorf("get container detail info failure %s", err.Error())
					}
					container.ID = event.ID
				}
				c.cacheContainer(ContainerEvent{Action: event.Action, Container: container})
			}
		}
	}
}
func (c *ContainerLogManage) getContainer(containerID string) (types.ContainerJSON, error) {
	ctx, cancel := context.WithTimeout(c.ctx, time.Second*5)
	defer cancel()
	return c.conf.DockerCli.ContainerInspect(ctx, containerID)
}

var handleAction = []string{"create", "start", "stop", "die"}

func checkEventAction(action string) bool {
	for _, enable := range handleAction {
		if enable == action {
			return true
		}
	}
	return false
}

func createContainerLog(ctx context.Context, container types.ContainerJSON) *ContainerLog {
	cctx, cancel := context.WithCancel(ctx)
	return &ContainerLog{
		ctx:           cctx,
		cancel:        cancel,
		ContainerJSON: container,
	}
}

//ContainerLog container log struct
type ContainerLog struct {
	ctx    context.Context
	cancel context.CancelFunc
	types.ContainerJSON
	LogCopier      *Copier
	LogDriver      Logger
	stdout, stderr io.ReadCloser
}

func (container *ContainerLog) startLogging(stdout, stderr io.ReadCloser) error {
	container.stdout = stdout
	container.stderr = stderr
	l, err := container.StartLogger()
	if err != nil {
		if err == ErrNeglectedContainer {
			logrus.Debugf("find a container %s that do not define rainbond logger.", container.Name)
			return ErrNeglectedContainer
		}
		return fmt.Errorf("failed to initialize logging driver: %v", err)
	}
	copier := NewCopier(map[string]io.Reader{"stdout": stdout, "stderr": stderr}, l)
	container.LogCopier = copier
	copier.Run()
	container.LogDriver = l
	return nil
}
func getLoggerConfig(envs []string) (string, map[string]string) {
	config := make(map[string]string)
	var name string
	for _, v := range envs {
		if strings.HasPrefix(v, "LOGGER_DRIVER_NAME=") {
			name = v[19:]
		}
		if strings.HasPrefix(v, "LOGGER_DRIVER_OPT_") {
			envmap := strings.SplitN(v, "=", 2)
			config[envmap[0][18:]] = envmap[1]
		}
	}
	return name, config
}

//ErrNeglectedContainer not define logger name
var ErrNeglectedContainer = fmt.Errorf("Neglected container")

// StartLogger starts a new logger driver for the container.
func (container *ContainerLog) StartLogger() (Logger, error) {
	loggerName, config := getLoggerConfig(container.Config.Env)
	if loggerName == "" {
		return nil, ErrNeglectedContainer
	}
	initDriver, err := GetLogDriver(loggerName)
	if err != nil {
		return nil, fmt.Errorf("failed to get logging factory: %v", err)
	}
	createTime, _ := time.Parse(RFC3339NanoFixed, container.Created)
	info := Info{
		Config:              config,
		ContainerID:         container.ID,
		ContainerName:       container.Name,
		ContainerEntrypoint: container.Path,
		ContainerArgs:       container.Args,
		ContainerImageName:  container.Config.Image,
		ContainerCreated:    createTime,
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

//Stop stop copy container log
func (container *ContainerLog) Stop() {
	if container.LogCopier != nil {
		container.LogCopier.Close()
	}
	if container.LogDriver != nil {
		if err := container.LogDriver.Close(); err != nil {
			logrus.Errorf("close log driver failure %s", container.Name)
		}
	}
	if container.stdout != nil {
		container.stdout.Close()
	}
	if container.stderr != nil {
		container.stderr.Close()
	}
	container.cancel()
	logrus.Debugf("rainbond logger stop for container %s", container.Name)
}
