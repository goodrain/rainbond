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
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/worker/appm/conversion"
	"github.com/goodrain/rainbond/worker/appm/types/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sync"
)

type applyRuleController struct {
	controllerID string
	appService   []v1.AppService
	manager      *Manager
	stopChan     chan struct{}
}

// Begin begins applying rule
func (a *applyRuleController) Begin() {
	var wait sync.WaitGroup
	for _, service := range a.appService {
		go func(service v1.AppService) {
			wait.Add(1)
			defer wait.Done()
			if err := a.applyRules(&service); err != nil {
				logrus.Errorf("apply rules for service %s failure: %s", service.ServiceAlias, err.Error())
			}
		}(service)
	}
	wait.Wait()
	a.manager.callback(a.controllerID, nil)
}

func (a *applyRuleController) applyRules(app *v1.AppService) error {
	// delete old ingresses and secrets
	if err := a.deleteIngSec(app); err != nil {
		return err
	}
	// create new ingresses and secretes
	err := conversion.TenantServiceRegist(app, db.GetManager())
	if err != nil {
		return err
	}
	if err := a.createIngSec(app); err != nil {
		return err
	}
	//regist new app service
	a.manager.store.RegistAppService(app)
	return nil
}

// deleteIngSec deletes ingresses and secrets
func (a *applyRuleController) deleteIngSec(app *v1.AppService) error {
	// delete ingresses
	if ings := app.GetIngress(); ings != nil {
		for _, ing := range ings {
			err := a.manager.client.ExtensionsV1beta1().Ingresses(ing.Namespace).Delete(ing.Name, &metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}
	// delete secrets
	if secrets := app.GetSecrets(); secrets != nil {
		for _, secret := range secrets {
			if secret != nil {
				err := a.manager.client.CoreV1().Secrets(app.TenantID).Delete(secret.Name, &metav1.DeleteOptions{})
				if err != nil && !errors.IsNotFound(err) {
					return err
				}
			}
		}
	}
	return nil
}

// createIngSec creates create ingresses and secrets
func (a *applyRuleController) createIngSec(app *v1.AppService) error {
	if ings := app.GetIngress(); ings != nil {
		for _, ing := range ings {
			_, err := a.manager.client.ExtensionsV1beta1().Ingresses(app.TenantID).Create(ing)
			if err != nil {
				return err
			}
		}
	}
	if secrets := app.GetSecrets(); secrets != nil {
		for _, secr := range secrets {
			_, err := a.manager.client.CoreV1().Secrets(app.TenantID).Create(secr)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *applyRuleController) Stop() error {
	close(a.stopChan)
	return nil
}
