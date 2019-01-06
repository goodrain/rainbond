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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
)

type upgradeController struct {
	stopChan     chan struct{}
	controllerID string
	appService   []v1.AppService
	manager      *Manager
}

func (s *upgradeController) Begin() {
	var wait sync.WaitGroup
	for _, service := range s.appService {
		go func(service v1.AppService) {
			wait.Add(1)
			defer wait.Done()
			service.Logger.Info("App runtime begin upgrade app service "+service.ServiceAlias, getLoggerOption("starting"))
			if err := s.upgradeOne(service); err != nil {
				if err != ErrWaitTimeOut {
					service.Logger.Error(fmt.Sprintf("upgrade service %s failure %s", service.ServiceAlias, err.Error()), GetCallbackLoggerOption())
					logrus.Errorf("upgrade service %s failure %s", service.ServiceAlias, err.Error())
				} else {
					service.Logger.Error(fmt.Sprintf("upgrade service timeout,please waiting it complete"), GetTimeoutLoggerOption())
				}
			} else {
				service.Logger.Info(fmt.Sprintf("upgrade service %s success", service.ServiceAlias), GetLastLoggerOption())
			}
		}(service)
	}
	wait.Wait()
	s.manager.callback(s.controllerID, nil)
}
func (s *upgradeController) Stop() error {
	return nil
}

func (s *upgradeController) upgradeOne(app v1.AppService) error {
	//first: check and create namespace
	_, err := s.manager.client.CoreV1().Namespaces().Get(app.TenantID, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = s.manager.client.CoreV1().Namespaces().Create(app.GetTenant())
		}
		if err != nil {
			return fmt.Errorf("create or check namespace failure %s", err.Error())
		}
	}

	if deployment := app.GetDeployment(); deployment != nil {
		_, err := s.manager.client.AppsV1().Deployments(deployment.Namespace).Patch(deployment.Name, types.MergePatchType, app.UpgradePatch["deployment"])
		if err != nil {
			app.Logger.Error(fmt.Sprintf("upgrade deployment %s failure %s", app.ServiceAlias, err.Error()), getLoggerOption("failure"))
			return fmt.Errorf("upgrade deployment %s failure %s", app.ServiceAlias, err.Error())
		}
	}
	if statefulset := app.GetStatefulSet(); statefulset != nil {
		_, err := s.manager.client.AppsV1().StatefulSets(statefulset.Namespace).Patch(statefulset.Name, types.MergePatchType, app.UpgradePatch["statefulset"])
		if err != nil {
			app.Logger.Error(fmt.Sprintf("upgrade statefulset %s failure %s", app.ServiceAlias, err.Error()), getLoggerOption("failure"))
			return fmt.Errorf("upgrade statefulset %s failure %s", app.ServiceAlias, err.Error())
		}
	}

	if ingresses := app.GetIngress(); ingresses != nil {
		for _, ingress := range ingresses {
			_, err := s.manager.client.Extensions().Ingresses(ingress.Namespace).Update(ingress)
			if err != nil {
				app.Logger.Error(fmt.Sprintf("upgrade ingress %s failure %s", app.ServiceAlias, err.Error()), getLoggerOption("failure"))
				logrus.Errorf("upgrade ingress %s failure %s", app.ServiceAlias, err.Error())
			}
		}
	}
	if services := app.GetServices(); services != nil {
		for _, service := range services {
			_, err := s.manager.client.CoreV1().Services(service.Namespace).Update(service)
			if err != nil {
				app.Logger.Error(fmt.Sprintf("upgrade service %s failure %s", app.ServiceAlias, err.Error()), getLoggerOption("failure"))
				logrus.Errorf("upgrade service %s failure %s", app.ServiceAlias, err.Error())
			}
		}
	}
	if configs := app.GetConfigMaps(); configs != nil {
		for _, config := range configs {
			_, err := s.manager.client.CoreV1().ConfigMaps(config.Namespace).Update(config)
			if err != nil {
				app.Logger.Error(fmt.Sprintf("upgrade service %s failure %s", app.ServiceAlias, err.Error()), getLoggerOption("failure"))
				logrus.Errorf("upgrade service %s failure %s", app.ServiceAlias, err.Error())
			}
		}
	}
	if secrets := app.GetSecrets(); secrets != nil {
		for _, secret := range secrets {
			_, err := s.manager.client.CoreV1().Secrets(secret.Namespace).Update(secret)
			if err != nil {
				app.Logger.Error(fmt.Sprintf("upgrade secret %s failure %s", app.ServiceAlias, err.Error()), getLoggerOption("failure"))
				logrus.Errorf("upgrade secret %s failure %s", app.ServiceAlias, err.Error())
			}
		}
	}
	return s.WaitingReady(app)
}

//WaitingReady wait app start or upgrade ready
func (s *upgradeController) WaitingReady(app v1.AppService) error {
	storeAppService := s.manager.store.GetAppService(app.ServiceID)
	var initTime int32
	if podt := app.GetPodTemplate(); podt != nil {
		if probe := podt.Spec.Containers[0].ReadinessProbe; probe != nil {
			initTime = probe.InitialDelaySeconds
		}
	}
	//at least waiting time is 40 second
	timeout := time.Second * time.Duration(40+initTime)
	if storeAppService != nil && storeAppService.Replicas >= 0 {
		timeout = timeout * time.Duration((storeAppService.Replicas)*2)
	}
	if err := WaitUpgradeReady(s.manager.store, storeAppService, timeout, app.Logger, s.stopChan); err != nil {
		return err
	}
	return nil
}
