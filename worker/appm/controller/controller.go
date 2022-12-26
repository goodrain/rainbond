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

	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/util/apply"
	"github.com/goodrain/rainbond/worker/appm/store"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//Controller service operating controller interface
type Controller interface {
	Begin()
	Stop() error
}

//TypeController controller type
type TypeController string

//TypeStartController start service type
var TypeStartController TypeController = "start"

//TypeStopController start service type
var TypeStopController TypeController = "stop"

//TypeRestartController restart service type
var TypeRestartController TypeController = "restart"

//TypeUpgradeController start service type
var TypeUpgradeController TypeController = "upgrade"

//TypeScalingController start service type
var TypeScalingController TypeController = "scaling"

// TypeApplyRuleController -
var TypeApplyRuleController TypeController = "apply_rule"

// TypeApplyConfigController -
var TypeApplyConfigController TypeController = "apply_config"

// TypeControllerRefreshHPA -
var TypeControllerRefreshHPA TypeController = "refreshhpa"

//Manager controller manager
type Manager struct {
	ctx           context.Context
	cancel        context.CancelFunc
	client        kubernetes.Interface
	runtimeClient client.Client
	apply         apply.Applicator
	controllers   map[string]Controller
	store         store.Storer
	lock          sync.Mutex
}

//NewManager new manager
func NewManager(store store.Storer, client kubernetes.Interface, runtimeClient client.Client) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		ctx:           ctx,
		cancel:        cancel,
		client:        client,
		apply:         apply.NewAPIApplicator(runtimeClient),
		runtimeClient: runtimeClient,
		controllers:   make(map[string]Controller),
		store:         store,
	}
}

//Stop stop all controller
func (m *Manager) Stop() error {
	m.cancel()
	return nil
}

//GetControllerSize get running controller number
func (m *Manager) GetControllerSize() int {
	m.lock.Lock()
	defer m.lock.Unlock()
	return len(m.controllers)
}

//ExportController -
func (m *Manager) ExportController(AppName, AppVersion string, EventIDs []string, end bool, apps ...v1.AppService) error {
	controllerID := util.NewUUID()
	controller := &exportController{
		controllerID: controllerID,
		appService:   apps,
		manager:      m,
		stopChan:     make(chan struct{}),
		ctx:          context.Background(),
		AppName:      AppName,
		AppVersion:   AppVersion,
		EventIDs:     EventIDs,
		End:          end,
	}
	m.controllers[controllerID] = controller
	controller.Begin()
	return nil
}

//StartController create and start service controller
func (m *Manager) StartController(controllerType TypeController, apps ...v1.AppService) error {
	var controller Controller
	controllerID := util.NewUUID()
	switch controllerType {
	case TypeStartController:
		controller = &startController{
			controllerID: controllerID,
			appService:   apps,
			manager:      m,
			stopChan:     make(chan struct{}),
			ctx:          context.Background(),
		}
	case TypeStopController:
		controller = &stopController{
			controllerID: controllerID,
			appService:   apps,
			manager:      m,
			stopChan:     make(chan struct{}),
			ctx:          context.Background(),
		}
	case TypeScalingController:
		controller = &scalingController{
			controllerID: controllerID,
			appService:   apps,
			manager:      m,
			stopChan:     make(chan struct{}),
		}
	case TypeUpgradeController:
		controller = &upgradeController{
			controllerID: controllerID,
			appService:   apps,
			manager:      m,
			stopChan:     make(chan struct{}),
			ctx:          context.Background(),
		}
	case TypeRestartController:
		controller = &restartController{
			controllerID: controllerID,
			appService:   apps,
			manager:      m,
			stopChan:     make(chan struct{}),
			ctx:          context.Background(),
		}
	case TypeApplyRuleController:
		controller = &applyRuleController{
			controllerID: controllerID,
			appService:   apps,
			manager:      m,
			stopChan:     make(chan struct{}),
			ctx:          context.Background(),
		}
	case TypeApplyConfigController:
		controller = &applyConfigController{
			controllerID: controllerID,
			appService:   apps[0],
			manager:      m,
			stopChan:     make(chan struct{}),
			ctx:          context.Background(),
		}
	case TypeControllerRefreshHPA:
		controller = &refreshXPAController{
			controllerID: controllerID,
			appService:   apps,
			manager:      m,
			stopChan:     make(chan struct{}),
			ctx:          context.Background(),
		}
	default:
		return fmt.Errorf("No support controller")
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	m.controllers[controllerID] = controller
	go controller.Begin()
	return nil
}

func (m *Manager) callback(controllerID string, err error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.controllers, controllerID)
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
