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
	"sync"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/goodrain/rainbond/worker/appm/f"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
)

type refreshXPAController struct {
	controllerID string
	appService   []v1.AppService
	manager      *Manager
	stopChan     chan struct{}
	ctx          context.Context
}

// Begin begins applying rule
func (a *refreshXPAController) Begin() {
	var wait sync.WaitGroup
	for _, service := range a.appService {
		wait.Add(1)
		go func(service v1.AppService) {
			defer wait.Done()
			if err := a.applyOne(a.manager.client, &service); err != nil {
				logrus.Errorf("apply rules for service %s failure: %s", service.ServiceAlias, err.Error())
			}
		}(service)
	}
	wait.Wait()
	a.manager.callback(a.controllerID, nil)
}

func (a *refreshXPAController) applyOne(clientset kubernetes.Interface, app *v1.AppService) error {
	for _, hpa := range app.GetHPAs() {
		f.EnsureHPA(hpa, clientset)
	}

	for _, hpa := range app.GetDelHPAs() {
		logrus.Debugf("hpa name: %s; start deleting hpa.", hpa.GetName())
		err := clientset.AutoscalingV2beta2().HorizontalPodAutoscalers(hpa.GetNamespace()).Delete(context.Background(), hpa.GetName(), metav1.DeleteOptions{})
		if err != nil {
			// don't return error, hope it is ok next time
			logrus.Warningf("error deleting secret(%#v): %v", hpa, err)
		}
	}

	return nil
}

func (a *refreshXPAController) Stop() error {
	close(a.stopChan)
	return nil
}
