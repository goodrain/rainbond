// RAINBOND, Application Management Platform
// Copyright (C) 2014-2019 Goodrain Co., Ltd.

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

package handler

import (
	"fmt"
	"strings"

	"container/list"
	"github.com/sirupsen/logrus"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	gclient "github.com/goodrain/rainbond/mq/client"
)

//BatchOperationHandler batch operation handler
type BatchOperationHandler struct {
	mqCli            gclient.MQClient
	operationHandler *OperationHandler
}

//BatchOperationResult batch operation result
type BatchOperationResult struct {
	BatchResult []OperationResult `json:"batche_result"`
}

//CreateBatchOperationHandler create batch operation handler
func CreateBatchOperationHandler(mqCli gclient.MQClient, operationHandler *OperationHandler) *BatchOperationHandler {
	return &BatchOperationHandler{
		mqCli:            mqCli,
		operationHandler: operationHandler,
	}
}

func setStartupSequenceConfig(configs map[string]string, depsids []string) map[string]string {
	if configs == nil {
		configs = make(map[string]string, 1)
	}
	configs["boot_seq_dep_service_ids"] = strings.Join(depsids, ",")
	return configs
}

func checkResourceEnough(serviceID string) error {
	service, err := db.GetManager().TenantServiceDao().GetServiceByID(serviceID)
	if err != nil {
		logrus.Errorf("get service by id error, %v", err)
		return err
	}
	tenant, err := db.GetManager().TenantDao().GetTenantByUUID(service.TenantID)
	if err != nil {
		logrus.Errorf("get tenant by id error: %v", err)
		return err
	}

	return CheckTenantResource(tenant, service.ContainerMemory*service.Replicas)
}

func (b *BatchOperationHandler) serviceStartupSequence(serviceIDs []string) map[string][]string {
	sd, err := NewServiceDependency(serviceIDs)
	if err != nil {
		logrus.Warningf("create a new ServiceDependency: %v", err)
	}
	startupSeqConfigs := sd.serviceStartupSequence()
	logrus.Debugf("startup sequence configurations: %#v", startupSeqConfigs)
	return startupSeqConfigs
}

//Build build
func (b *BatchOperationHandler) Build(buildInfos []model.BuildInfoRequestStruct) (re BatchOperationResult) {
	var serviceIDs []string
	for _, info := range buildInfos {
		serviceIDs = append(serviceIDs, info.ServiceID)
	}
	startupSeqConfigs := b.serviceStartupSequence(serviceIDs)

	var retrys []model.BuildInfoRequestStruct
	for _, buildInfo := range buildInfos {
		if err := checkResourceEnough(buildInfo.ServiceID); err != nil {
			re.BatchResult = append(re.BatchResult, OperationResult{
				ServiceID:     buildInfo.ServiceID,
				Operation:     "build",
				EventID:       buildInfo.EventID,
				Status:        "failure",
				ErrMsg:        err.Error(),
				DeployVersion: "",
			})
			continue
		}
		buildInfo.Configs = setStartupSequenceConfig(buildInfo.Configs, startupSeqConfigs[buildInfo.ServiceID])
		buildre := b.operationHandler.Build(buildInfo)
		if buildre.Status != "success" {
			retrys = append(retrys, buildInfo)
		} else {
			re.BatchResult = append(re.BatchResult, buildre)
		}
	}
	for _, retry := range retrys {
		re.BatchResult = append(re.BatchResult, b.operationHandler.Build(retry))
	}
	return
}

//Start batch start
func (b *BatchOperationHandler) Start(startInfos []model.StartOrStopInfoRequestStruct) (re BatchOperationResult) {
	var serviceIDs []string
	for _, info := range startInfos {
		serviceIDs = append(serviceIDs, info.ServiceID)
	}
	startupSeqConfigs := b.serviceStartupSequence(serviceIDs)

	var retrys []model.StartOrStopInfoRequestStruct
	for _, startInfo := range startInfos {
		if err := checkResourceEnough(startInfo.ServiceID); err != nil {
			re.BatchResult = append(re.BatchResult, OperationResult{
				ServiceID:     startInfo.ServiceID,
				Operation:     "start",
				EventID:       startInfo.EventID,
				Status:        "failure",
				ErrMsg:        err.Error(),
				DeployVersion: "",
			})
			continue
		}

		// startup sequence
		startInfo.Configs = setStartupSequenceConfig(startInfo.Configs, startupSeqConfigs[startInfo.ServiceID])
		startre := b.operationHandler.Start(startInfo)
		if startre.Status != "success" {
			retrys = append(retrys, startInfo)
		} else {
			re.BatchResult = append(re.BatchResult, startre)
		}
	}
	for _, retry := range retrys {
		re.BatchResult = append(re.BatchResult, b.operationHandler.Start(retry))
	}
	return
}

//Stop batch stop
func (b *BatchOperationHandler) Stop(stopInfos []model.StartOrStopInfoRequestStruct) (re BatchOperationResult) {
	var retrys []model.StartOrStopInfoRequestStruct
	for _, stopInfo := range stopInfos {
		stopre := b.operationHandler.Stop(stopInfo)
		if stopre.Status != "success" {
			retrys = append(retrys, stopInfo)
		} else {
			re.BatchResult = append(re.BatchResult, stopre)
		}
	}
	for _, retry := range retrys {
		re.BatchResult = append(re.BatchResult, b.operationHandler.Stop(retry))
	}
	return
}

//Upgrade batch upgrade
func (b *BatchOperationHandler) Upgrade(upgradeInfos []model.UpgradeInfoRequestStruct) (re BatchOperationResult) {
	var serviceIDs []string
	for _, info := range upgradeInfos {
		serviceIDs = append(serviceIDs, info.ServiceID)
	}
	startupSeqConfigs := b.serviceStartupSequence(serviceIDs)

	var retrys []model.UpgradeInfoRequestStruct
	for _, upgradeInfo := range upgradeInfos {
		if err := checkResourceEnough(upgradeInfo.ServiceID); err != nil {
			re.BatchResult = append(re.BatchResult, OperationResult{
				ServiceID:     upgradeInfo.ServiceID,
				Operation:     "upgrade",
				EventID:       upgradeInfo.EventID,
				Status:        "failure",
				ErrMsg:        err.Error(),
				DeployVersion: "",
			})
			continue
		}
		upgradeInfo.Configs = setStartupSequenceConfig(upgradeInfo.Configs, startupSeqConfigs[upgradeInfo.ServiceID])
		stopre := b.operationHandler.Upgrade(upgradeInfo)
		if stopre.Status != "success" {
			retrys = append(retrys, upgradeInfo)
		} else {
			re.BatchResult = append(re.BatchResult, stopre)
		}
	}
	for _, retry := range retrys {
		re.BatchResult = append(re.BatchResult, b.operationHandler.Upgrade(retry))
	}
	return
}

// ServiceDependency documents a set of services and their dependencies.
// provides the ability to build linked lists of dependencies and find circular dependencies.
type ServiceDependency struct {
	serviceIDs  []string
	sid2depsids map[string][]string
	depsid2sids map[string][]string
}

// NewServiceDependency creates a new ServiceDependency.
func NewServiceDependency(serviceIDs []string) (*ServiceDependency, error) {
	relations, err := db.GetManager().TenantServiceRelationDao().ListByServiceIDs(serviceIDs)
	if err != nil {
		return nil, fmt.Errorf("list retions: %v", err)
	}
	sid2depsids := make(map[string][]string)
	depsid2sids := make(map[string][]string)
	for _, relation := range relations {
		sid2depsids[relation.ServiceID] = append(sid2depsids[relation.ServiceID], relation.DependServiceID)
		depsid2sids[relation.DependServiceID] = append(depsid2sids[relation.DependServiceID], relation.ServiceID)
	}

	logrus.Debugf("create a new ServiceDependency; sid2depsids: %#v; depsid2sids: %#v", sid2depsids, depsid2sids)
	return &ServiceDependency{
		serviceIDs:  serviceIDs,
		sid2depsids: sid2depsids,
		depsid2sids: depsid2sids,
	}, nil
}

// The order in which services are started is determined by their dependencies. If interdependencies occur, one of them is ignored.
func (s *ServiceDependency) serviceStartupSequence() map[string][]string {
	headNodes := s.headNodes()
	var lists []*list.List
	for _, h := range headNodes {
		l := list.New()
		l.PushBack(h)
		lists = append(lists, s.buildLinkListByHead(l)...)
	}

	result := make(map[string][]string)
	for _, l := range lists {
		cur := l.Front()
		for cur != nil && cur.Next() != nil {
			existingVals := result[cur.Value.(string)]
			exists := false
			for _, val := range existingVals {
				if val == cur.Next().Value.(string) {
					exists = true
					break
				}
			}
			if !exists {
				result[cur.Value.(string)] = append(result[cur.Value.(string)], cur.Next().Value.(string))
			}
			cur = cur.Next()
		}
	}

	return result
}

// headNodes finds out the service ID of all head nodes. The head nodes are services that are not dependent on other services.
func (s *ServiceDependency) headNodes() []string {
	var headNodes []string
	for _, sid := range s.serviceIDs {
		if _, ok := s.depsid2sids[sid]; ok {
			continue
		}

		headNodes = append(headNodes, sid)
	}

	// if there is no head node(i.e. a->b->c->d->a), then a node is randomly selected.
	// however, this node cannot be a tail node
	for _, sid := range s.serviceIDs {
		// does not depend on other services, it is the tail node
		if _, ok := s.sid2depsids[sid]; !ok {
			continue
		}

		headNodes = append(headNodes, sid)
		logrus.Debugf("randomly select '%s' as the head node", sid)
		break
	}

	return headNodes
}

// buildLinkListByHead recursively creates linked lists of service dependencies.
//
// recursive end condition:
// 1. nil or empty input
// 2. no more children
// 3. child node is already in the linked list
func (s *ServiceDependency) buildLinkListByHead(l *list.List) []*list.List {
	// nil or empty input
	if l == nil || l.Len() == 0 {
		return nil
	}

	// the last node is the head node of the new linked list
	sid, _ := l.Back().Value.(string)
	depsids, ok := s.sid2depsids[sid]
	// no more children
	if !ok {
		copy := list.New()
		copy.PushBackList(l)
		return []*list.List{copy}
	}

	var result []*list.List
	for _, depsid := range depsids {
		// child node is already in the linked list
		if alreadyInLinkedList(l, depsid) || s.childInLinkedList(l, depsid) {
			copy := list.New()
			copy.PushBackList(l)
			result = append(result, copy)
			continue
		}

		newl := list.New()
		newl.PushBackList(l)
		newl.PushBack(depsid)

		sublists := s.buildLinkListByHead(newl)
		if len(sublists) == 0 {
			result = append(result, newl)
		} else {
			for _, sublist := range sublists {
				result = append(result, sublist)
			}
		}
	}

	return result
}

func (s *ServiceDependency) childInLinkedList(l *list.List, sid string) bool {
	depsids, ok := s.sid2depsids[sid]
	if !ok {
		return false
	}

	for _, depsid := range depsids {
		if alreadyInLinkedList(l, depsid) {
			return true
		}
	}

	return false
}

func alreadyInLinkedList(l *list.List, depsid string) bool {
	pre := l.Back()
	for pre != nil {
		val := pre.Value.(string)
		if val == depsid {
			return true
		}
		pre = pre.Prev()
	}

	return false
}
