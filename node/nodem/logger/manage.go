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
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/goodrain/rainbond/cmd/node-proxy/option"
)

//RFC3339NanoFixed time format
var RFC3339NanoFixed = "2006-01-02T15:04:05.000000000Z07:00"

//ContainerLogManage container log manage
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
	go func() {
		if err := <-errchan; err != nil {
			logrus.Errorf(err.Error())
		}
	}()
	go c.handleLogger()
	go c.listAndWatchContainer(errchan)
	go c.loollist()
	logrus.Infof("start container log manage success")
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

func (c *ContainerLogManage) getContainerLogByFile(info types.ContainerJSON) (*LogFile, error) {
	return nil, nil
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
func (c *ContainerLogManage) handleLogger() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case cevent := <-c.cchan:
			switch cevent.Action {
			case "start":
				loggerType := cevent.Container.HostConfig.LogConfig.Type
				if loggerType != "json-file" && loggerType != "syslog" {
					continue
				}
				if logger, ok := c.containerLogs.Load(cevent.Container.ID); ok {
					clog, okf := logger.(*ContainerLog)
					if okf {
						clog.Restart()
						logrus.Infof("restart copy container log for container %s", cevent.Container.Name)
					}
				} else {
					go func() {
						retry := 0
						for retry < maxJSONDecodeRetry {
							retry++
							var reader *LogFile
							if cevent.Container.LogPath != "" {
								var err error
								reader, err = NewLogFile(cevent.Container.LogPath, 2, false, decodeFunc, 0640, getTailReader)
								if err != nil {
									logrus.Errorf("create logger failure %s", err.Error())
									time.Sleep(time.Second * 1)
									continue
								}
							} else {
								time.Sleep(time.Second * 1)
								//retry get container inspect info
								cevent.Container, _ = c.getContainer(cevent.Container.ID)
								continue
							}
							clog := createContainerLog(c.ctx, cevent.Container, reader)
							if err := clog.StartLogging(); err != nil {
								clog.Stop()
								if err == ErrNeglectedContainer {
									return
								}
								logrus.Errorf("start copy docker log failure %s", err.Error())
								time.Sleep(time.Second * 1)
								//retry get container inspect info
								cevent.Container, _ = c.getContainer(cevent.Container.ID)
								continue
							}
							c.containerLogs.Store(cevent.Container.ID, clog)
							logrus.Infof("start copy container log for container %s", cevent.Container.Name)
							return
						}
					}()
				}
			case "die", "destroy":
				if logger, ok := c.containerLogs.Load(cevent.Container.ID); ok {
					clog, okf := logger.(*ContainerLog)
					if okf {
						clog.Stop()
					}
					c.containerLogs.Delete(cevent.Container.ID)
					logrus.Infof("remove copy container log for container %s", cevent.Container.Name)
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
func (c *ContainerLogManage) listContainer() []types.Container {
	lictctx, cancel := context.WithTimeout(c.ctx, time.Second*60)
	defer cancel()
	containers, err := c.conf.DockerCli.ContainerList(lictctx, types.ContainerListOptions{})
	if err != nil {
		logrus.Errorf("list containers failure.%s", err.Error())
		containers, _ = c.conf.DockerCli.ContainerList(lictctx, types.ContainerListOptions{})
	}
	return containers
}

func (c *ContainerLogManage) loollist() {
	ticker := time.NewTicker(time.Minute * 10)
	for {
		select {
		case <-c.ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			for _, container := range c.listContainer() {
				cj, _ := c.getContainer(container.ID)
				if cj.ContainerJSONBase == nil || cj.HostConfig == nil || cj.HostConfig.LogConfig.Type == "" {
					continue
				}
				loggerType := cj.HostConfig.LogConfig.Type
				if loggerType != "json-file" && loggerType != "syslog" {
					continue
				}
				if _, exist := c.containerLogs.Load(container.ID); !exist {
					c.cacheContainer(ContainerEvent{Action: "start", Container: cj})
				}
			}
		}
	}
}

func (c *ContainerLogManage) listAndWatchContainer(errchan chan error) {
	containers := c.listContainer()
	for _, con := range containers {
		container, err := c.getContainer(con.ID)
		if err != nil {
			if !strings.Contains(err.Error(), "No such container") {
				logrus.Errorf("get container detail info failure %s", err.Error())
			}
			// The log path cannot be obtained if the container details cannot be obtained
			continue
		}
		c.cacheContainer(ContainerEvent{Action: "start", Container: container})
	}
	logrus.Info("list containers complete, start watch container")
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
					break
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

var handleAction = []string{"create", "start", "stop", "die", "destroy"}

func checkEventAction(action string) bool {
	for _, enable := range handleAction {
		if enable == action {
			return true
		}
	}
	return false
}

func createContainerLog(ctx context.Context, container types.ContainerJSON, reader *LogFile) *ContainerLog {
	cctx, cancel := context.WithCancel(ctx)
	return &ContainerLog{
		ctx:           cctx,
		cancel:        cancel,
		ContainerJSON: container,
		reader:        reader,
	}
}

//ContainerLog container log struct
type ContainerLog struct {
	ctx    context.Context
	cancel context.CancelFunc
	types.ContainerJSON
	LogCopier *Copier
	LogDriver []Logger
	reader    *LogFile
	since     time.Time
	stoped    *bool
}

//StartLogging start copy log
func (container *ContainerLog) StartLogging() error {
	loggers, err := container.startLogger()
	if err != nil {
		if err == ErrNeglectedContainer {
			logrus.Debugf("find a container %s that do not define rainbond logger.", container.Name)
			return ErrNeglectedContainer
		}
		return fmt.Errorf("failed to initialize logging driver: %v", err)
	}
	copier := NewCopier(container.reader, loggers, container.since)
	container.LogCopier = copier
	copier.Run()
	container.LogDriver = loggers
	return nil
}

//ContainerLoggerConfig logger config
type ContainerLoggerConfig struct {
	Name    string
	Options map[string]string
}

func getLoggerConfig(envs []string) []*ContainerLoggerConfig {
	var configs = make(map[string]*ContainerLoggerConfig)
	var envMap = make(map[string]string, len(envs))
	for _, v := range envs {
		info := strings.SplitN(v, "=", 2)
		if len(info) > 1 {
			envMap[strings.ToLower(info[0])] = info[1]
			if strings.HasPrefix(info[0], "LOGGER_DRIVER_NAME") {
				if _, exist := configs[info[1]]; !exist {
					configs[info[1]] = &ContainerLoggerConfig{
						Name: info[1],
					}
				}
			}
		}
	}
	var re []*ContainerLoggerConfig
	for i, c := range configs {
		if config, ok := envMap[strings.ToLower("LOGGER_DRIVER_OPT_"+c.Name)]; ok {
			var options = make(map[string]string)
			json.Unmarshal([]byte(config), &options)
			configs[i].Options = options
		}
		re = append(re, configs[i])
	}
	return re
}

//ErrNeglectedContainer not define logger name
var ErrNeglectedContainer = fmt.Errorf("Neglected container")

// startLogger starts a new logger driver for the container.
func (container *ContainerLog) startLogger() ([]Logger, error) {
	configs := getLoggerConfig(container.Config.Env)
	var loggers []Logger
	for _, config := range configs {
		initDriver, err :=
			GetLogDriver(config.Name)
		if err != nil {
			logrus.Warnf("get container log driver failure %s", err.Error())
			continue
		}
		createTime, _ := time.Parse(RFC3339NanoFixed, container.Created)
		info := Info{
			Config:              config.Options,
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
			logrus.Warnf("init container log driver failure %s", err.Error())
			continue
		}
		loggers = append(loggers, l)
	}
	if len(loggers) == 0 {
		return nil, ErrNeglectedContainer
	}
	return loggers, nil
}

//Restart restart
func (container *ContainerLog) Restart() {
	if *container.stoped {
		copier := NewCopier(container.reader, container.LogDriver, container.since)
		container.LogCopier = copier
		copier.Run()
	}
}

//Stop stop copy container log
func (container *ContainerLog) Stop() {
	if container.LogCopier != nil {
		container.LogCopier.Close()
	}
	container.since = time.Now()
	var containerLogStop = true
	container.stoped = &containerLogStop
	logrus.Debugf("rainbond logger stop for container %s", container.Name)
}

//Close close
func (container *ContainerLog) Close() {
	if container.LogCopier != nil {
		container.LogCopier.Close()
	}
	container.cancel()
	logrus.Debugf("rainbond logger close for container %s", container.Name)
}
