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

package healthz

import (
	"context"
	"errors"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/mq/client"
	"github.com/goodrain/rainbond/prober"
	"github.com/goodrain/rainbond/prober/types/v1"
)

var defaultManager Manager

type Manager interface {
	Init() error
	Start()
	Stop()
	GetCurrentStatus(serviceName string) (string, error)
}

type manager struct {
	dbm    db.Manager
	pm     prober.Manager
	mqcli  client.MQClient
	ctx    context.Context
	cancel context.CancelFunc
}

func NewManager(mqcli client.MQClient) Manager {
	ctx, cancel := context.WithCancel(context.Background())
	defaultManager = &manager{
		dbm:    db.GetManager(),
		pm:     prober.CreateManager(),
		mqcli:  mqcli,
		ctx:    ctx,
		cancel: cancel,
	}
	return defaultManager
}

func GetManager() Manager {
	return defaultManager
}

func CloseManager() error {
	if defaultManager == nil {
		logrus.Warningf("default healthz manager has not been initialized yet")
		return errors.New("default healthz manager has not been initialized yet")
	}
	defaultManager.Stop()
	return nil
}

func (m *manager) Init() error {
	// svcs, err := m.dbm.TenantServiceDao().ListThirdPartyServices()
	// if err != nil {
	// 	return err
	// }
	// var services []*v1.Service
	// for _, svc := range svcs {
	// 	if !m.dbm.TenantServicesPortDao().HasOpenPort(svc.ServiceID) {
	// 		continue
	// 	}
	// 	endpoints, err := thirdparty.ListEndpoints(svc.ServiceID, m.dbm)
	// 	if err != nil {
	// 		logrus.Warningf("ServiceID: %s; Ignore; Error listing endpoints: %v", svc.ServiceID, err)
	// 		continue
	// 	}
	// 	if endpoints == nil || len(endpoints) == 0 {
	// 		logrus.Warningf("ServiceID: %s; Ignore; Empty endpoints", svc.ServiceID)
	// 		continue
	// 	}
	// 	conv, err := thirdparty.Conv(endpoints)
	// 	if err != nil {
	// 		if err != nil {
	// 			logrus.Warningf("ServiceID: %s; Ignore; Error struct conversion: %v", svc.ServiceID, err)
	// 			continue
	// 		}
	// 	}
	// 	probes, err := m.dbm.ServiceProbeDao().GetServiceProbes(svc.ServiceID)
	// 	service := createService(probes, err)
	// 	for _, ep := range conv {
	// 		if ep.IPs == nil || len(ep.IPs) == 0 {
	// 			continue
	// 		}
	// 		for _, ip := range ep.IPs {
	// 			service.Name = workerutil.GenServiceName(svc.ServiceID, ip)
	// 			service.ServiceHealth.Name = workerutil.GenServiceName(svc.ServiceID, ip) // TODO: unused ServiceHealth.Name, consider to delete it.
	// 			service.ServiceHealth.Address = fmt.Sprintf("%s/%d", ip, ep.Port)
	// 			services = append(services, service)
	// 		}
	// 	}
	// }
	// m.pm.SetServices(&services)
	return nil
}

func createService(probes []*dbmodel.TenantServiceProbe, err error) *v1.Service {
	var service v1.Service
	if err != nil || probes == nil || len(probes) == 0 {
		// no defined probe, use default one
		service = v1.Service{
			Disable: false,
		}
		service.ServiceHealth.Model = "tcp"
		service.ServiceHealth.TimeInterval = 5
		service.ServiceHealth.MaxErrorsNum = 3
	} else {
		service = v1.Service{
			Disable: false,
		}
		service.ServiceHealth.Model = probes[0].Scheme
		service.ServiceHealth.TimeInterval = probes[0].PeriodSecond
		service.ServiceHealth.MaxErrorsNum = probes[0].FailureThreshold
	}
	return &service
}

func (m *manager) Start() {
	for _, service := range *m.pm.GetServices() {
		watcher := m.pm.WatchServiceHealthy(service.Name)
		m.pm.EnableWatcher(watcher.GetServiceName(), watcher.GetID())
		defer watcher.Close()
		defer m.pm.DisableWatcher(watcher.GetServiceName(), watcher.GetID())

		for {
			select {
			case event := <-watcher.Watch():
				switch event.Status {
				case v1.StatHealthy:
					logrus.Debugf("is [%s] of service %s.", event.Status, event.Name)
				case v1.StatUnhealthy:
					if service.ServiceHealth != nil {
						if event.ErrorNumber > service.ServiceHealth.MaxErrorsNum {
							logrus.Infof("is [%s] of service %s %d times and restart it.", event.Status, event.Name, event.ErrorNumber)
						}
						//sid, err := workerutil.GetServiceID(event.Name)
						//if err != nil {
						//	logrus.Warningf("error getting service id: %v", err)
						//	continue
						//}
						//body := make(map[string]interface{})
						//body["service_id"] = sid
						//body["action"] = "stat-unhealthy"
						//err = m.mqcli.SendBuilderTopic(client.TaskStruct{
						//	Topic:    client.WorkerTopic,
						//	TaskType: "apply_rule",
						//	TaskBody: body,
						//})
						//if err != nil {
						//	logrus.Warningf("errro sending msg to mq: %v", err)
						//}
					}
				case v1.StatDeath:
					logrus.Infof("is [%s] of service %s %d times and start it.", event.Status, event.Name, event.ErrorNumber)
				}
			case <-m.ctx.Done():
				return
			}
		}
	}

	m.pm.Start()
}

// Stop stops healthz manager.
func (m *manager) Stop() {
	m.cancel()
}

// GetCurrentStatus returns the current status of the service according to serviceName.
func (m *manager) GetCurrentStatus(serviceName string) (string, error) {
	status, err := m.pm.GetCurrentServiceHealthy(serviceName)
	if err != nil {
		return "", err
	}
	return status.Status, nil
}
