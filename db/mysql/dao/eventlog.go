// Copyright (C) 2014-2018 Goodrain Co., Ltd.
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

package dao

import (
	"github.com/goodrain/rainbond/db/model"

	"github.com/jinzhu/gorm"
)

//EventLogMessageDaoImpl EventLogMessageDaoImpl
type EventLogMessageDaoImpl struct {
	DB *gorm.DB
}

//AddModel AddModel
func (e *EventLogMessageDaoImpl) AddModel(mo model.Interface) error {
	return nil
}

//UpdateModel UpdateModel
func (e *EventLogMessageDaoImpl) UpdateModel(mo model.Interface) error {
	return nil
}

//GetEventLogMessages get event log message
func (e *EventLogMessageDaoImpl) GetEventLogMessages(eventID string) ([]*model.EventLogMessage, error) {
	var messageRaw []*model.EventLogMessage
	if err := e.DB.Where("event_id=?", eventID).Find(&messageRaw).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return messageRaw, nil
		}
		return nil, err
	}
	return messageRaw, nil
}

//DeleteServiceLog TODO:
func (e *EventLogMessageDaoImpl) DeleteServiceLog(serviceID string) error {
	return nil
}

func (e *EventLogMessageDaoImpl) GetAllServiceEventLog() ([]*model.EventLogMessage, error) {
	var messageRaw []*model.EventLogMessage
	if err := e.DB.Find(&messageRaw).Error; err != nil {
		return nil, err
	}
	return messageRaw, nil

}

func (e *EventLogMessageDaoImpl) DeleteServiceEventLog(obj *model.EventLogMessage) error {
	if err := e.DB.Delete(obj).Error; err != nil {
		return err
	}
	return nil
}

func (e *EventLogMessageDaoImpl) DeleteServiceEventLogByEventId(eventId string) error {
	var messageRaw []*model.EventLogMessage
	isNotExist := e.DB.Where("event_id=?", eventId).First(&messageRaw).RecordNotFound()
	if isNotExist{
		return nil
	}
	if err := e.DB.Where("event_id=?", eventId).Delete(&messageRaw).Error;err!=nil{
		return err
	}
	return nil
}
