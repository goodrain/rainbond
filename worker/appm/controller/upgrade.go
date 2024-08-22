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

// 本文件定义了 `upgradeController` 结构体及其相关方法，用于管理应用程序的升级过程。主要功能包括：
// 1. `Begin` 方法：启动应用程序升级，遍历所有服务并并行执行升级操作。
// 2. `Stop` 方法：停止升级过程，当前实现为空。
// 3. `upgradeConfigMap` 方法：升级应用程序的配置映射（ConfigMap），包括更新、创建和删除操作。
// 4. `upgradeService` 方法：升级应用程序的服务，包括更新现有服务或创建新的服务，处理删除操作。
// 5. `upgradeOne` 方法：执行单个应用程序的完整升级流程，包括检查或创建命名空间、应用自定义组件、升级配置映射、服务、持久卷声明、状态副本集和秘密等。
// 6. `WaitingReady` 方法：等待应用程序启动或升级完成，确保应用程序达到预期的就绪状态。
//
// 该文件依赖于多个包，包括 Kubernetes API 客户端、日志记录库和 Rainbond 内部的 util 和 event 模块。通过这些依赖，`upgradeController` 可以与 Kubernetes 集群进行交互，处理应用程序的升级需求，并提供详细的日志记录功能。
//
// 主要用于 Rainbond 应用管理平台中的应用程序升级控制。

package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

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

func (s *upgradeController) Begin() {
	var wait sync.WaitGroup
	for _, service := range s.appService {
		wait.Add(1)
		go func(service v1.AppService) {
			defer wait.Done()
			service.Logger.Info("App runtime begin upgrade app service "+service.ServiceAlias, event.GetLoggerOption("starting"))
			if err := s.upgradeOne(service); err != nil {
				if err != ErrWaitTimeOut {
					service.Logger.Error(util.Translation("upgrade service error"), event.GetCallbackLoggerOption())
					logrus.Errorf("upgrade service %s failure %s", service.ServiceAlias, err.Error())
				} else {
					service.Logger.Error(util.Translation("upgrade service timeout"), event.GetTimeoutLoggerOption())
				}
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
			newc, err := s.manager.client.CoreV1().ConfigMaps(nowApp.GetNamespace()).Update(s.ctx, new, metav1.UpdateOptions{})
			if err != nil {
				logrus.Errorf("update config map failure %s", err.Error())
			}
			nowApp.SetConfigMap(newc)
			nowConfigMapMaps[new.Name] = nil
			logrus.Debugf("update configmap %s for service %s", new.Name, newapp.ServiceID)
		} else {
			newc, err := s.manager.client.CoreV1().ConfigMaps(nowApp.GetNamespace()).Create(s.ctx, new, metav1.CreateOptions{})
			if err != nil {
				logrus.Errorf("update config map failure %s", err.Error())
			}
			nowApp.SetConfigMap(newc)
			logrus.Debugf("create configmap %s for service %s", new.Name, newapp.ServiceID)
		}
	}
	for name, handle := range nowConfigMapMaps {
		if handle != nil {
			if err := s.manager.client.CoreV1().ConfigMaps(nowApp.GetNamespace()).Delete(s.ctx, name, metav1.DeleteOptions{}); err != nil {
				logrus.Errorf("delete config map failure %s", err.Error())
			}
			logrus.Debugf("delete configmap %s for service %s", name, newapp.ServiceID)
		}
	}
}

func (s *upgradeController) upgradeService(newapp v1.AppService) {
	nowApp := s.manager.store.GetAppService(newapp.ServiceID)
	nowServices := nowApp.GetServices(true)
	newService := newapp.GetServices(true)
	var nowServiceMaps = make(map[string]*corev1.Service, len(nowServices))
	for i, now := range nowServices {
		nowServiceMaps[now.Name] = nowServices[i]
	}
	for i := range newService {
		new := newService[i]
		if nowConfig, ok := nowServiceMaps[new.Name]; ok {
			nowConfig.Spec.Ports = new.Spec.Ports
			nowConfig.Spec.Type = new.Spec.Type
			nowConfig.Labels = new.Labels
			newc, err := s.manager.client.CoreV1().Services(nowApp.GetNamespace()).Update(s.ctx, nowConfig, metav1.UpdateOptions{})
			if err != nil {
				logrus.Errorf("update service failure %s", err.Error())
			}
			nowApp.SetService(newc)
			nowServiceMaps[new.Name] = nil
			logrus.Debugf("update service %s for service %s", new.Name, newapp.ServiceID)
		} else {
			err := CreateKubeService(s.manager.client, nowApp.GetNamespace(), new)
			if err != nil {
				logrus.Errorf("update service failure %s", err.Error())
			}
			nowApp.SetService(new)
			logrus.Debugf("create service %s for service %s", new.Name, newapp.ServiceID)
		}
	}
	for name, handle := range nowServiceMaps {
		if handle != nil {
			if err := s.manager.client.CoreV1().Services(nowApp.GetNamespace()).Delete(s.ctx, name, metav1.DeleteOptions{}); err != nil {
				logrus.Errorf("delete service failure %s", err.Error())
			}
			logrus.Debugf("delete service %s for service %s", name, newapp.ServiceID)
		}
	}
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
	s.upgradeConfigMap(app)
	if deployment := app.GetDeployment(); deployment != nil {
		_, err = s.manager.client.AppsV1().Deployments(deployment.Namespace).Patch(s.ctx, deployment.Name, types.MergePatchType, app.UpgradePatch["deployment"], metav1.PatchOptions{})
		if err != nil {
			app.Logger.Error(fmt.Sprintf("upgrade deployment %s failure %s", app.ServiceAlias, err.Error()), event.GetLoggerOption("failure"))
			return fmt.Errorf("upgrade deployment %s failure %s", app.ServiceAlias, err.Error())
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

	if statefulset := app.GetStatefulSet(); statefulset != nil {
		_, err = s.manager.client.AppsV1().StatefulSets(statefulset.Namespace).Patch(s.ctx, statefulset.Name, types.MergePatchType, app.UpgradePatch["statefulset"], metav1.PatchOptions{})
		if err != nil {
			logrus.Errorf("patch statefulset error : %s", err.Error())
			app.Logger.Error(fmt.Sprintf("upgrade statefulset %s failure %s", app.ServiceAlias, err.Error()), event.GetLoggerOption("failure"))
			return fmt.Errorf("upgrade statefulset %s failure %s", app.ServiceAlias, err.Error())
		}
	}

	oldApp := s.manager.store.GetAppService(app.ServiceID)
	s.upgradeService(app)
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

	return s.WaitingReady(app)
}

// WaitingReady wait app start or upgrade ready
func (s *upgradeController) WaitingReady(app v1.AppService) error {
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
