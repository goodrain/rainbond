// Copyright (C) 2014-2018 Goodrain Co., Ltd.
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

package prometheus

import (
	"context"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/monitor/option"
	"github.com/goodrain/rainbond/discover"
	"github.com/prometheus/common/model"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"sync"
	"syscall"
	"time"
	"errors"
)

const (
	STARTING = iota
	STARTED
	STOPPED
)

type Manager struct {
	cancel     context.CancelFunc
	ctx        context.Context
	Opt        *option.Config
	Config     *Config
	Process    *os.Process
	Status     int
	Registry   *discover.KeepAlive
	httpClient *http.Client
	l          *sync.Mutex
	a          *AlertingRulesManager
}

func NewManager(config *option.Config, a *AlertingRulesManager) *Manager {
	client := &http.Client{
		Timeout: time.Second * 3,
	}

	reg, err := discover.CreateKeepAlive(config.EtcdEndpoints, "prometheus", config.BindIp, config.BindIp, config.Port)
	if err != nil {
		panic(err)
	}

	ioutil.WriteFile(config.ConfigFile, []byte(""), 0644)

	m := &Manager{
		Opt: config,
		Config: &Config{
			GlobalConfig: GlobalConfig{
				ScrapeInterval:     model.Duration(time.Second * 5),
				EvaluationInterval: model.Duration(time.Second * 30),
			},
			RuleFiles: []string{config.AlertingRulesFile},
			AlertingConfig:AlertingConfig{
				AlertmanagerConfigs:[]*AlertmanagerConfig{},
			},
		},
		Registry:   reg,
		httpClient: client,
		l:          &sync.Mutex{},
		a:          a,
	}

	m.LoadConfig()
	al := &AlertmanagerConfig{
		ServiceDiscoveryConfig:ServiceDiscoveryConfig{
			StaticConfigs:[]*Group{
				{
					Targets:config.AlertManagerUrl,
				},
			},
		},
	}
	m.Config.AlertingConfig.AlertmanagerConfigs = append(m.Config.AlertingConfig.AlertmanagerConfigs, al)
	m.SaveConfig()
	m.a.InitRulesConfig()

	return m
}

func (p *Manager) StartDaemon(errchan chan error) {
	logrus.Info("Starting prometheus.")

	// start prometheus
	procAttr := &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	}
	process, err := os.StartProcess("/bin/prometheus", p.Opt.StartArgs, procAttr)
	if err != nil {
		if err != nil {
			logrus.Error("Can not start prometheus daemon: ", err)
			os.Exit(11)
		}
	}
	p.Process = process

	// waiting started
	for {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", p.Opt.Port), time.Second)
		if err == nil {
			logrus.Info("The prometheus daemon is started.")
			conn.Close()
			break
		} else {
			logrus.Info("Wait prometheus to start: ", err)
		}
		time.Sleep(time.Second)
	}

	p.Status = STARTED

	// listen prometheus is exit
	go func() {
		_, err := p.Process.Wait()
		logrus.Warn("Exited prometheus unexpectedly.")
		if err == nil {
			err = errors.New("exited prometheus unexpectedly")
		}

		p.Status = STOPPED
		errchan <- err
	}()
}

func (p *Manager) StopDaemon() {
	if p.Status != STOPPED {
		logrus.Info("Stopping prometheus daemon ...")
		p.Process.Signal(syscall.SIGTERM)
		p.Process.Wait()
		logrus.Info("Stopped prometheus daemon.")
	}
}

func (p *Manager) RestartDaemon() error {
	if p.Status == STARTED {
		logrus.Debug("Restart daemon for prometheus.")
		if err := p.Process.Signal(syscall.SIGHUP); err != nil {
			logrus.Error("Failed to restart daemon for prometheus: ", err)
			return err
		}
	}
	return nil
}

func (p *Manager) LoadConfig() error {
	logrus.Info("Load prometheus config file.")
	content, err := ioutil.ReadFile(p.Opt.ConfigFile)
	if err != nil {
		logrus.Error("Failed to read prometheus config file: ", err)
		logrus.Info("Init config file by default values.")
		return nil
	}

	if err := yaml.Unmarshal(content, p.Config); err != nil {
		logrus.Error("Unmarshal prometheus config string to object error.", err.Error())
		return err
	}
	logrus.Debugf("Loaded config file to memory: %+v", p.Config)

	return nil
}

func (p *Manager) SaveConfig() error {
	logrus.Debug("Save prometheus config file.")
	data, err := yaml.Marshal(p.Config)
	if err != nil {
		logrus.Error("Marshal prometheus config to yaml error.", err.Error())
		return err
	}

	err = ioutil.WriteFile(p.Opt.ConfigFile, data, 0644)
	if err != nil {
		logrus.Error("Write prometheus config file error.", err.Error())
		return err
	}

	return nil
}

func (p *Manager) UpdateScrape(scrape *ScrapeConfig) {
	logrus.Debugf("update scrape: %+v", scrape)

	p.l.Lock()
	defer p.l.Unlock()

	exist := false
	for i, s := range p.Config.ScrapeConfigs {
		if s.JobName == scrape.JobName {
			p.Config.ScrapeConfigs[i] = scrape
			exist = true
			break
		}
	}

	if !exist {
		p.Config.ScrapeConfigs = append(p.Config.ScrapeConfigs, scrape)
	}

	p.SaveConfig()
	p.RestartDaemon()
}
