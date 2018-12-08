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
	"k8s.io/apimachinery/pkg/api/errors"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/worker/appm/types/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type deleteController struct {
	controllerID string
	appService   []v1.AppService
	manager      *Manager
}

// Begin begins to delete k8s resources
func (d *deleteController) Begin() {
	var wait sync.WaitGroup
	for _, service := range d.appService {
		go func(service v1.AppService) {
			wait.Add(1)
			defer wait.Done()
			service.Logger.Info("App runtime begin delete app service "+service.ServiceAlias,
				getLoggerOption("deleting"))
			if err := d.deleteOne(service); err != nil {
				service.Logger.Info(fmt.Sprintf("delete service %s failure %s", service.ServiceAlias, err.Error()),
					GetCallbackLoggerOption())
				logrus.Errorf("delete service %s failure %s", service.ServiceAlias, err.Error())
			} else {
				service.Logger.Info(fmt.Sprintf("delete service %s success", service.ServiceAlias), GetLastLoggerOption())
			}
		}(service)
	}
	wait.Wait()
	d.manager.callback(d.controllerID, nil)
}

func (d *deleteController) deleteOne(app v1.AppService) error {
	if secrets := app.GetSecrets(); secrets != nil {
		for _, secret := range secrets {
			if secret != nil {
				err := d.manager.client.CoreV1().Secrets(app.TenantID).Delete(secret.Name, &metav1.DeleteOptions{})
				if err != nil && !errors.IsNotFound(err) {
					return fmt.Errorf("delete secret failure:%s", err.Error())
				}
				d.manager.store.OnDelete(secret)
			}
		}
	}
	if ingresses := app.GetIngress(); ingresses != nil {
		for _, ingress := range ingresses {
			err := d.manager.client.ExtensionsV1beta1().Ingresses(app.TenantID).Delete(ingress.Name, &metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("delete ingress failure:%s", err.Error())
			}
			d.manager.store.OnDelete(ingress)
		}
	}
	app.Logger.Info("success deleting ingresses and secrets", getLoggerOption("running"))
	return nil
}

func (d *deleteController) Stop() error {
	return nil
}
