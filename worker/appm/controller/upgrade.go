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
	stderrors "errors"
	"fmt"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/appm/f"
	"github.com/goodrain/rainbond/worker/appm/store"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type upgradeController struct {
	stopChan     chan struct{}
	controllerID string
	appService   []v1.AppService
	manager      *Manager
	ctx          context.Context
}

// truncateErr renders err as a string truncated to at most max bytes. A nil
// err yields an empty string. The truncation guards against oversized K8s API
// response bodies bloating the event log stream.
func truncateErr(err error, max int) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if max <= 0 || len(msg) <= max {
		return msg
	}
	cut := msg[:max]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}

func (s *upgradeController) Begin() {
	var wait sync.WaitGroup
	for _, service := range s.appService {
		wait.Add(1)
		go func(service v1.AppService) {
			defer wait.Done()
			service.Logger.Info("App runtime begin upgrade app service "+service.ServiceAlias, event.GetLoggerOption("starting"))
			if err := s.upgradeOne(service); err != nil {
				service.Logger.Error(
					fmt.Sprintf("upgrade %s failure: %s", service.ServiceAlias, truncateErr(err, 1024)),
					event.GetLoggerOption("failure"))
				service.Logger.Error(util.Translation("upgrade service error"), event.GetCallbackLoggerOption())
				logrus.Errorf("upgrade service %s failure %s", service.ServiceAlias, err.Error())
			} else {
				service.Logger.Info(fmt.Sprintf("upgrade service %s success", service.ServiceAlias), event.GetLastLoggerOption())
			}
		}(service)
	}
	wait.Wait()
	s.manager.callback(s.controllerID, nil)
}
func (s *upgradeController) Stop() error {
	return nil
}

// upgradeConfigMap reconciles the namespace ConfigMaps from nowApp to newapp.
// create/update failures are aggregated and returned so the caller can fail the
// upgrade fast (the error reaches the event log stream via the failure detail
// line). delete failures of stale ConfigMaps are only logged: a leftover
// ConfigMap does not affect the new version running, so it must not block.
func (s *upgradeController) upgradeConfigMap(nowApp *v1.AppService, newapp v1.AppService) error {
	nowConfigMaps := nowApp.GetConfigMaps()
	newConfigMaps := newapp.GetConfigMaps()
	var nowConfigMapMaps = make(map[string]*corev1.ConfigMap, len(nowConfigMaps))
	for i, now := range nowConfigMaps {
		nowConfigMapMaps[now.Name] = nowConfigMaps[i]
	}
	var errs []error
	for _, new := range newConfigMaps {
		if nowConfig, ok := nowConfigMapMaps[new.Name]; ok {
			new.UID = nowConfig.UID
			newc, err := s.manager.client.CoreV1().ConfigMaps(nowApp.GetNamespace()).Update(s.ctx, new, metav1.UpdateOptions{})
			if err != nil {
				logrus.Errorf("update config map failure %s", err.Error())
				errs = append(errs, fmt.Errorf("update configmap %s failure: %s", new.Name, err.Error()))
				continue
			}
			nowApp.SetConfigMap(newc)
			nowConfigMapMaps[new.Name] = nil
			logrus.Debugf("update configmap %s for service %s", new.Name, newapp.ServiceID)
		} else {
			newc, err := s.manager.client.CoreV1().ConfigMaps(nowApp.GetNamespace()).Create(s.ctx, new, metav1.CreateOptions{})
			if err != nil {
				logrus.Errorf("create config map failure %s", err.Error())
				errs = append(errs, fmt.Errorf("create configmap %s failure: %s", new.Name, err.Error()))
				continue
			}
			nowApp.SetConfigMap(newc)
			logrus.Debugf("create configmap %s for service %s", new.Name, newapp.ServiceID)
		}
	}
	for name, handle := range nowConfigMapMaps {
		if handle != nil {
			if err := s.manager.client.CoreV1().ConfigMaps(nowApp.GetNamespace()).Delete(s.ctx, name, metav1.DeleteOptions{}); err != nil {
				// delete of stale configmap is best-effort, never blocks the upgrade
				logrus.Errorf("delete config map failure %s", err.Error())
			}
			logrus.Debugf("delete configmap %s for service %s", name, newapp.ServiceID)
		}
	}
	return stderrors.Join(errs...)
}

// upgradeService reconciles the namespace Services from nowApp to newapp.
// create/update failures are aggregated and returned; delete failures of stale
// Services are only logged and never block the upgrade.
func (s *upgradeController) upgradeService(nowApp *v1.AppService, newapp v1.AppService) error {
	nowServices := nowApp.GetServices(true)
	newService := newapp.GetServices(true)
	var nowServiceMaps = make(map[string]*corev1.Service, len(nowServices))
	for i, now := range nowServices {
		nowServiceMaps[now.Name] = nowServices[i]
	}
	var errs []error
	for i := range newService {
		new := newService[i]
		if nowConfig, ok := nowServiceMaps[new.Name]; ok {
			nowConfig.Spec.Ports = new.Spec.Ports
			nowConfig.Spec.Type = new.Spec.Type
			nowConfig.Labels = new.Labels
			newc, err := s.manager.client.CoreV1().Services(nowApp.GetNamespace()).Update(s.ctx, nowConfig, metav1.UpdateOptions{})
			if err != nil {
				logrus.Errorf("update service failure %s", err.Error())
				errs = append(errs, fmt.Errorf("update service %s failure: %s", new.Name, err.Error()))
				continue
			}
			nowApp.SetService(newc)
			nowServiceMaps[new.Name] = nil
			logrus.Debugf("update service %s for service %s", new.Name, newapp.ServiceID)
		} else {
			err := CreateKubeService(s.manager.client, nowApp.GetNamespace(), new)
			if err != nil {
				logrus.Errorf("create service failure %s", err.Error())
				errs = append(errs, fmt.Errorf("create service %s failure: %s", new.Name, err.Error()))
				continue
			}
			nowApp.SetService(new)
			logrus.Debugf("create service %s for service %s", new.Name, newapp.ServiceID)
		}
	}
	for name, handle := range nowServiceMaps {
		if handle != nil {
			if err := s.manager.client.CoreV1().Services(nowApp.GetNamespace()).Delete(s.ctx, name, metav1.DeleteOptions{}); err != nil {
				// delete of stale service is best-effort, never blocks the upgrade
				logrus.Errorf("delete service failure %s", err.Error())
			}
			logrus.Debugf("delete service %s for service %s", name, newapp.ServiceID)
		}
	}
	return stderrors.Join(errs...)
}

func (s *upgradeController) upgradeManualClaims(oldApp *v1.AppService, newApp *v1.AppService) error {
	handleErr := func(msg string, err error) error {
		logrus.Warning(msg)
		return err
	}
	return f.UpgradeClaims(s.manager.client, newApp, oldApp.GetClaimsManually(), newApp.GetClaimsManually(), handleErr)
}

func (s *upgradeController) upgradeOne(app v1.AppService) error {
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
	nowApp := s.manager.store.GetAppService(app.ServiceID)
	if nowApp == nil {
		return fmt.Errorf("app service %s not found in store", app.ServiceID)
	}
	if err := s.upgradeConfigMap(nowApp, app); err != nil {
		return fmt.Errorf("upgrade configmap for service %s failure: %w", app.ServiceAlias, err)
	}
	if deployment := app.GetDeployment(); deployment != nil {
		_, err = s.manager.client.AppsV1().Deployments(deployment.Namespace).Patch(s.ctx, deployment.Name, types.MergePatchType, app.UpgradePatch["deployment"], metav1.PatchOptions{})
		if err != nil {
			app.Logger.Error(fmt.Sprintf("upgrade deployment %s failure %s", app.ServiceAlias, err.Error()), event.GetLoggerOption("failure"))
			return fmt.Errorf("upgrade deployment %s failure %s", app.ServiceAlias, err.Error())
		}
	}

	if statefulset := app.GetStatefulSet(); statefulset != nil {
		_, err = s.manager.client.AppsV1().StatefulSets(statefulset.Namespace).Patch(s.ctx, statefulset.Name, types.MergePatchType, app.UpgradePatch["statefulset"], metav1.PatchOptions{})
		if err != nil {
			logrus.Errorf("patch statefulset error : %s", err.Error())
			app.Logger.Error(fmt.Sprintf("upgrade statefulset %s failure %s", app.ServiceAlias, err.Error()), event.GetLoggerOption("failure"))
			return fmt.Errorf("upgrade statefulset %s failure %s", app.ServiceAlias, err.Error())
		}
	}

	oldApp := nowApp
	if err := s.upgradeManualClaims(oldApp, &app); err != nil {
		return fmt.Errorf("upgrade manual claims for service %s failure %w", app.ServiceAlias, err)
	}
	if err := s.upgradeService(oldApp, app); err != nil {
		return fmt.Errorf("upgrade service for service %s failure: %w", app.ServiceAlias, err)
	}
	handleErr := func(msg string, err error) error {
		// ignore ingress and secret error
		logrus.Warning(msg)
		return nil
	}
	oldIngresses, oldBetaIngresses := oldApp.GetIngress(true)
	newIngresses, newBetaIngresses := app.GetIngress(true)
	_ = f.UpgradeSecrets(s.manager.client, &app, oldApp.GetSecrets(true), app.GetSecrets(true), handleErr)
	_ = f.UpgradeIngress(s.manager.client, &app, oldIngresses, newIngresses, oldBetaIngresses, newBetaIngresses, handleErr)
	for _, secret := range app.GetEnvVarSecrets(true) {
		err := f.CreateOrUpdateSecret(s.manager.client, secret)
		if err != nil {
			return fmt.Errorf("[upgradeController] [upgradeOne] create or update secrets: %v", err)
		}
	}

	if crd, _ := s.manager.store.GetCrd(store.ServiceMonitor); crd != nil {
		client, err := s.manager.store.GetServiceMonitorClient()
		if err != nil {
			logrus.Errorf("create service monitor client failure %s", err.Error())
		}
		if client != nil {
			_ = f.UpgradeServiceMonitor(client, &app, oldApp.GetServiceMonitors(true), app.GetServiceMonitors(true), handleErr)
		}
	}

	// No longer wait for upgrade ready - let probe health detection handle it
	return nil
}

// WaitingReady wait app start or upgrade ready
func (s *upgradeController) WaitingReady(app v1.AppService) error {
	// kubeblocks_component 的 workload 由 KubeBlocks 管理，Rainbond 不等待其工作负载就绪
	if app.ServiceType == v1.TypeKubeBlocks {
		return nil
	}

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
	timeout := time.Second * time.Duration(40+initTime)
	if storeAppService != nil && storeAppService.Replicas >= 0 {
		timeout = timeout * time.Duration((storeAppService.Replicas)*2)
	}
	if err := WaitUpgradeReady(s.manager.store, storeAppService, timeout, app.Logger, s.stopChan); err != nil {
		return err
	}
	return nil
}
