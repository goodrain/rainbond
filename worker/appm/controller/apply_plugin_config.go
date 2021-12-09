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

	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type applyConfigController struct {
	controllerID string
	appService   v1.AppService
	manager      *Manager
	stopChan     chan struct{}
	ctx          context.Context
}

// Begin begins applying rule
func (a *applyConfigController) Begin() {
	nowApp := a.manager.store.GetAppService(a.appService.ServiceID)
	nowConfigMaps := nowApp.GetConfigMaps()
	newConfigMaps := a.appService.GetConfigMaps()
	var nowConfigMapMaps = make(map[string]*corev1.ConfigMap, len(nowConfigMaps))
	for i, now := range nowConfigMaps {
		nowConfigMapMaps[now.Name] = nowConfigMaps[i]
	}
	for _, new := range newConfigMaps {
		if nowConfig, ok := nowConfigMapMaps[new.Name]; ok {
			new.UID = nowConfig.UID
			newc, err := a.manager.client.CoreV1().ConfigMaps(nowApp.GetNamespace()).Update(context.Background(), new, metav1.UpdateOptions{})
			if err != nil {
				logrus.Errorf("update config map failure %s", err.Error())
			}
			nowApp.SetConfigMap(newc)
			nowConfigMapMaps[new.Name] = nil
			logrus.Debugf("update configmap %s for service %s", new.Name, a.appService.ServiceID)
		} else {
			newc, err := a.manager.client.CoreV1().ConfigMaps(nowApp.GetNamespace()).Create(context.Background(), new, metav1.CreateOptions{})
			if err != nil {
				logrus.Errorf("update config map failure %s", err.Error())
			}
			nowApp.SetConfigMap(newc)
			logrus.Debugf("create configmap %s for service %s", new.Name, a.appService.ServiceID)
		}
	}
	for name, handle := range nowConfigMapMaps {
		if handle != nil {
			if err := a.manager.client.CoreV1().ConfigMaps(nowApp.GetNamespace()).Delete(context.Background(), name, metav1.DeleteOptions{}); err != nil {
				logrus.Errorf("delete config map failure %s", err.Error())
			}
			logrus.Debugf("delete configmap %s for service %s", name, a.appService.ServiceID)
		}
	}
	a.manager.callback(a.controllerID, nil)
}

func (a *applyConfigController) Stop() error {
	close(a.stopChan)
	return nil
}
