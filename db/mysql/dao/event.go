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
	"encoding/json"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

//AddModel AddModel
func (c *EventDaoImpl) AddModel(mo model.Interface) error {
	result := mo.(*model.ServiceEvent)
	var oldResult model.ServiceEvent
	if ok := c.DB.Where("event_id=?", result.EventID).Find(&oldResult).RecordNotFound(); ok {
		if err := c.DB.Create(result).Error; err != nil {
			return err
		}
	} else {
		logrus.Infoln("event result is exist")
		return c.UpdateModel(mo)
	}
	return nil
}

//UpdateModel UpdateModel
func (c *EventDaoImpl) UpdateModel(mo model.Interface) error {
	result := mo.(*model.ServiceEvent)

	var oldResult model.ServiceEvent
	if ok := c.DB.Where("event_id=?", result.EventID).Find(&oldResult).RecordNotFound(); !ok {
		finalUpdateEvent(result, &oldResult)
		oldB, _ := json.Marshal(oldResult)
		logrus.Infof("update event to %s", string(oldB))
		if err := c.DB.Save(&oldResult).Error; err != nil {
			return err
		}
	}
	return nil
}
func finalUpdateEvent(target *model.ServiceEvent, old *model.ServiceEvent) {
	if target.CodeVersion != "" {
		old.CodeVersion = target.CodeVersion
	}
	if target.OptType != "" {
		old.OptType = target.OptType
	}

	if target.Status != "" {
		old.Status = target.Status
	}
	if target.Message != "" {
		old.Message = target.Message
	}
	old.FinalStatus = "complete"
	if target.FinalStatus != "" {
		old.FinalStatus = target.FinalStatus
	}

	old.EndTime = time.Now().String()
	if old.Status == "failure" && old.OptType == "callback" {
		old.DeployVersion = old.OldDeployVersion
	}
}

//EventDaoImpl EventLogMessageDaoImpl
type EventDaoImpl struct {
	DB *gorm.DB
}

//GetEventByEventID get event log message
func (c *EventDaoImpl) GetEventByEventID(eventID string) (*model.ServiceEvent, error) {
	var result model.ServiceEvent
	if err := c.DB.Where("event_id=?", eventID).Find(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

//GetEventByEventIDs get event info
func (c *EventDaoImpl) GetEventByEventIDs(eventIDs []string) ([]*model.ServiceEvent, error) {
	var result []*model.ServiceEvent
	if err := c.DB.Where("event_id in (?)", eventIDs).Find(&result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return result, nil
}

//GetEventByServiceID get event log message
func (c *EventDaoImpl) GetEventByServiceID(serviceID string) ([]*model.ServiceEvent, error) {
	var result []*model.ServiceEvent
	if err := c.DB.Where("service_id=?", serviceID).Find(&result).Order("start_time DESC").Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			//return messageRaw, nil
		}
		return nil, err
	}
	return result, nil
}

//GetEventByServiceID delete event log
func (c *EventDaoImpl) DelEventByServiceID(serviceID string) error {
	var result []*model.ServiceEvent
	isNoteExist := c.DB.Where("service_id=?", serviceID).Find(&result).RecordNotFound()
	if isNoteExist {
		return nil
	}
	if err := c.DB.Where("service_id=?", serviceID).Delete(result).Error; err != nil {
		return err
	}
	return nil
}

//NotificationEventDaoImpl NotificationEventDaoImpl
type NotificationEventDaoImpl struct {
	DB *gorm.DB
}

//AddModel AddModel
func (c *NotificationEventDaoImpl) AddModel(mo model.Interface) error {
	result := mo.(*model.NotificationEvent)
	result.LastTime = time.Now()
	result.FirstTime = time.Now()
	result.CreatedAt = time.Now()
	var oldResult model.NotificationEvent
	if ok := c.DB.Where("hash = ?", result.Hash).Find(&oldResult).RecordNotFound(); ok {
		if err := c.DB.Create(result).Error; err != nil {
			return err
		}
	} else {
		logrus.Infoln("event result is exist")
		return c.UpdateModel(mo)
	}
	return nil
}

//UpdateModel UpdateModel
func (c *NotificationEventDaoImpl) UpdateModel(mo model.Interface) error {
	result := mo.(*model.NotificationEvent)
	var oldResult model.NotificationEvent
	if ok := c.DB.Where("hash = ?", result.Hash).Find(&oldResult).RecordNotFound(); !ok {
		result.FirstTime = oldResult.FirstTime
		result.ID = oldResult.ID
	}
	return c.DB.Save(result).Error
}

//GetNotificationEventByKind GetNotificationEventByKind
func (c *NotificationEventDaoImpl) GetNotificationEventByKind(kind, kindID string) ([]*model.NotificationEvent, error) {
	var result []*model.NotificationEvent
	if err := c.DB.Where("kind=? and kind_id=?", kind, kindID).Find(&result).Order("last_time DESC").Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return result, nil
		}
		return nil, err
	}
	return result, nil
}

//GetNotificationEventByTime GetNotificationEventByTime
func (c *NotificationEventDaoImpl) GetNotificationEventByTime(start, end time.Time) ([]*model.NotificationEvent, error) {
	var result []*model.NotificationEvent
	if !start.IsZero() && !end.IsZero() {
		if err := c.DB.Where("last_time>? and last_time<? and is_handle=?", start, end, 0).Find(&result).Order("last_time DESC").Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return result, nil
			}
			return nil, err
		}
		return result, nil
	}
	if err := c.DB.Where("last_time<? and is_handle=?", time.Now(), 0).Find(&result).Order("last_time DESC").Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return result, nil
		}
		return nil, err
	}
	return result, nil
}

//GetNotificationEventNotHandle GetNotificationEventNotHandle
func (c *NotificationEventDaoImpl) GetNotificationEventNotHandle() ([]*model.NotificationEvent, error) {
	var result []*model.NotificationEvent
	if err := c.DB.Where("is_handle=?", false).Find(&result).Order("last_time DESC").Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return result, nil
		}
		return nil, err
	}
	return result, nil
}

//GetNotificationEventByHash GetNotificationEventByHash
func (c *NotificationEventDaoImpl) GetNotificationEventByHash(hash string) (*model.NotificationEvent, error) {
	var result model.NotificationEvent
	if err := c.DB.Where("hash=?", hash).Find(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}
