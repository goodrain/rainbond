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

	"github.com/goodrain/rainbond/worker/appm/store"

	"github.com/goodrain/rainbond/util"

	"k8s.io/client-go/kubernetes"

	"github.com/goodrain/rainbond/event"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
)

//Controller service operating controller interface
type Controller interface {
	Begin() error
	Stop() error
}

//TypeController controller type
type TypeController string

//TypeStartController start service type
var TypeStartController TypeController = "start"

//TypeStopController start service type
var TypeStopController TypeController = "stop"

//TypeUpgradeController start service type
var TypeUpgradeController TypeController = "upgrade"

//TypeScalingController start service type
var TypeScalingController TypeController = "scaling"

//Manager controller manager
type Manager struct {
	ctx         context.Context
	cancel      context.CancelFunc
	client      kubernetes.Clientset
	controllers map[string]Controller
	store       store.Storer
	lock        sync.Mutex
}

//GetController get start service controller
func (m *Manager) GetController(controllerType TypeController, eventLogger event.Logger, apps ...v1.AppService) (Controller, error) {
	var controller Controller
	controllerID := util.NewUUID()
	switch controllerType {
	case TypeStartController:
		controller = &startController{
			controllerID: controllerID,
			appService:   apps,
			manager:      m,
		}
	default:
		return nil, fmt.Errorf("No support controller")
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	m.controllers[controllerID] = controller
	return controller, nil
}
func (m *Manager) callbcak(controllerID string, err error) {

}

func getLoggerOption(status string) map[string]string {
	return map[string]string{"step": "appruntime", "status": status}
}

type sequencelist []sequence
type sequence []*v1.AppService

func (s *sequencelist) Contains(id string) bool {
	for _, l := range *s {
		for _, l2 := range l {
			if l2.ServiceID == id {
				return true
			}
		}
	}
	return false
}
func (s *sequencelist) Add(ids []*v1.AppService) {
	*s = append(*s, ids)
}

func foundsequence(source map[string]*v1.AppService, sl *sequencelist) {
	if len(source) == 0 {
		return
	}
	var deleteKey []string
source:
	for _, s := range source {
		for _, d := range s.Dependces {
			if !sl.Contains(d) {
				continue source
			}
		}
		deleteKey = append(deleteKey, s.ServiceID)
	}
	var list []*v1.AppService
	for _, d := range deleteKey {
		list = append(list, source[d])
		delete(source, d)
	}
	sl.Add(list)
	foundsequence(source, sl)
}

func decisionSequence(appService []*v1.AppService) sequencelist {
	var sourceIDs = make(map[string]*v1.AppService, len(appService))
	for _, a := range appService {
		sourceIDs[a.ServiceID] = a
	}
	var sl sequencelist
	foundsequence(sourceIDs, &sl)
	return sl
}
