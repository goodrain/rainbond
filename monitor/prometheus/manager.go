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
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/monitor/option"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"os/exec"
	"sync"
	"time"
	discover3 "github.com/goodrain/rainbond/discover.v2"
	"fmt"
)

type Manager struct {
	ApiUrl     string
	Opt        *option.Config
	Config     *Config
	Reg        *discover3.KeepAlive
	httpClient *http.Client
	l          *sync.Mutex
}

func NewManager(config *option.Config) *Manager {
	client := &http.Client{
		Timeout: time.Second * 3,
	}

	reg, err := discover3.CreateKeepAlive(config.EtcdEndpoints, "prometheus", "http", config.BindIp, config.Port)
	if err != nil {
		panic(err)
	}

	return &Manager{
		ApiUrl:     fmt.Sprintf("http://127.0.0.1:%s", config.Port),
		Opt:        config,
		Config:     &Config{},
		Reg:        reg,
		httpClient: client,
		l:          &sync.Mutex{},
	}
}

func (p *Manager) LoadConfig() error {
	context, err := ioutil.ReadFile(p.Opt.ConfigFile)
	if err != nil {
		logrus.Error("Failed to read prometheus config file: ", err)
		return err
	}

	if err := yaml.Unmarshal(context, p.Config); err != nil {
		logrus.Error("Unmarshal prometheus config string to object error.", err.Error())
		return err
	}

	return nil
}

func (p *Manager) SaveConfig() error {
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

func (p *Manager) StartDaemon(done chan bool) {
	cmd := "which prometheus && " +
		"prometheus " +
		"--web.listen-address=:%s " +
		"--storage.tsdb.path=/prometheusdata " +
		"--storage.tsdb.retention=7d " +
		"--config.file=%s &"

	cmd = fmt.Sprintf(cmd, p.Opt.Port, p.Opt.ConfigFile)

	err := exec.Command("sh", "-c", cmd).Run()
	if err != nil {
		logrus.Error("Can not start prometheus daemon: ", err)
		panic(err)
	}

	p.Reg.Start()
	defer p.Reg.Stop()

	t := time.Tick(time.Second * 5)
	for {
		select {
		case <-done:
			exec.Command("sh", "-c", "kill `pgrep prometheus`").Run()
			return
		case <-t:
			err := exec.Command("sh", "-c", "pgrep prometheus").Run()
			if err != nil {
				logrus.Error("the prometheus process is exited, ready to restart it.")
				err := exec.Command("sh", "-c", cmd).Run()
				if err == nil {
					logrus.Error("Failed to restart the prometheus daemon: ", err)
				}
			}
		}
	}

}

func (p *Manager) RestartDaemon() error {
	request, err := http.NewRequest("POST", p.ApiUrl+"/-/reload", nil)
	if err != nil {
		logrus.Error("Create request to load config error: ", err)
		return err
	}

	_, err = p.httpClient.Do(request)
	if err != nil {
		logrus.Error("load config error: ", err)
		return err
	}

	return nil
}

func (p *Manager) UpdateScrape(scrape *ScrapeConfig) {
	p.l.Lock()
	defer p.l.Unlock()

	for i, s := range p.Config.ScrapeConfigs {
		if s.JobName == scrape.JobName {
			p.Config.ScrapeConfigs[i] = scrape
			break
		}
	}

	p.SaveConfig()
	p.RestartDaemon()
}
