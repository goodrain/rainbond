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
	"container/list"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/goodrain/rainbond/api/model"
	apiutil "github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	gclient "github.com/goodrain/rainbond/mq/client"
	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/util/retryutil"
	"github.com/goodrain/rainbond/worker/client"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

//BatchOperationHandler batch operation handler
type BatchOperationHandler struct {
	mqCli            gclient.MQClient
	operationHandler *OperationHandler
	statusCli        *client.AppRuntimeSyncClient
}

//BatchOperationResult batch operation result
type BatchOperationResult struct {
	BatchResult []OperationResult `json:"batche_result"`
}

//CreateBatchOperationHandler create batch operation handler
func CreateBatchOperationHandler(mqCli gclient.MQClient, statusCli *client.AppRuntimeSyncClient, operationHandler *OperationHandler) *BatchOperationHandler {
	return &BatchOperationHandler{
		mqCli:            mqCli,
		operationHandler: operationHandler,
		statusCli:        statusCli,
	}
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
func (b *BatchOperationHandler) Build(ctx context.Context, tenant *dbmodel.Tenants, operator string, batchOpReqs model.BatchOpRequesters) (model.BatchOpResult, error) {
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		defer util.Elapsed("[BatchOperationHandler] build components")()
	}

	// setup start sequence config
	componentIDs := batchOpReqs.ComponentIDs()
	startupSeqConfigs := b.serviceStartupSequence(componentIDs)

	// check allocatable memory
	allocm, err := NewAllocMemory(ctx, b.statusCli, tenant, batchOpReqs)
	if err != nil {
		return nil, errors.WithMessage(err, "new alloc memory")
	}
	batchOpResult := allocm.BatchOpResult()
	validBuilds := allocm.BatchOpRequests()

	batchOpReqs, batchOpResult2 := b.checkEvents(batchOpReqs)
	batchOpResult = append(batchOpResult, batchOpResult2...)

	// create events
	if err := b.createEvents(tenant.UUID, operator, batchOpReqs, allocm.badOpRequest, allocm.memoryType); err != nil {
		return nil, err
	}

	for _, build := range validBuilds {
		build.UpdateConfig("boot_seq_dep_service_ids", strings.Join(startupSeqConfigs[build.GetComponentID()], ","))
		err := retryutil.Retry(1*time.Microsecond, 1, func() (bool, error) {
			if err := b.operationHandler.build(build); err != nil {
				return false, err
			}
			return true, nil
		})
		item := build.BatchOpFailureItem()
		if err != nil {
			item.ErrMsg = err.Error()
		} else {
			item.Success()
		}
		batchOpResult = append(batchOpResult, item)
	}

	return batchOpResult, nil
}

//Start batch start
func (b *BatchOperationHandler) Start(ctx context.Context, tenant *dbmodel.Tenants, operator string, batchOpReqs model.BatchOpRequesters) (model.BatchOpResult, error) {
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		defer util.Elapsed("[BatchOperationHandler] start components")()
	}

	// setup start sequence config
	componentIDs := batchOpReqs.ComponentIDs()
	startupSeqConfigs := b.serviceStartupSequence(componentIDs)

	// chekc allocatable memory
	allocm, err := NewAllocMemory(ctx, b.statusCli, tenant, batchOpReqs)
	if err != nil {
		return nil, errors.WithMessage(err, "new alloc memory")
	}
	batchOpResult := allocm.BatchOpResult()
	validRequestes := allocm.BatchOpRequests()

	batchOpReqs, batchOpResult2 := b.checkEvents(batchOpReqs)
	batchOpResult = append(batchOpResult, batchOpResult2...)

	// create events
	if err := b.createEvents(tenant.UUID, operator, batchOpReqs, allocm.BadOpRequests(), allocm.memoryType); err != nil {
		return nil, err
	}

	for _, req := range validRequestes {
		// startup sequence
		req.UpdateConfig("boot_seq_dep_service_ids", strings.Join(startupSeqConfigs[req.GetComponentID()], ","))
		err := retryutil.Retry(1*time.Microsecond, 1, func() (bool, error) {
			if err := b.operationHandler.Start(req); err != nil {
				return false, err
			}
			return true, nil
		})
		item := req.BatchOpFailureItem()
		if err != nil {
			item.ErrMsg = err.Error()
		} else {
			item.Success()
		}
		batchOpResult = append(batchOpResult, item)
	}

	return batchOpResult, nil
}

//Stop batch stop
func (b *BatchOperationHandler) Stop(ctx context.Context, tenant *dbmodel.Tenants, operator string, batchOpReqs model.BatchOpRequesters) (model.BatchOpResult, error) {
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		defer util.Elapsed("[BatchOperationHandler] stop components")()
	}

	batchOpReqs, batchOpResult := b.checkEvents(batchOpReqs)

	// create events
	if err := b.createEvents(tenant.UUID, operator, batchOpReqs, nil, ""); err != nil {
		return nil, err
	}

	for _, req := range batchOpReqs {
		err := retryutil.Retry(1*time.Microsecond, 1, func() (bool, error) {
			if err := b.operationHandler.Stop(req); err != nil {
				return false, err
			}
			return true, nil
		})
		item := req.BatchOpFailureItem()
		if err != nil {
			item.ErrMsg = err.Error()
		} else {
			item.Success()
		}
		batchOpResult = append(batchOpResult, item)
	}

	return batchOpResult, nil
}

//Upgrade batch upgrade
func (b *BatchOperationHandler) Upgrade(ctx context.Context, tenant *dbmodel.Tenants, operator string, batchOpReqs model.BatchOpRequesters) (model.BatchOpResult, error) {
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		defer util.Elapsed("[BatchOperationHandler] upgrade components")()
	}

	// setup start sequence config
	componentIDs := batchOpReqs.ComponentIDs()
	startupSeqConfigs := b.serviceStartupSequence(componentIDs)

	// chekc allocatable memory
	allocm, err := NewAllocMemory(ctx, b.statusCli, tenant, batchOpReqs)
	if err != nil {
		return nil, errors.WithMessage(err, "new alloc memory")
	}
	batchOpResult := allocm.BatchOpResult()
	validUpgrades := allocm.BatchOpRequests()

	validUpgrades, batchOpResult2 := b.checkEvents(validUpgrades)
	batchOpResult = append(batchOpResult, batchOpResult2...)

	// create events
	if err := b.createEvents(tenant.UUID, operator, batchOpReqs, allocm.BadOpRequests(), allocm.memoryType); err != nil {
		return nil, err
	}

	for _, upgrade := range validUpgrades {
		upgrade.UpdateConfig("boot_seq_dep_service_ids", strings.Join(startupSeqConfigs[upgrade.GetComponentID()], ","))
		err := retryutil.Retry(1*time.Microsecond, 1, func() (bool, error) {
			if err := b.operationHandler.upgrade(upgrade); err != nil {
				return false, err
			}
			return true, nil
		})
		item := upgrade.BatchOpFailureItem()
		if err != nil {
			item.ErrMsg = err.Error()
		} else {
			item.Success()
		}
		batchOpResult = append(batchOpResult, item)
	}
	return batchOpResult, nil
}

func (b *BatchOperationHandler) checkEvents(batchOpReqs model.BatchOpRequesters) (model.BatchOpRequesters, model.BatchOpResult) {
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		defer util.Elapsed("[BatchOperationHandler] check events")()
	}

	var validReqs model.BatchOpRequesters
	var batchOpResult model.BatchOpResult
	for _, req := range batchOpReqs {
		req := req
		if apiutil.CanDoEvent("", dbmodel.SYNEVENTTYPE, "service", req.GetComponentID(), "") {
			validReqs = append(validReqs, req)
			continue
		}
		item := req.BatchOpFailureItem()
		item.ErrMsg = "The last event has not been completed"
		batchOpResult = append(batchOpResult, item)
	}
	return validReqs, batchOpResult
}

func (b *BatchOperationHandler) createEvents(tenantID, operator string, batchOpReqs, badOpReqs model.BatchOpRequesters, memoryType string) error {
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		defer util.Elapsed("[BatchOperationHandler] create events")()
	}

	bads := make(map[string]struct{})
	for _, req := range badOpReqs {
		bads[req.GetEventID()] = struct{}{}
	}

	var events []*dbmodel.ServiceEvent
	for _, req := range batchOpReqs {
		event := &dbmodel.ServiceEvent{
			EventID:   req.GetEventID(),
			TenantID:  tenantID,
			Target:    dbmodel.TargetTypeService,
			TargetID:  req.GetComponentID(),
			UserName:  operator,
			StartTime: time.Now().Format(time.RFC3339),
			SynType:   dbmodel.ASYNEVENTTYPE,
			OptType:   req.OpType(),
		}
		_, ok := bads[req.GetEventID()]
		if ok {
			event.Reason = memoryType
			event.EndTime = event.StartTime
			event.FinalStatus = "complete"
			event.Status = "failure"

		}
		events = append(events, event)
	}

	return db.GetManager().DB().Transaction(func(tx *gorm.DB) error {
		return db.GetManager().ServiceEventDaoTransactions(tx).CreateEventsInBatch(events)
	})
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
			result = append(result, sublists...)
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

// AllocMemory represents a allocatable memory.
type AllocMemory struct {
	tenant          *dbmodel.Tenants
	allcm           *int64
	memoryType      string
	components      map[string]*dbmodel.TenantServices
	batchOpResult   model.BatchOpResult
	batchOpRequests model.BatchOpRequesters
	badOpRequest    model.BatchOpRequesters
}

// NewAllocMemory creates a new AllocMemory.
func NewAllocMemory(ctx context.Context, statusCli *client.AppRuntimeSyncClient, tenant *dbmodel.Tenants, batchOpReqs model.BatchOpRequesters) (*AllocMemory, error) {
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		defer util.Elapsed("[NewAllocMemory] check allocatable memory")()
	}

	am := &AllocMemory{
		tenant: tenant,
	}

	if tenant.LimitMemory != 0 {
		tenantUsedResource, err := statusCli.GetTenantResource(tenant.UUID)
		if err != nil {
			return nil, err
		}
		allocm := tenant.LimitMemory - int(tenantUsedResource.MemoryLimit)
		am.allcm = util.Int64(int64(allocm))
		am.memoryType = "tenant_lack_of_memory"
	} else {
		allcm, err := ClusterAllocMemory(ctx)
		if err != nil {
			return nil, err
		}
		am.allcm = util.Int64(allcm)
		am.memoryType = "cluster_lack_of_memory"
	}

	components, err := am.listComponents(batchOpReqs.ComponentIDs())
	if err != nil {
		return nil, err
	}
	am.components = components

	// check alloc memory for every components.
	var reqs model.BatchOpRequesters
	var batchOpResult model.BatchOpResult
	var badOpRequest model.BatchOpRequesters
	for _, req := range batchOpReqs {
		req := req
		if err := am.check(req.GetComponentID()); err != nil {
			item := req.BatchOpFailureItem()
			item.ErrMsg = err.Error()
			batchOpResult = append(batchOpResult, item)
			badOpRequest = append(badOpRequest, req)
			continue
		}
		reqs = append(reqs, req)
	}
	am.batchOpResult = batchOpResult
	am.batchOpRequests = reqs
	am.badOpRequest = badOpRequest

	return am, nil
}

// BatchOpResult returns the batchOpResult.
func (a *AllocMemory) BatchOpResult() model.BatchOpResult {
	return a.batchOpResult
}

// BatchOpRequests returns the batchOpRequests.
func (a *AllocMemory) BatchOpRequests() model.BatchOpRequesters {
	return a.batchOpRequests
}

// BadOpRequests returns the badOpRequests.
func (a *AllocMemory) BadOpRequests() model.BatchOpRequesters {
	return a.badOpRequest
}

func (a *AllocMemory) listComponents(componentIDs []string) (map[string]*dbmodel.TenantServices, error) {
	components, err := db.GetManager().TenantServiceDao().GetServiceByIDs(componentIDs)
	if err != nil {
		return nil, err
	}

	// make a map for compoenents
	res := make(map[string]*dbmodel.TenantServices)
	for _, cpt := range components {
		cpt := cpt
		res[cpt.ServiceID] = cpt
	}
	return res, nil
}

func (a *AllocMemory) check(componentID string) error {
	component, ok := a.components[componentID]
	if !ok {
		return errors.New("component not found")
	}
	requestMemory := component.ContainerMemory * component.Replicas

	allom := util.Int64Value(a.allcm)
	if requestMemory > int(allom) {
		logrus.Errorf("request memory is %d, but got %d allocatable memory", requestMemory, allom)
		return errors.New("tenant_lack_of_memory")
	}

	*a.allcm -= int64(requestMemory)

	return nil
}
