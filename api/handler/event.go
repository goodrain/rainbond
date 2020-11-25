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

	"github.com/jinzhu/gorm"

	"time"

	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	tutil "github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
)

//TIMELAYOUT timelayout
const TIMELAYOUT = "2006-01-02T15:04:05"

//ErrEventIsRuning  last event is running
var ErrEventIsRuning = fmt.Errorf("event is running")

//ErrEventIDIsExist event id is exist
var ErrEventIDIsExist = fmt.Errorf("event_id is exist")

func createEvent(eventID, serviceID, optType, tenantID string) (*dbmodel.ServiceEvent, error) {
	if eventID == "" {
		eventID = tutil.NewUUID()
	}
	event := dbmodel.ServiceEvent{}
	event.EventID = eventID
	event.ServiceID = serviceID
	event.OptType = optType
	event.TenantID = tenantID
	now := time.Now()
	timeNow := now.Format(TIMELAYOUT)
	event.StartTime = timeNow
	event.UserName = "system"
	events, err := db.GetManager().ServiceEventDao().GetEventByServiceID(serviceID)
	if err != nil && err.Error() != gorm.ErrRecordNotFound.Error() {
		return nil, err
	}
	err = checkCanAddEvent(serviceID, event.EventID, events)
	if err != nil {
		logrus.Errorf("error check event %s", err.Error())
		return nil, err
	}
	if err := db.GetManager().ServiceEventDao().AddModel(&event); err != nil {
		return nil, err
	}
	return &event, nil
}

func checkCanAddEvent(serviceID, eventID string, existEvents []*dbmodel.ServiceEvent) error {
	if len(existEvents) == 0 {
		return nil
	}
	latestEvent := existEvents[0]
	if latestEvent.EventID == eventID {
		return ErrEventIDIsExist
	}
	if latestEvent.FinalStatus == "" {
		//未完成
		timeOut, err := checkEventTimeOut(latestEvent)
		if err != nil {
			return err
		}
		logrus.Debugf("event %s timeOut %v", latestEvent.EventID, timeOut)
		if timeOut {
			return nil
		}
		return ErrEventIsRuning
	}
	return nil
}
func checkEventTimeOut(event *dbmodel.ServiceEvent) (bool, error) {
	startTime := event.StartTime
	start, err := time.ParseInLocation(TIMELAYOUT, startTime, time.Local)
	if err != nil {
		return true, err
	}
	if event.OptType == "deploy" || event.OptType == "create" || event.OptType == "build" || event.OptType == "upgrade" {
		end := start.Add(3 * time.Minute)
		if time.Now().After(end) {
			event.FinalStatus = "timeout"
			err = db.GetManager().ServiceEventDao().UpdateModel(event)
			return true, err
		}
	} else {
		end := start.Add(30 * time.Second)
		if time.Now().After(end) {
			event.FinalStatus = "timeout"
			err = db.GetManager().ServiceEventDao().UpdateModel(event)
			return true, err
		}
	}
	return false, nil
}
