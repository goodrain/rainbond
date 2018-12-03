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
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

func (a *applyRuleController) Stop() error {
	close(a.stopChan)
	return nil
}

func (a *applyRuleController) applyRules(app *v1.AppService) error {
	// update service
	for _, service := range app.GetServices() {
		ensureService(service, a.manager.client)
	}
	// update ingress
	logrus.Debugf("Ingresses: %v", app.GetIngress())
	for _, ing := range app.GetIngress() {
		ensureIngress(ing, a.manager.client)
	}
	// update secret
	logrus.Debugf("Secrets: %v", app.GetSecrets())
	for _, secret := range app.GetSecrets() {
		ensureSecret(secret, a.manager.client)
	}
	return nil
}

func ensureService(new *corev1.Service, clientSet kubernetes.Interface) {
	old, err := clientSet.CoreV1().Services(new.Namespace).Get(new.Name, metav1.GetOptions{})
	logrus.Debugf("old service: %v", old)
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			s, err := clientSet.CoreV1().Services(new.Namespace).Create(new)
			if err != nil {
				logrus.Warningf("error creating service %+v: %v", new, err)
				return
			}
			logrus.Debugf("Successfully created service: %v", s)
			return
		}
		logrus.Errorf("error getting service(%s): %v", fmt.Sprintf("%s/%s", new.Namespace, new.Name), err)
		return
	}
	new.ResourceVersion = old.ResourceVersion
	new.Spec.ClusterIP = old.Spec.ClusterIP
	s, err := clientSet.CoreV1().Services(new.Namespace).Update(new)
	if err != nil {
		logrus.Warningf("error updating service %+v: %v", new, err)
		return
	}
	logrus.Debugf("Successfully updated service: %v", s)
}

func ensureIngress(ingress *extensions.Ingress, clientSet kubernetes.Interface) {
	_, err := clientSet.ExtensionsV1beta1().Ingresses(ingress.Namespace).Update(ingress)

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			i, err := clientSet.ExtensionsV1beta1().Ingresses(ingress.Namespace).Create(ingress)
			if err != nil {
				logrus.Warningf("error creating ingress %+v: %v", ingress, err)
			}
			logrus.Debugf("Successfully created ingress: %v", i)
			return
		}

		logrus.Warningf("error updating ingress %+v: %v", ingress, err)
	}
}

func ensureSecret(secret *corev1.Secret, clientSet kubernetes.Interface) {
	_, err := clientSet.CoreV1().Secrets(secret.Namespace).Update(secret)

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			s, err := clientSet.CoreV1().Secrets(secret.Namespace).Create(secret)
			if err != nil {
				logrus.Warningf("error creating secret %+v: %v", secret, err)
			}
			logrus.Debugf("Successfully created secret: %v", s)
			return
		}

		logrus.Warningf("error updating secret %+v: %v", secret, err)
	}
}
