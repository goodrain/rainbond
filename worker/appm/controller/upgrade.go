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
	corev1 "k8s.io/api/core/v1"
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
func (s *upgradeController) upgradeConfigMap(newapp v1.AppService) {
	nowApp := s.manager.store.GetAppService(newapp.ServiceID)
	nowConfigMaps := nowApp.GetConfigMaps()
	newConfigMaps := newapp.GetConfigMaps()
	var nowConfigMapMaps = make(map[string]*corev1.ConfigMap, len(nowConfigMaps))
	for i, now := range nowConfigMaps {
		nowConfigMapMaps[now.Name] = nowConfigMaps[i]
	}
	for _, new := range newConfigMaps {
		if nowConfig, ok := nowConfigMapMaps[new.Name]; ok {
			new.UID = nowConfig.UID
			newc, err := s.manager.client.CoreV1().ConfigMaps(nowApp.TenantID).Update(new)
			if err != nil {
				logrus.Errorf("update config map failure %s", err.Error())
			}
			nowApp.SetConfigMap(newc)
			nowConfigMapMaps[new.Name] = nil
			logrus.Debugf("update configmap %s for service %s", new.Name, newapp.ServiceID)
		} else {
			newc, err := s.manager.client.CoreV1().ConfigMaps(nowApp.TenantID).Create(new)
			if err != nil {
				logrus.Errorf("update config map failure %s", err.Error())
			}
			nowApp.SetConfigMap(newc)
			logrus.Debugf("create configmap %s for service %s", new.Name, newapp.ServiceID)
		}
	}
	for name, handle := range nowConfigMapMaps {
		if handle != nil {
			if err := s.manager.client.CoreV1().ConfigMaps(nowApp.TenantID).Delete(name, &metav1.DeleteOptions{}); err != nil {
				logrus.Errorf("delete config map failure %s", err.Error())
			}
			logrus.Debugf("delete configmap %s for service %s", name, newapp.ServiceID)
		}
	}
}

func (s *upgradeController) upgradeService(newapp v1.AppService) {
	nowApp := s.manager.store.GetAppService(newapp.ServiceID)
	nowServices := nowApp.GetServices()
	newService := newapp.GetServices()
	var nowServiceMaps = make(map[string]*corev1.Service, len(nowServices))
	for i, now := range nowServices {
		nowServiceMaps[now.Name] = nowServices[i]
	}
	for _, new := range newService {
		if nowConfig, ok := nowServiceMaps[new.Name]; ok {
			new.UID = nowConfig.UID
			new.Spec.ClusterIP = nowConfig.Spec.ClusterIP
			new.ResourceVersion = nowConfig.ResourceVersion
			newc, err := s.manager.client.CoreV1().Services(nowApp.TenantID).Update(new)
			if err != nil {
				logrus.Errorf("update service failure %s", err.Error())
			}
			nowApp.SetService(newc)
			nowServiceMaps[new.Name] = nil
			logrus.Debugf("update service %s for service %s", new.Name, newapp.ServiceID)
		} else {
			newc, err := s.manager.client.CoreV1().Services(nowApp.TenantID).Create(new)
			if err != nil {
				logrus.Errorf("update service failure %s", err.Error())
			}
			nowApp.SetService(newc)
			logrus.Debugf("create service %s for service %s", new.Name, newapp.ServiceID)
		}
	}
	for name, handle := range nowServiceMaps {
		if handle != nil {
			if err := s.manager.client.CoreV1().Services(nowApp.TenantID).Delete(name, &metav1.DeleteOptions{}); err != nil {
				logrus.Errorf("delete service failure %s", err.Error())
			}
			logrus.Debugf("delete service %s for service %s", name, newapp.ServiceID)
		}
	}
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
	s.upgradeConfigMap(app)
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
	//upgrade k8s service
	s.upgradeService(app)
	//upgrade k8s secrets
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
