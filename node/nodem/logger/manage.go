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
	"github.com/goodrain/rainbond/builder/sources"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/goodrain/rainbond/cmd/node/option"
	// Register grpc event types
	_ "github.com/containerd/containerd/api/events"
)

//RFC3339NanoFixed time format
var RFC3339NanoFixed = "2006-01-02T15:04:05.000000000Z07:00"

//ContainerLogManage container log manage
type ContainerLogManage struct {
	ctx           context.Context
	cancel        context.CancelFunc
	conf          *option.Conf
	cchan         chan sources.ContainerEvent
	containerLogs sync.Map
}

//CreatContainerLogManage create a container log manage
func CreatContainerLogManage(conf *option.Conf) *ContainerLogManage {
	ctx, cancel := context.WithCancel(context.Background())
	return &ContainerLogManage{
		ctx:    ctx,
		cancel: cancel,
		conf:   conf,
		cchan:  make(chan sources.ContainerEvent, 100),
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

func (c *ContainerLogManage) handleLogger() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case cevent := <-c.cchan:
			switch cevent.Action {
			case sources.CONTAINER_ACTION_START, sources.CONTAINER_ACTION_CREATE:
				if cevent.Container.ContainerRuntime == sources.ContainerRuntimeDocker {
					if cevent.Action == sources.CONTAINER_ACTION_CREATE {
						continue
					}
					loggerType := cevent.Container.HostConfig.LogConfig.Type
					if loggerType != "json-file" && loggerType != "syslog" {
						continue
					}
				}
				if logger, ok := c.containerLogs.Load(cevent.Container.GetId()); ok {
					clog, okf := logger.(*ContainerLog)
					if okf {
						clog.Restart()
						logrus.Infof("restart copy container log for container %s", cevent.Container.GetMetadata().GetName())
					}
				} else {
					go func() {
						retry := 0
						for retry < maxJSONDecodeRetry {
							retry++
							var reader *LogFile
							if cevent.Container.GetLogPath() != "" {
								var err error
								reader, err = NewLogFile(cevent.Container.GetLogPath(), 2, false, decodeFunc, 0640, getTailReader)
								if err != nil {
									logrus.Errorf("create logger failure %s", err.Error())
									time.Sleep(time.Second * 1)
									continue
								}
							} else {
								time.Sleep(time.Second * 1)
								//retry get container inspect info
								cevent.Container, _ = c.getContainer(cevent.Container.GetId())
								continue
							}
							clog := createContainerLog(c.ctx, cevent.Container, reader, c.conf)
							if err := clog.StartLogging(); err != nil {
								clog.Stop()
								if err == ErrNeglectedContainer {
									return
								}
								logrus.Errorf("start copy docker log failure %s", err.Error())
								time.Sleep(time.Second * 1)
								//retry get container inspect info
								cevent.Container, _ = c.getContainer(cevent.Container.GetId())
								continue
							}
							c.containerLogs.Store(cevent.Container.GetId(), clog)
							logrus.Infof("start copy container log for container %s", cevent.Container.GetMetadata().GetName())
							return
						}
					}()
				}
			case sources.CONTAINER_ACTION_STOP, sources.CONTAINER_ACTION_DESTROY, sources.CONTAINER_ACTION_DIE:
				if cevent.Container.ContainerRuntime == sources.ContainerRuntimeDocker && cevent.Action != sources.CONTAINER_ACTION_STOP {
					continue
				}
				if logger, ok := c.containerLogs.Load(cevent.Container.GetId()); ok {
					clog, okf := logger.(*ContainerLog)
					if okf {
						clog.Stop()
					}
					c.containerLogs.Delete(cevent.Container.GetId())
					logrus.Infof("remove copy container log for container %s", cevent.Container.GetMetadata().GetName())
				}
			}
		}
	}
}

func (c *ContainerLogManage) cacheContainer(cs ...sources.ContainerEvent) {
	for _, container := range cs {
		logrus.Debugf("found a container %s %s", container.Container.GetMetadata().GetName(), container.Action)
		c.cchan <- container
	}
}

func (c *ContainerLogManage) listContainer() []*runtimeapi.Container {
	containers, err := c.conf.ContainerImageCli.ListContainers()
	if err != nil {
		logrus.Errorf("list containers failure.%s", err.Error())
		containers, _ = c.conf.ContainerImageCli.ListContainers()
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
				cj, _ := c.getContainer(container.GetId())
				if cj.GetLogPath() == "" {
					continue
				}
				if cj.ContainerRuntime == sources.ContainerRuntimeDocker {
					loggerType := cj.ContainerJSON.HostConfig.LogConfig.Type
					if loggerType != "json-file" && loggerType != "syslog" {
						continue
					}
				}
				if _, exist := c.containerLogs.Load(container.GetId()); !exist {
					c.cacheContainer(sources.ContainerEvent{Action: sources.CONTAINER_ACTION_START, Container: cj})
				}
			}
		}
	}
}

func (c *ContainerLogManage) listAndWatchContainer(errchan chan error) {
	containers := c.listContainer()
	logrus.Infof("found %d containers", len(containers))
	for _, con := range containers {
		container, err := c.getContainer(con.GetId())
		if err != nil {
			if !strings.Contains(err.Error(), "No such container") {
				logrus.Errorf("get container detail info failure %s", err.Error())
			}
			// The log path cannot be obtained if the container details cannot be obtained
			continue
		}
		logrus.Debugf("found a container %s ", container.GetMetadata().GetName())
		c.cacheContainer(sources.ContainerEvent{Action: sources.CONTAINER_ACTION_START, Container: container})
	}
	logrus.Info("list containers complete, start watch container")
	for {
		if err := c.watchContainer(); err != nil {
			logrus.Errorf("watch container error %s, will retry", err.Error())
		}
		time.Sleep(time.Second * 1)
	}
}

//Out -
type Out struct {
	Timestamp time.Time
	Namespace string
	Topic     string
	Event     string
}

func (c *ContainerLogManage) watchContainer() error {
	return c.conf.ContainerImageCli.WatchContainers(c.ctx, c.cchan)
}

func (c *ContainerLogManage) getContainer(containerID string) (*sources.ContainerDesc, error) {
	return c.conf.ContainerImageCli.InspectContainer(containerID)
}

func createContainerLog(ctx context.Context, container *sources.ContainerDesc, reader *LogFile, conf *option.Conf) *ContainerLog {
	cctx, cancel := context.WithCancel(ctx)
	return &ContainerLog{
		ctx:           cctx,
		cancel:        cancel,
		ContainerDesc: container,
		reader:        reader,
		conf:          conf,
	}
}

//ContainerLog container log struct
type ContainerLog struct {
	ctx    context.Context
	cancel context.CancelFunc
	conf   *option.Conf
	*sources.ContainerDesc
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
			logrus.Debugf("find a container %s that do not define rainbond logger.", container.ContainerStatus.GetMetadata().GetName())
			return ErrNeglectedContainer
		}
		return fmt.Errorf("failed to initialize logging driver: %v", err)
	}
	runtimeClient, _ := container.conf.ContainerImageCli.GetRuntimeClient()
	copier := NewCopier(container.reader, loggers, container.since, container.GetId(), runtimeClient)
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

// CRI Interface does not currently support obtaining container environment variables
// Therefore, obtaining log-driven configuration from environment variables is not supported for the time being.
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
			_ = json.Unmarshal([]byte(config), &options)
			configs[i].Options = options
		}
		re = append(re, configs[i])
	}
	return re
}

//ErrNeglectedContainer not define logger name
var ErrNeglectedContainer = fmt.Errorf("Neglected container")

// containerInfo is extra info returned by containerd grpc api
// it's NOT part of cri-api, so we keep this struct being internal visibility.
// If we don't care sth details, we will keep it being interface type.
type containerInfo struct {
	Sandboxid      string                    `json:"sandboxID"`
	Pid            int                       `json:"pid"`
	Removing       bool                      `json:"removing"`
	Snapshotkey    string                    `json:"snapshotKey"`
	Snapshotter    string                    `json:"snapshotter"`
	Runtimetype    string                    `json:"runtimeType"`
	Runtimeoptions interface{}               `json:"runtimeOptions"`
	Config         *ContainerInfoConfig      `json:"config"`
	Runtimespec    *ContainerInfoRuntimeSpec `json:"runtimeSpec"`
}

// ContainerInfoMetadata ...
type ContainerInfoMetadata struct {
	Name string `json:"name"`
}

// ContainerInfoEnv ...
type ContainerInfoEnv struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ContainerInfoMount ...
type ContainerInfoMount struct {
	ContainerPath  string `json:"container_path"`
	HostPath       string `json:"host_path"`
	Readonly       bool   `json:"readonly,omitempty"`
	SelinuxRelabel bool   `json:"selinux_relabel,omitempty"`
}

// ContainerInfoConfig ...
type ContainerInfoConfig struct {
	Metadata    *ContainerInfoMetadata `json:"metadata"`
	Image       interface{}            `json:"image"`
	Envs        []*ContainerInfoEnv    `json:"envs"`
	Mounts      []*ContainerInfoMount  `json:"mounts"`
	Labels      interface{}            `json:"labels"`
	Annotations interface{}            `json:"annotations"`
	LogPath     string                 `json:"log_path"`
	Linux       interface{}            `json:"linux,omitempty"`
}

// ContainerInfoRuntimeMount ...
type ContainerInfoRuntimeMount struct {
	Destination string   `json:"destination"`
	Type        string   `json:"type"`
	Source      string   `json:"source"`
	Options     []string `json:"options"`
}

// ContainerInfoRuntimeSpec ...
type ContainerInfoRuntimeSpec struct {
	Ociversion  string                       `json:"ociVersion"`
	Process     interface{}                  `json:"process"`
	Root        interface{}                  `json:"root"`
	Mounts      []*ContainerInfoRuntimeMount `json:"mounts"`
	Annotations interface{}                  `json:"annotations"`
	Linux       interface{}                  `json:"linux,omitempty"`
}

// ContainerEnv is container env
type ContainerEnv struct {
	Key   string
	Value string
}

func (container *ContainerLog) provideLoggerInfo() (*Info, error) {
	if container.ContainerRuntime == sources.ContainerRuntimeDocker {
		return container.provideDockerdLoggerInfo()
	}
	return container.provideContainerdLoggerInfo()
}

func (container *ContainerLog) provideDockerdLoggerInfo() (*Info, error) {
	createTime, _ := time.Parse(RFC3339NanoFixed, container.Created)
	containerJSON := container.ContainerJSON
	return &Info{
		ContainerID:         containerJSON.ID,
		ContainerName:       containerJSON.Name,
		ContainerEntrypoint: containerJSON.Path,
		ContainerArgs:       containerJSON.Args,
		ContainerImageName:  containerJSON.Config.Image,
		ContainerCreated:    createTime,
		ContainerEnv:        containerJSON.Config.Env,
		ContainerLabels:     containerJSON.Config.Labels,
		DaemonName:          "docker",
	}, nil
}

func (container *ContainerLog) provideContainerdLoggerInfo() (*Info, error) {
	logrus.Debugf("container %s status: %v [%v]", container.ContainerStatus.GetMetadata().GetName(), *container.ContainerStatus, container.Info)
	// NOTE: unmarshal the extra info to get the container envs and mounts data.
	// Mounts should include both image volume and container mount.
	extraContainerInfo := new(containerInfo)
	err := json.Unmarshal([]byte(container.Info["info"]), extraContainerInfo)
	if err != nil {
		logrus.Warnf("failed to unmarshal container info: %v", err)
	}
	var containerEnvs []string
	if extraContainerInfo.Config != nil {
		for _, ce := range extraContainerInfo.Config.Envs {
			logrus.Infof("container [%s] env: %s=%s", container.ContainerStatus.GetId(), ce.Key, ce.Value)
			containerEnvs = append(containerEnvs, fmt.Sprintf("%s=%s", ce.Key, ce.Value))
		}
	}
	createTime := time.Unix(container.ContainerStatus.GetCreatedAt(), 0)
	return &Info{
		ContainerID:        container.ContainerStatus.GetId(),
		ContainerName:      container.ContainerStatus.GetMetadata().GetName(),
		ContainerImageName: container.ContainerStatus.GetImageRef(),
		ContainerCreated:   createTime,
		ContainerEnv:       containerEnvs,
		ContainerLabels:    container.ContainerStatus.GetLabels(),
		DaemonName:         "containerd",
	}, nil
}

// startLogger starts a new logger driver for the container.
func (container *ContainerLog) startLogger() ([]Logger, error) {
	info, err := container.provideLoggerInfo()
	if err != nil {
		return nil, err
	}
	configs := getLoggerConfig(info.ContainerEnv)
	var loggers []Logger
	for _, config := range configs {
		initDriver, err :=
			GetLogDriver(config.Name)
		logrus.Infof("get log driver %s", config.Name)
		if err != nil {
			logrus.Warnf("get container log driver failure %s", err.Error())
			continue
		}
		info.Config = config.Options
		l, err := initDriver(*info)
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
	if container == nil {
		return
	}
	if *container.stoped {
		runtimeClient, _ := container.conf.ContainerImageCli.GetRuntimeClient()
		copier := NewCopier(container.reader, container.LogDriver, container.since, container.GetId(), runtimeClient)
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
	logrus.Debugf("rainbond logger stop for container %s", container.ContainerStatus.GetMetadata().GetName())
}

//Close close
func (container *ContainerLog) Close() {
	if container.LogCopier != nil {
		container.LogCopier.Close()
	}
	container.cancel()
	logrus.Debugf("rainbond logger close for container %s", container.ContainerStatus.GetMetadata().GetName())
}
