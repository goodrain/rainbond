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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/goodrain/rainbond/cmd/monitor/option"
	"github.com/goodrain/rainbond/discover"
	etcdutil "github.com/goodrain/rainbond/util/etcd"
	"github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

const (
	// STARTING starting
	STARTING = iota
	// STARTED started
	STARTED
	//STOPPED stoped
	STOPPED
)

// Manager manage struct
type Manager struct {
	cancel        context.CancelFunc
	ctx           context.Context
	Opt           *option.Config
	generatedConf []byte
	Config        *Config
	Process       *os.Process
	Status        int
	Registry      *discover.KeepAlive
	httpClient    *http.Client
	l             *sync.Mutex
	a             *AlertingRulesManager
}

// NewManager new manager
func NewManager(config *option.Config, a *AlertingRulesManager) *Manager {
	client := &http.Client{
		Timeout: time.Second * 3,
	}

	etcdClientArgs := &etcdutil.ClientArgs{
		Endpoints: config.EtcdEndpoints,
		CaFile:    config.EtcdCaFile,
		CertFile:  config.EtcdCertFile,
		KeyFile:   config.EtcdKeyFile,
	}
	reg, err := discover.CreateKeepAlive(etcdClientArgs, "prometheus", config.BindIP, config.BindIP, config.Port)
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
			RuleFiles: []string{config.AlertingRulesFile, "/etc/prometheus/rules/*.yml"},
			AlertingConfig: AlertingConfig{
				AlertmanagerConfigs: []*AlertmanagerConfig{},
			},
		},
		Registry:   reg,
		httpClient: client,
		l:          &sync.Mutex{},
		a:          a,
	}

	m.LoadConfig()
	if len(config.AlertManagerURL) > 0 {
		al := &AlertmanagerConfig{
			ServiceDiscoveryConfig: ServiceDiscoveryConfig{
				StaticConfigs: []*Group{
					{
						Targets: config.AlertManagerURL,
					},
				},
			},
		}
		m.Config.AlertingConfig.AlertmanagerConfigs = append(m.Config.AlertingConfig.AlertmanagerConfigs, al)
	}
	m.SaveConfig()
	m.a.InitRulesConfig()
	return m
}

// StartDaemon start prometheus daemon
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

// StopDaemon stop daemon
func (p *Manager) StopDaemon() {
	if p.Status != STOPPED {
		logrus.Info("Stopping prometheus daemon ...")
		p.Process.Signal(syscall.SIGTERM)
		p.Process.Wait()
		logrus.Info("Stopped prometheus daemon.")
	}
}

//ReloadConfig reload prometheus config
func (p *Manager) ReloadConfig() error {
	if p.Status == STARTED {
		logrus.Debug("Restart daemon for prometheus.")
		if err := p.Process.Signal(syscall.SIGHUP); err != nil {
			logrus.Error("Failed to restart daemon for prometheus: ", err)
			return err
		}
	}
	return nil
}

//LoadConfig load config
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

// SaveConfig save config
func (p *Manager) SaveConfig() error {
	logrus.Debug("Save prometheus config file.")
	currentConf, err := yaml.Marshal(p.Config)
	if err != nil {
		logrus.Error("Marshal prometheus config to yaml error.", err.Error())
		return err
	}
	if bytes.Equal(currentConf, p.generatedConf) {
		logrus.Debug("updating Prometheus configuration skipped, no configuration change")
		return nil
	}
	err = ioutil.WriteFile(p.Opt.ConfigFile, currentConf, 0644)
	if err != nil {
		logrus.Error("Write prometheus config file error.", err.Error())
		return err
	}
	if err := p.ReloadConfig(); err != nil {
		return err
	}
	p.generatedConf = currentConf
	logrus.Info("reload prometheus config success")
	return nil
}

// UpdateScrape update scrape
func (p *Manager) UpdateScrape(scrapes ...*ScrapeConfig) {
	p.l.Lock()
	defer p.l.Unlock()
	for _, scrape := range scrapes {
		logrus.Debugf("update scrape: %+v", scrape)
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
	}
	if err := p.SaveConfig(); err != nil {
		logrus.Errorf("save prometheus config failure:%s", err.Error())
	}
}

// UpdateAndRemoveScrape update and remove scrape
func (p *Manager) UpdateAndRemoveScrape(remove []*ScrapeConfig, scrapes ...*ScrapeConfig) {
	p.l.Lock()
	defer p.l.Unlock()
	for _, scrape := range scrapes {
		logrus.Debugf("update scrape: %+v", scrape)
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
	}
	for _, rm := range remove {
		for i, s := range p.Config.ScrapeConfigs {
			if s.JobName == rm.JobName {
				logrus.Infof("remove scrape %s", rm.JobName)
				p.Config.ScrapeConfigs = append(p.Config.ScrapeConfigs[0:i], p.Config.ScrapeConfigs[i+1:]...)
				break
			}
		}
	}
	if err := p.SaveConfig(); err != nil {
		logrus.Errorf("save prometheus config failure:%s", err.Error())
	}
}
