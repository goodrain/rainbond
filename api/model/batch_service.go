// Copyright (C) 2014-2021 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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

package model

import (
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	wmodel "github.com/goodrain/rainbond/worker/discover/model"
)

var _ ComponentOpReq = &ComponentStartReq{}
var _ ComponentOpReq = &ComponentStopReq{}
var _ ComponentOpReq = &ComponentBuildReq{}
var _ ComponentOpReq = &ComponentUpgradeReq{}

// BatchOpRequesters -
type BatchOpRequesters []ComponentOpReq

// ComponentIDs returns a list of components ids.
func (b BatchOpRequesters) ComponentIDs() []string {
	var componentIDs []string
	for _, item := range b {
		componentIDs = append(componentIDs, item.GetComponentID())
	}
	return componentIDs
}

// ComponentOpReq -
type ComponentOpReq interface {
	GetComponentID() string
	GetEventID() string
	TaskBody(component *dbmodel.TenantServices) interface{}
	BatchOpFailureItem() *ComponentOpResult
	UpdateConfig(key, value string)
	OpType() string
	SetVersion(version string)
	GetVersion() string
}

// BatchOpResult -
type BatchOpResult []*ComponentOpResult

// BatchOpResultItemStatus is the status of ComponentOpResult.
type BatchOpResultItemStatus string

// BatchOpResultItemStatus -
var (
	BatchOpResultItemStatusFailure BatchOpResultItemStatus = "failure"
	BatchOpResultItemStatusSuccess BatchOpResultItemStatus = "success"
)

// ComponentOpResult -
type ComponentOpResult struct {
	ServiceID     string                  `json:"service_id"`
	Operation     string                  `json:"operation"`
	EventID       string                  `json:"event_id"`
	Status        BatchOpResultItemStatus `json:"status"`
	ErrMsg        string                  `json:"err_message"`
	DeployVersion string                  `json:"deploy_version"`
}

// Success sets the status to success.
func (b *ComponentOpResult) Success() {
	b.Status = BatchOpResultItemStatusSuccess
}

// ComponentOpGeneralReq -
type ComponentOpGeneralReq struct {
	EventID   string            `json:"event_id"`
	ServiceID string            `json:"service_id"`
	Configs   map[string]string `json:"configs"`
	// When determining the startup sequence of services, you need to know the services they depend on
	DepServiceIDInBootSeq []string `json:"dep_service_ids_in_boot_seq"`
}

// UpdateConfig -
func (b *ComponentOpGeneralReq) UpdateConfig(key, value string) {
	if b.Configs == nil {
		b.Configs = make(map[string]string)
	}
	b.Configs[key] = value
}

// ComponentStartReq -
type ComponentStartReq struct {
	ComponentOpGeneralReq
}

// GetEventID -
func (s *ComponentStartReq) GetEventID() string {
	if s.EventID == "" {
		s.EventID = util.NewUUID()
	}
	return s.EventID
}

// GetVersion -
func (s *ComponentStartReq) GetVersion() string {
	return ""
}

// SetVersion -
func (s *ComponentStartReq) SetVersion(string) {
	// no need
}

// GetComponentID -
func (s *ComponentStartReq) GetComponentID() string {
	return s.ServiceID
}

// TaskBody -
func (s *ComponentStartReq) TaskBody(cpt *dbmodel.TenantServices) interface{} {
	return &wmodel.StartTaskBody{
		TenantID:              cpt.TenantID,
		ServiceID:             cpt.ServiceID,
		DeployVersion:         cpt.DeployVersion,
		EventID:               s.GetEventID(),
		Configs:               s.Configs,
		DepServiceIDInBootSeq: s.DepServiceIDInBootSeq,
	}
}

// OpType -
func (s *ComponentStartReq) OpType() string {
	return "start-service"
}

// BatchOpFailureItem -
func (s *ComponentStartReq) BatchOpFailureItem() *ComponentOpResult {
	return &ComponentOpResult{
		ServiceID: s.ServiceID,
		EventID:   s.GetEventID(),
		Operation: "start",
		Status:    BatchOpResultItemStatusFailure,
	}
}

// ComponentStopReq -
type ComponentStopReq struct {
	ComponentStartReq
}

// OpType -
func (s *ComponentStopReq) OpType() string {
	return "stop-service"
}

// BatchOpFailureItem -
func (s *ComponentStopReq) BatchOpFailureItem() *ComponentOpResult {
	return &ComponentOpResult{
		ServiceID: s.ServiceID,
		EventID:   s.GetEventID(),
		Operation: "stop",
		Status:    BatchOpResultItemStatusFailure,
	}
}
