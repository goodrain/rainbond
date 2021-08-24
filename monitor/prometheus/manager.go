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
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ghodss/yaml"
	"github.com/goodrain/rainbond/cmd/monitor/option"
	mv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/rulefmt"
	"github.com/sirupsen/logrus"
	thanostypes "github.com/thanos-io/thanos/pkg/store/storepb"
	cyaml "gopkg.in/yaml.v2"
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
	Opt           *option.Config
	generatedConf []byte
	Config        *Config
	Process       *os.Process
	Status        int
	httpClient    *http.Client
	l             *sync.Mutex
	Rules         []*mv1.PrometheusRule
	ruleContent   map[string][]byte
}

// NewManager new manager
func NewManager(config *option.Config) *Manager {
	client := &http.Client{
		Timeout: time.Second * 3,
	}

	ioutil.WriteFile(config.ConfigFile, []byte(""), 0644)

	m := &Manager{
		Opt: config,
		Config: &Config{
			GlobalConfig: GlobalConfig{
				ScrapeInterval:     model.Duration(time.Second * 5),
				EvaluationInterval: model.Duration(time.Second * 30),
			},
			RuleFiles: []string{},
			AlertingConfig: AlertingConfig{
				AlertmanagerConfigs: []*AlertmanagerConfig{},
			},
		},
		httpClient:  client,
		l:           &sync.Mutex{},
		ruleContent: make(map[string][]byte),
	}
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
	// waiting started
	go func() {
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
func (p *Manager) saveRuleFile() (list []string, change bool, err error) {
	os.MkdirAll(path.Join(p.Opt.AlertingRulesFileDir+".cache"), 0755)
	for _, r := range p.Rules {
		content, err := yaml.Marshal(r.Spec)
		if err != nil {
			logrus.Errorf("marshal rule config failure %s", err.Error())
			return nil, false, err
		}
		errs := ValidateRule(r.Spec)
		if len(errs) != 0 {
			logrus.Debugf("invalid rule %s content: %s", r.Name, content)
			for _, err := range errs {
				logrus.Errorf("invalid rule %s: %s", r.Name, err.Error())
			}
			return nil, false, err
		}
		os.MkdirAll(path.Join(p.Opt.AlertingRulesFileDir+".cache", r.Namespace), 0755)
		ruleFile := path.Join(p.Opt.AlertingRulesFileDir+".cache", r.Namespace, r.Name+".yaml")
		if err := ioutil.WriteFile(ruleFile, content, 0755); err != nil {
			return nil, false, err
		}
		if old, ok := p.ruleContent[path.Join(r.Namespace, r.Name)]; ok && !change {
			change = !bytes.Equal(old, content)
		} else {
			change = true
			p.ruleContent[path.Join(r.Namespace, r.Name)] = content
		}
		list = append(list, path.Join(p.Opt.AlertingRulesFileDir, r.Namespace, r.Name+".yaml"))
	}
	// clear old config and taking effect the new configuration
	os.RemoveAll(p.Opt.AlertingRulesFileDir)
	if err := os.Rename(p.Opt.AlertingRulesFileDir+".cache", p.Opt.AlertingRulesFileDir); err != nil {
		return nil, false, err
	}
	return
}

// ValidateRule takes PrometheusRuleSpec and validates it using the upstream prometheus rule validator
func ValidateRule(promRule mv1.PrometheusRuleSpec) []error {
	for i, group := range promRule.Groups {
		if group.PartialResponseStrategy == "" {
			continue
		}
		if _, ok := thanostypes.PartialResponseStrategy_value[strings.ToUpper(group.PartialResponseStrategy)]; !ok {
			return []error{
				fmt.Errorf("invalid partial_response_strategy %s value", group.PartialResponseStrategy),
			}
		}
		// reset this as the upstream prometheus rule validator
		// is not aware of the partial_response_strategy field
		promRule.Groups[i].PartialResponseStrategy = ""
	}
	content, err := yaml.Marshal(promRule)
	if err != nil {
		return []error{fmt.Errorf("failed to marshal content %s", err.Error())}
	}
	_, errs := rulefmt.Parse(content)
	return errs
}

// SaveConfig save config
func (p *Manager) SaveConfig() error {
	filePaths, ruleChange, err := p.saveRuleFile()
	if err != nil {
		return fmt.Errorf("write rule file failure %s", err.Error())
	}
	p.Config.RuleFiles = filePaths
	logrus.Debug("save prometheus config file.")
	currentConf, err := cyaml.Marshal(p.Config)
	if err != nil {
		logrus.Error("Marshal prometheus config to yaml error.", err.Error())
		return err
	}
	if !ruleChange && bytes.Equal(currentConf, p.generatedConf) {
		logrus.Debug("updating Prometheus configuration skipped, no configuration change")
		return nil
	}
	err = ioutil.WriteFile(p.Opt.ConfigFile, currentConf, 0644)
	if err != nil {
		logrus.Error("write prometheus config file error.", err.Error())
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
func (p *Manager) UpdateScrapeAndRule(scrapes []*ScrapeConfig, rules []*mv1.PrometheusRule) {
	p.Config.ScrapeConfigs = scrapes
	p.Rules = rules
	if err := p.SaveConfig(); err != nil {
		logrus.Errorf("save prometheus config failure:%s", err.Error())
	}
}
