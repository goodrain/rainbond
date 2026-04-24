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
	"math"
	"sync"
	"time"

	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
)

type scalingController struct {
	controllerID string
	appService   []v1.AppService
	manager      *Manager
	stopChan     chan struct{}
}

//Begin  start handle service scaling
func (s *scalingController) Begin() {
	var wait sync.WaitGroup
	for _, service := range s.appService {
		go func(service v1.AppService) {
			wait.Add(1)
			defer wait.Done()
			service.Logger.Info("App runtime begin horizontal scaling app service "+service.ServiceAlias, event.GetLoggerOption("starting"))
			if err := s.scalingOne(service); err != nil {
				service.Logger.Error(util.Translation("horizontal scaling service error"), event.GetCallbackLoggerOption())
				logrus.Errorf("horizontal scaling service %s failure %s", service.ServiceAlias, err.Error())
			} else {
				service.Logger.Info(fmt.Sprintf("horizontal scaling service %s success", service.ServiceAlias), event.GetLastLoggerOption())
			}
		}(service)
	}
	wait.Wait()
	s.manager.callback(s.controllerID, nil)
}

//Replicas petch replicas to n
func Replicas(n int) []byte {
	return []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, n))
}

func (s *scalingController) scalingOne(service v1.AppService) error {
	if service.ServiceType == v1.TypeKubeBlocks {
		return nil
	}
	if statefulset := service.GetStatefulSet(); statefulset != nil {
		_, err := s.manager.client.AppsV1().StatefulSets(statefulset.Namespace).Patch(
			context.Background(),
			statefulset.Name,
			types.StrategicMergePatchType,
			Replicas(int(service.Replicas)),
			metav1.PatchOptions{},
		)
		if err != nil {
			logrus.Error("patch statefulset info error.", err.Error())
			return err
		}
	}
	if deployment := service.GetDeployment(); deployment != nil {
		_, err := s.manager.client.AppsV1().Deployments(deployment.Namespace).Patch(
			context.Background(),
			deployment.Name,
			types.StrategicMergePatchType,
			Replicas(int(service.Replicas)),
			metav1.PatchOptions{},
		)
		if err != nil {
			logrus.Error("patch deployment info error.", err.Error())
			return err
		}
	}
	// No longer wait for scaling ready - let probe health detection handle it
	return nil
}

//WaitingReady wait app start or upgrade ready
func (s *scalingController) WaitingReady(app v1.AppService) error {
	storeAppService := s.manager.store.GetAppService(app.ServiceID)
	var initTime int32
	if podt := app.GetPodTemplate(); podt != nil {
		if probe := podt.Spec.Containers[0].ReadinessProbe; probe != nil {
			initTime = probe.InitialDelaySeconds
		}
	}
	//at least waiting time is 40 second
	initTime += 40
	waitingReplicas := math.Abs(float64(storeAppService.Replicas) - float64(storeAppService.GetReadyReplicas()))
	timeout := time.Duration(initTime * int32(waitingReplicas))
	if timeout.Seconds() < 40 {
		timeout = time.Duration(time.Second * 40)
	}
	if err := WaitReady(s.manager.store, storeAppService, timeout, app.Logger, s.stopChan); err != nil {
		return err
	}
	return nil
}
func (s *scalingController) Stop() error {
	close(s.stopChan)

	return nil
}
