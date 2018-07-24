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

package controller

import (
	"context"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/node/nodem/healthy"
	"github.com/goodrain/rainbond/node/nodem/service"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"github.com/coreos/etcd/clientv3"
	"os/exec"
)

type ManagerService struct {
	ctx            context.Context
	cancel         context.CancelFunc
	conf           *option.Conf
	ctr            Controller
	cluster        client.ClusterClient
	healthyManager healthy.Manager
	services       []*service.Service
}

func (m *ManagerService) GetAllService() ([]*service.Service, error) {
	return m.services, nil
}

// start and monitor all service
func (m *ManagerService) Start() error {
	logrus.Info("Starting node controller manager.")

	err := m.Online()
	if err != nil {
		return err
	}

	for _, s := range m.services {
		serviceName := s.Name
		go m.SyncService(serviceName)
	}

	return nil
}

// stop manager
func (m *ManagerService) Stop() error {
	m.cancel()
	return nil
}

// start all service of on the node
func (m *ManagerService) Online() error {
	services, err := loadServicesFromLocal(m.conf.ServiceListFile)
	if err != nil {
		logrus.Error("Failed to load all services: ", err)
		return err
	}
	m.services = services

	// registry local services endpoint into cluster manager
	hostIp := m.cluster.GetOptions().HostIP
	for _, s := range services {
		logrus.Debug("Parse endpoints for service: ", s.Name)
		for _, end := range s.Endpoints {
			logrus.Debug("Discovery endpoints: ", end.Name)
			endpoint := toEndpoint(end, hostIp)
			oldEndpoints := m.cluster.GetEndpoints(end.Name)
			if exist := isExistEndpoint(oldEndpoints, endpoint); !exist {
				oldEndpoints = append(oldEndpoints, endpoint)
				m.cluster.SetEndpoints(end.Name, oldEndpoints)
			}
		}
	}

	err = m.WriteServices()
	if err != nil {
		return err
	}

	if ok := m.ctr.CheckBeforeStart(); !ok {
		return fmt.Errorf("check environments is not passed")
	}

	m.ctr.StartList(m.services)

	return nil
}

// stop all service of on the node
func (m *ManagerService) Offline() error {
	// Anti-registry local services endpoint from cluster manager
	hostIp := m.cluster.GetOptions().HostIP
	services, _ := m.GetAllService()
	for _, s := range services {
		for _, end := range s.Endpoints {
			logrus.Debug("Anti-registry endpoint: ", end.Name)
			endpoint := toEndpoint(end, hostIp)
			oldEndpoints := m.cluster.GetEndpoints(end.Name)
			if exist := isExistEndpoint(oldEndpoints, endpoint); exist {
				m.cluster.SetEndpoints(end.Name, rmEndpointFrom(oldEndpoints, endpoint))
			}
		}
	}

	if err := m.ctr.StopList(m.services); err != nil {
		return err
	}

	return nil
}

// synchronize all service status to as we expect
func (m *ManagerService) SyncService(name string) {
	logrus.Error("Start watch the service status ", name)

	w := m.healthyManager.WatchServiceHealthy(name)
	if w == nil {
		logrus.Error("Not found watcher of the service ", name)
		return
	}

	unhealthyNum := 0
	maxUnhealthyNum := 2

	for {
		select {
		case event := <-w.Watch():
			switch event.Status {
			case service.Stat_healthy:
				logrus.Debugf("[%s] check service %s.", event.Status, event.Name)
			case service.Stat_unhealthy:
				logrus.Infof("[%s] check service %s %d times.", event.Status, event.Name, unhealthyNum)
				if unhealthyNum > maxUnhealthyNum {
					logrus.Infof("[%s] check service %s %d times and will be restart.", event.Status, event.Name, unhealthyNum)
					if event.Name == "docker" {
						logrus.Infof("Skip restart docker daemon.")
						continue
					}
					m.ctr.RestartService(event.Name)
					unhealthyNum = 0
				}
				unhealthyNum++
			case service.Stat_death:
				logrus.Infof("[%s] check service %s %d times.", event.Status, event.Name, unhealthyNum)
				if unhealthyNum > maxUnhealthyNum {
					logrus.Infof("[%s] check service %s %d times and will be restart.", event.Status, event.Name, unhealthyNum)
					if event.Name == "docker" {
						logrus.Infof("Skip restart docker daemon.")
						continue
					}
					m.ctr.RestartService(event.Name)
					unhealthyNum = 0
				}
				unhealthyNum++
			}
		case <-m.ctx.Done():
			return
		}
	}
}

/*
1. reload services config from local file system
2. regenerate systemd config for all service
3. start all services of status is not running
*/
func (m *ManagerService) ReLoadServices() error {
	return m.Online()
}

func (m *ManagerService) WriteServices() error {
	for _, s := range m.services {
		if s.Name == "docker" {
			continue
		}
		err := m.ctr.WriteConfig(s)
		if err != nil {
			return err
		}
		m.ctr.EnableService(s.Name)
	}

	return nil
}

func (m *ManagerService) RemoveServices() error {
	for _, s := range m.services {
		if s.Name == "docker" {
			continue
		}
		m.ctr.DisableService(s.Name)
		m.ctr.RemoveConfig(s.Name)
	}

	return nil
}

func loadServicesFromLocal(serviceListFile string) ([]*service.Service, error) {
	logrus.Info("Loading all services from local.")

	// load default-configs.yaml
	content, err := ioutil.ReadFile(serviceListFile)
	if err != nil {
		logrus.Error("Failed to read default configs file: ", err)
		return nil, err
	}

	var defaultConfigs service.Services
	err = yaml.Unmarshal(content, &defaultConfigs)
	if err != nil {
		logrus.Error("Failed to parse default configs yaml file: ", err)
		return nil, err
	}

	return defaultConfigs.Services, nil
}

func isExistEndpoint(etcdEndPoints []string, end string) bool {
	for _, v := range etcdEndPoints {
		if v == end {
			return true
		}
	}
	return false
}

func rmEndpointFrom(etcdEndPoints []string, end string) []string {
	endPoints := make([]string, 0, 5)
	for _, v := range etcdEndPoints {
		if v != end {
			endPoints = append(endPoints, v)
		}
	}
	return endPoints
}

func toEndpoint(reg *service.Endpoint, ip string) string {
	if reg.Protocol == "" {
		return fmt.Sprintf("%s:%s", ip, reg.Port)
	}
	return fmt.Sprintf("%s://%s:%s", reg.Protocol, ip, reg.Port)
}

func StartRequiresSystemd(conf *option.Conf) error {
	services, err := loadServicesFromLocal(conf.ServiceListFile)
	if err != nil {
		logrus.Error("Failed to load all services: ", err)
		return err
	}

	err = exec.Command("/usr/bin/systemctl", "start", "docker").Run()
	if err != nil {
		fmt.Printf("Start docker daemon: %v", err)
		return err
	}

	for _, s := range services {
		if s.Name == "etcd" {
			fileName := fmt.Sprintf("/etc/systemd/system/%s.service", s.Name)
			content := service.ToConfig(s)
			if content == "" {
				err := fmt.Errorf("can not generate config for service %s", s.Name)
				fmt.Println(err)
				return err
			}

			if err := ioutil.WriteFile(fileName, []byte(content), 0644); err != nil {
				fmt.Printf("Generate config file %s: %v", fileName, err)
				return err
			}
			err = exec.Command("/usr/bin/systemctl", "start", s.Name).Run()
			if err != nil {
				fmt.Printf("Start service %s: %v", s.Name, err)
				return err
			}
		}
	}

	return nil
}

func NewManagerService(conf *option.Conf, healthyManager healthy.Manager) (*ManagerService, *clientv3.Client, client.ClusterClient) {
	ctx, cancel := context.WithCancel(context.Background())

	etcdcli, err := clientv3.New(conf.Etcd)
	if err != nil {
		return nil, nil, nil
	}
	cluster := client.NewClusterClient(conf, etcdcli)

	manager := &ManagerService{
		ctx:            ctx,
		cancel:         cancel,
		conf:           conf,
		cluster:        cluster,
		ctr:            NewControllerSystemd(conf, cluster),
		healthyManager: healthyManager,
	}

	return manager, etcdcli, cluster
}
