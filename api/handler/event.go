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
	"time"

	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/sirupsen/logrus"
)

// ServiceEventHandler -
type ServiceEventHandler struct {
}

// NewServiceEventHandler -
func NewServiceEventHandler() *ServiceEventHandler {
	return &ServiceEventHandler{}
}

// ListByEventIDs -
func (s *ServiceEventHandler) ListByEventIDs(eventIDs []string) ([]*dbmodel.ServiceEvent, error) {
	events, err := db.GetManager().ServiceEventDao().GetEventByEventIDs(eventIDs)
	if err != nil {
		return nil, err
	}

	// timeout events
	var timeoutEvents []*dbmodel.ServiceEvent
	for _, event := range events {
		if !s.isTimeout(event) {
			continue
		}
		event.Status = "timeout"
		event.FinalStatus = "complete"
		timeoutEvents = append(timeoutEvents, event)
	}

	return events, db.GetManager().ServiceEventDao().UpdateInBatch(timeoutEvents)
}

func (s *ServiceEventHandler) isTimeout(event *dbmodel.ServiceEvent) bool {
	if event.FinalStatus != "" {
		return false
	}

	startTime, err := time.ParseInLocation(time.RFC3339, event.StartTime, time.Local)
	if err != nil {
		logrus.Errorf("[ServiceEventHandler] [isTimeout] parse start time(%s): %v", event.StartTime, err)
		return false
	}

	if event.OptType == "deploy" || event.OptType == "create" || event.OptType == "build" || event.OptType == "upgrade" {
		end := startTime.Add(3 * time.Minute)
		if time.Now().After(end) {
			return true
		}
	} else {
		end := startTime.Add(30 * time.Second)
		if time.Now().After(end) {
			return true
		}
	}

	return false
}
