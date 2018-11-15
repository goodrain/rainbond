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

package controller

import (
	"fmt"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"

	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
)

type startController struct {
	stopChan     chan struct{}
	controllerID string
	appService   []v1.AppService
	manager      *Manager
	callback     func(controllerID string, err error)
}

func (s *startController) Begin() error {
	var sourceIDs = make(map[string]*v1.AppService, len(s.appService))
	for _, a := range s.appService {
		sourceIDs[a.ServiceID] = &a
	}
	var sl sequencelist
	foundsequence(sourceIDs, &sl)
	for _, slist := range sl {
		var wait sync.WaitGroup
		for _, service := range slist {
			go func(service v1.AppService) {
				wait.Add(1)
				defer wait.Done()
				service.Logger.Info("App runtime begin start app service "+service.ServiceAlias, getLoggerOption("starting"))
				if err := s.startOne(&wait, service); err != nil {
					service.Logger.Error(fmt.Sprintf("start service %s failure %s", service.ServiceAlias, err.Error()), getLoggerOption("failure"))
					logrus.Errorf("start service %s failure %s", service.ServiceAlias, err.Error())
				}
			}(*service)
		}
		wait.Wait()
	}
	return nil
}

func (s *startController) startOne(wait *sync.WaitGroup, app v1.AppService) error {
	//step 1: create configmap
	if configs := app.GetConfigMaps(); configs != nil {
		for _, config := range configs {
			_, err := s.manager.client.CoreV1().ConfigMaps(app.TenantID).Create(config)
			if err != nil {
				return fmt.Errorf("create config map failure:%s", err.Error())
			}
		}
	}
	//step 2: create statefulset or deployment
	if statefulset := app.GetStatefulSet(); statefulset != nil {
		_, err := s.manager.client.AppsV1().StatefulSets(app.TenantID).Create(statefulset)
		if err != nil {
			return fmt.Errorf("create statefulset failure:%s", err.Error())
		}
	}
	if deployment := app.GetDeployment(); deployment != nil {
		_, err := s.manager.client.AppsV1().Deployments(app.TenantID).Create(deployment)
		if err != nil {
			return fmt.Errorf("create deployment failure:%s", err.Error())
		}
	}
	//step 3: create services
	if services := app.GetServices(); services != nil {
		for _, service := range services {
			_, err := s.manager.client.CoreV1().Services(app.TenantID).Create(service)
			if err != nil {
				return fmt.Errorf("create service failure:%s", err.Error())
			}
		}
	}
	//step 4: create secrets
	if secrets := app.GetSecrets(); secrets != nil {
		for _, secret := range secrets {
			_, err := s.manager.client.CoreV1().Secrets(app.TenantID).Create(secret)
			if err != nil {
				return fmt.Errorf("create secret failure:%s", err.Error())
			}
		}
	}
	//step 5: create ingress
	if ingresses := app.GetIngress(); ingresses != nil {
		for _, ingress := range ingresses {
			_, err := s.manager.client.ExtensionsV1beta1().Ingresses(app.TenantID).Create(ingress)
			if err != nil {
				return fmt.Errorf("create ingress failure:%s", err.Error())
			}
		}
	}
	//step 6: waiting endpoint ready
	app.Logger.Info("Create all app model success, will waiting app ready", getLoggerOption("running"))
	return s.WaitingReady(app)
}

//WaitingReady wait app start or upgrade ready
func (s *startController) WaitingReady(app v1.AppService) error {
	storeAppService := s.manager.store.GetAppService(app.ServiceID, app.DeployVersion, app.CreaterID)
	var initTime int32
	if podt := app.GetPodTemplate(); podt != nil {
		if probe := podt.Spec.Containers[0].ReadinessProbe; probe != nil {
			initTime = probe.InitialDelaySeconds
		}
	}
	//at least waiting time is 40 second
	initTime += 40
	if err := storeAppService.WaitReady(time.Duration(initTime), app.Logger, s.stopChan); err != nil {
		return err
	}
	return nil
}
func (s *startController) Stop() error {
	close(s.stopChan)
	return nil
}
