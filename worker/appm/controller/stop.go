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

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/Sirupsen/logrus"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type stopController struct {
	stopChan     chan struct{}
	controllerID string
	appService   []v1.AppService
	manager      *Manager
}

func (s *stopController) Begin() {
	var wait sync.WaitGroup
	for _, service := range s.appService {
		go func(service v1.AppService) {
			wait.Add(1)
			defer wait.Done()
			service.Logger.Info("App runtime begin stop app service "+service.ServiceAlias, getLoggerOption("starting"))
			if err := s.stopOne(service); err != nil {
				if err != ErrWaitTimeOut {
					service.Logger.Error(fmt.Sprintf("stop service %s failure %s", service.ServiceAlias, err.Error()), GetCallbackLoggerOption())
					logrus.Errorf("stop service %s failure %s", service.ServiceAlias, err.Error())
				} else {
					service.Logger.Error(fmt.Sprintf("stop service timeout,please waiting it closed"), GetTimeoutLoggerOption())
				}
			} else {
				service.Logger.Info(fmt.Sprintf("stop service %s success", service.ServiceAlias), GetLastLoggerOption())
			}
		}(service)
	}
	wait.Wait()
	s.manager.callback(s.controllerID, nil)
}
func (s *stopController) stopOne(app v1.AppService) error {
	//step 1: delete services
	if services := app.GetServices(); services != nil {
		for _, service := range services {
			err := s.manager.client.CoreV1().Services(app.TenantID).Delete(service.Name, &metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("delete service failure:%s", err.Error())
			}
		}
	}
	//step 2: delete secrets
	if secrets := app.GetSecrets(); secrets != nil {
		for _, secret := range secrets {
			if secret != nil {
				err := s.manager.client.CoreV1().Secrets(app.TenantID).Delete(secret.Name, &metav1.DeleteOptions{})
				if err != nil && !errors.IsNotFound(err) {
					return fmt.Errorf("delete secret failure:%s", err.Error())
				}
				s.manager.store.OnDelete(secret)
			}
		}
	}
	//step 3: delete ingress
	if ingresses := app.GetIngress(); ingresses != nil {
		for _, ingress := range ingresses {
			err := s.manager.client.ExtensionsV1beta1().Ingresses(app.TenantID).Delete(ingress.Name, &metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("delete ingress failure:%s", err.Error())
			}
			s.manager.store.OnDelete(ingress)
		}
	}
	//step 4: delete configmap
	if configs := app.GetConfigMaps(); configs != nil {
		for _, config := range configs {
			err := s.manager.client.CoreV1().ConfigMaps(app.TenantID).Delete(config.Name, &metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("delete config map failure:%s", err.Error())
			}
			s.manager.store.OnDelete(config)
		}
	}
	//step 5: delete statefulset or deployment
	if statefulset := app.GetStatefulSet(); statefulset != nil {
		err := s.manager.client.AppsV1().StatefulSets(app.TenantID).Delete(statefulset.Name, &metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("delete statefulset failure:%s", err.Error())
		}
		s.manager.store.OnDelete(statefulset)
	}
	if deployment := app.GetDeployment(); deployment != nil {
		err := s.manager.client.AppsV1().Deployments(app.TenantID).Delete(deployment.Name, &metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("delete deployment failure:%s", err.Error())
		}
		s.manager.store.OnDelete(deployment)
	}
	//step 6: delete all pod
	if pods := app.GetPods(); pods != nil {
		for _, pod := range pods {
			err := s.manager.client.CoreV1().Pods(app.TenantID).Delete(pod.Name, &metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("delete pod failure:%s", err.Error())
			}
			s.manager.store.OnDelete(pod)
		}
	}
	//step 7: waiting endpoint ready
	app.Logger.Info("Delete all app model success, will waiting app closed", getLoggerOption("running"))
	return s.WaitingReady(app)
}
func (s *stopController) Stop() error {
	close(s.stopChan)
	return nil
}

//WaitingReady wait app start or upgrade ready
func (s *stopController) WaitingReady(app v1.AppService) error {
	storeAppService := s.manager.store.GetAppService(app.ServiceID)
	//at least waiting time is 40 second
	var timeout = time.Second * 40
	if storeAppService != nil && storeAppService.Replicas > 0 {
		timeout = time.Duration(storeAppService.Replicas) * timeout
	}
	if err := WaitStop(s.manager.store, storeAppService, timeout, app.Logger, s.stopChan); err != nil {
		return err
	}
	return nil
}
