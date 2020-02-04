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

	workerutil "github.com/goodrain/rainbond/worker/util"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type startController struct {
	stopChan     chan struct{}
	controllerID string
	appService   []v1.AppService
	manager      *Manager
}

func (s *startController) Begin() {
	var sourceIDs = make(map[string]*v1.AppService, len(s.appService))
	var list []*v1.AppService // should be delete when using foundsequence
	for _, a := range s.appService {
		sourceIDs[a.ServiceID] = &a
		list = append(list, &a) // // should be delete when using foundsequence
	}
	var sl sequencelist
	sl = append(sl, list) // should be delete when using foundsequence
	//foundsequence(sourceIDs, &sl)
	for _, slist := range sl {
		var wait sync.WaitGroup
		for _, service := range slist {
			go func(service v1.AppService) {
				wait.Add(1)
				defer wait.Done()
				logrus.Debugf("App runtime begin start app service(%s)", service.ServiceAlias)
				service.Logger.Info("App runtime begin start app service "+service.ServiceAlias, event.GetLoggerOption("starting"))
				if err := s.startOne(service); err != nil {
					if err != ErrWaitTimeOut {
						service.Logger.Error(util.Translation("start service error"), event.GetCallbackLoggerOption())
						logrus.Errorf("start service %s failure %s", service.ServiceAlias, err.Error())
						s.errorCallback(service)
					} else {
						logrus.Debugf("Start service %s timeout, please wait or read service log.", service.ServiceAlias)
						service.Logger.Error(util.Translation("start service timeout"), event.GetTimeoutLoggerOption())
					}
				} else {
					logrus.Debugf("Start service %s success", service.ServiceAlias)
					service.Logger.Info(fmt.Sprintf("Start service %s success", service.ServiceAlias), event.GetLastLoggerOption())
				}
			}(*service)
		}
		wait.Wait()
		s.manager.callback(s.controllerID, nil)
	}
}
func (s *startController) errorCallback(app v1.AppService) error {
	app.Logger.Info("Begin clean resources that have been created", event.GetLoggerOption("starting"))
	stopController := stopController{
		manager: s.manager,
	}
	if err := stopController.stopOne(app); err != nil {
		logrus.Errorf("stop app failure after start failure. %s", err.Error())
		app.Logger.Error(fmt.Sprintf("Stop app failure %s", app.ServiceAlias), event.GetLoggerOption("failure"))
		return err
	}
	return nil
}
func (s *startController) startOne(app v1.AppService) error {
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
	//step 1: create configmap
	if configs := app.GetConfigMaps(); configs != nil {
		for _, config := range configs {
			_, err := s.manager.client.CoreV1().ConfigMaps(app.TenantID).Create(config)
			if err != nil && !errors.IsAlreadyExists(err) {
				return fmt.Errorf("create config map failure:%s", err.Error())
			}
		}
	}
	// before create app, prepare poddnsconfig
	podDNSConfig := workerutil.MakePodDNSConfig(s.manager.client, app.TenantID, s.manager.rbdNamespace, s.manager.rbdDNSName)
	//step 2: create statefulset or deployment
	if statefulset := app.GetStatefulSet(); statefulset != nil {
		if podDNSConfig != nil {
			statefulset.Spec.Template.Spec.DNSConfig = podDNSConfig
			statefulset.Spec.Template.Spec.DNSPolicy = "None"
		}

		_, err = s.manager.client.AppsV1().StatefulSets(app.TenantID).Create(statefulset)
		if err != nil {
			return fmt.Errorf("create statefulset failure:%s", err.Error())
		}
	}
	if deployment := app.GetDeployment(); deployment != nil {
		if podDNSConfig != nil {
			deployment.Spec.Template.Spec.DNSConfig = podDNSConfig
			deployment.Spec.Template.Spec.DNSPolicy = "None"
		}

		_, err = s.manager.client.AppsV1().Deployments(app.TenantID).Create(deployment)
		if err != nil {
			return fmt.Errorf("create deployment failure:%s;", err.Error())
		}
	}
	//step 3: create services
	if services := app.GetServices(); services != nil {
		if err := CreateKubeService(s.manager.client, app.TenantID, services...); err != nil {
			return fmt.Errorf("Create service failure %s", err.Error())
		}
	}
	//step 4: create secrets
	if secrets := app.GetSecrets(); secrets != nil {
		for _, secret := range secrets {
			_, err := s.manager.client.CoreV1().Secrets(app.TenantID).Create(secret)
			if err != nil && !errors.IsAlreadyExists(err) {
				return fmt.Errorf("create secret failure:%s", err.Error())
			}
		}
	}
	//step 5: create ingress
	if ingresses := app.GetIngress(); ingresses != nil {
		for _, ingress := range ingresses {
			_, err := s.manager.client.ExtensionsV1beta1().Ingresses(app.TenantID).Create(ingress)
			if err != nil && !errors.IsAlreadyExists(err) {
				return fmt.Errorf("create ingress failure:%s", err.Error())
			}
		}
	}

	if hpas := app.GetHPAs(); len(hpas) != 0 {
		for _, hpa := range hpas {
			_, err := s.manager.client.AutoscalingV2beta1().HorizontalPodAutoscalers(hpa.GetNamespace()).Create(hpa)
			if err != nil && !errors.IsAlreadyExists(err) {
				logrus.Debugf("hpa: %#v", hpa)
				return fmt.Errorf("create hpa: %v", err)
			}
		}
	}

	//step 6: waiting endpoint ready
	app.Logger.Info("Create all app model success, will waiting app ready", event.GetLoggerOption("running"))
	return s.WaitingReady(app)
}

//WaitingReady wait app start or upgrade ready
func (s *startController) WaitingReady(app v1.AppService) error {
	storeAppService := s.manager.store.GetAppService(app.ServiceID)
	var initTime int32
	if podt := app.GetPodTemplate(); podt != nil {
		for _, c := range podt.Spec.Containers {
			if c.ReadinessProbe != nil {
				initTime = c.ReadinessProbe.InitialDelaySeconds
				break
			}
			if c.LivenessProbe != nil {
				initTime = c.LivenessProbe.InitialDelaySeconds
				break
			}
		}
	}
	//at least waiting time is 40 second
	initTime += 40
	timeout := time.Duration(initTime * int32(app.Replicas))
	if timeout.Seconds() < 40 {
		timeout = time.Second * 40
	}
	if err := WaitReady(s.manager.store, storeAppService, timeout, app.Logger, s.stopChan); err != nil {
		return err
	}
	return nil
}
func (s *startController) Stop() error {
	close(s.stopChan)
	return nil
}
