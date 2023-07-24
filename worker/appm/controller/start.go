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
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/goodrain/rainbond/worker/appm/store"

	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type startController struct {
	stopChan     chan struct{}
	controllerID string
	appService   []v1.AppService
	manager      *Manager
	ctx          context.Context
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
	for _, slist := range sl {
		var wait sync.WaitGroup
		for _, service := range slist {
			wait.Add(1)
			go func(service v1.AppService) {
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
		ctx:     s.ctx,
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
	_, err := s.manager.client.CoreV1().Namespaces().Get(s.ctx, app.GetNamespace(), metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = s.manager.client.CoreV1().Namespaces().Create(s.ctx, app.GetTenant(), metav1.CreateOptions{})
		}
		if err != nil {
			return fmt.Errorf("create or check namespace failure %s", err.Error())
		}
	}
	// for custom component
	if len(app.GetManifests()) > 0 {
		for _, manifest := range app.GetManifests() {
			if err := s.manager.apply.Apply(s.ctx, manifest); err != nil {
				return fmt.Errorf("apply custom component manifest %s/%s failure %s", manifest.GetKind(), manifest.GetName(), err.Error())
			}
		}
	}
	// for core component
	//step 1: create configmap
	if configs := app.GetConfigMaps(); configs != nil {
		for _, config := range configs {
			_, err := s.manager.client.CoreV1().ConfigMaps(app.GetNamespace()).Create(s.ctx, config, metav1.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				return fmt.Errorf("create config map failure:%s", err.Error())
			}
		}
	}
	// create claims
	for _, claim := range app.GetClaimsManually() {
		logrus.Debugf("create claim: %s", claim.Name)
		_, err := s.manager.client.CoreV1().PersistentVolumeClaims(app.GetNamespace()).Create(s.ctx, claim, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return fmt.Errorf("create claims: %v", err)
		}
	}
	//step 2: create statefulset or deployment or job or cronjob
	if statefulset := app.GetStatefulSet(); statefulset != nil {
		_, err = s.manager.client.AppsV1().StatefulSets(app.GetNamespace()).Create(s.ctx, statefulset, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create statefulset failure:%s", err.Error())
		}
	}
	if deployment := app.GetDeployment(); deployment != nil {
		_, err = s.manager.client.AppsV1().Deployments(app.GetNamespace()).Create(s.ctx, deployment, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create deployment failure:%s;", err.Error())
		}
	}
	if job := app.GetJob(); job != nil {
		_, err = s.manager.client.BatchV1().Jobs(app.GetNamespace()).Create(s.ctx, job, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create job failure:%s;", err.Error())
		}
	}
	if cronjob := app.GetCronJob(); cronjob != nil {
		_, err = s.manager.client.BatchV1().CronJobs(app.GetNamespace()).Create(s.ctx, cronjob, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create cronjob failure:%s;", err.Error())
		}
	}
	if cronjob := app.GetBetaCronJob(); cronjob != nil {
		_, err = s.manager.client.BatchV1beta1().CronJobs(app.GetNamespace()).Create(s.ctx, cronjob, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create beta cronjob failure:%s;", err.Error())
		}
	}
	//step 3: create services
	if services := app.GetServices(true); services != nil {
		if err := CreateKubeService(s.manager.client, app.GetNamespace(), services...); err != nil {
			return fmt.Errorf("create service failure %s", err.Error())
		}
	}
	//step 4: create secrets
	if secrets := append(app.GetSecrets(true), app.GetEnvVarSecrets(true)...); secrets != nil {
		for _, secret := range secrets {
			if len(secret.ResourceVersion) == 0 {
				_, err := s.manager.client.CoreV1().Secrets(app.GetNamespace()).Create(s.ctx, secret, metav1.CreateOptions{})
				if err != nil && !errors.IsAlreadyExists(err) {
					return fmt.Errorf("create secret failure:%s", err.Error())
				}
			}
		}
	}
	//step 5: create ingress
	ingresses, betaIngresses := app.GetIngress(true)
	for _, ingress := range ingresses {
		if len(ingress.ResourceVersion) == 0 {
			_, err := s.manager.client.NetworkingV1().Ingresses(app.GetNamespace()).Create(s.ctx, ingress, metav1.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				return fmt.Errorf("create ingress failure:%s", err.Error())
			}
		}
	}
	for _, ingress := range betaIngresses {
		if len(ingress.ResourceVersion) == 0 {
			_, err := s.manager.client.NetworkingV1beta1().Ingresses(app.GetNamespace()).Create(s.ctx, ingress, metav1.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				return fmt.Errorf("create ingress failure:%s", err.Error())
			}
		}
	}

	//step 6: create hpa
	if hpas := app.GetHPAs(); len(hpas) != 0 {
		for _, hpa := range hpas {
			if len(hpa.ResourceVersion) == 0 {
				_, err := s.manager.client.AutoscalingV2().HorizontalPodAutoscalers(hpa.GetNamespace()).Create(s.ctx, hpa, metav1.CreateOptions{})
				if err != nil && !errors.IsAlreadyExists(err) {
					logrus.Debugf("hpa: %#v", hpa)
					return fmt.Errorf("create hpa: %v", err)
				}
			}
		}
	}
	if hpas := app.GetHPABeta2s(); len(hpas) != 0 {
		for _, hpa := range hpas {
			if len(hpa.ResourceVersion) == 0 {
				_, err := s.manager.client.AutoscalingV2beta2().HorizontalPodAutoscalers(hpa.GetNamespace()).Create(s.ctx, hpa, metav1.CreateOptions{})
				if err != nil && !errors.IsAlreadyExists(err) {
					logrus.Debugf("hpa: %#v", hpa)
					return fmt.Errorf("create hpa: %v", err)
				}
			}
		}
	}
	//step 7: create CR resource
	if crd, _ := s.manager.store.GetCrd(store.ServiceMonitor); crd != nil {
		if sms := app.GetServiceMonitors(true); len(sms) > 0 {
			smClient, err := s.manager.store.GetServiceMonitorClient()
			if err != nil {
				logrus.Errorf("create service monitor client failure %s", err.Error())
			}
			if smClient != nil {
				for _, sm := range sms {
					if len(sm.ResourceVersion) == 0 {
						_, err := smClient.MonitoringV1().ServiceMonitors(sm.GetNamespace()).Create(s.ctx, sm, metav1.CreateOptions{})
						if err != nil && !errors.IsAlreadyExists(err) {
							logrus.Errorf("create service monitor failure: %s", err.Error())
						}
					}
				}
			}
		}
	}

	//step 8: waiting endpoint ready
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
