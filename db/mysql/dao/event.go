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
	"context"
	"strings"
	"time"

	gormbulkups "github.com/atcdot/gorm-bulk-upsert"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
	update := mo.(*model.ServiceEvent)
	var oldResult model.ServiceEvent
	if ok := c.DB.Where("event_id=?", update.EventID).Find(&oldResult).RecordNotFound(); !ok {
		update.ID = oldResult.ID
		if err := c.DB.Save(&update).Error; err != nil {
			return err
		}
	}
	return nil
}

//EventDaoImpl EventLogMessageDaoImpl
type EventDaoImpl struct {
	DB *gorm.DB
}

// CreateEventsInBatch creates events in batch.
func (c *EventDaoImpl) CreateEventsInBatch(events []*model.ServiceEvent) error {
	var objects []interface{}
	for _, event := range events {
		event := event
		objects = append(objects, *event)
	}
	if err := gormbulkups.BulkUpsert(c.DB, objects, 200); err != nil {
		return errors.Wrap(err, "create events in batch")
	}
	return nil
}

// UpdateReason update reasion.
func (c *EventDaoImpl) UpdateReason(eventID string, reason string) error {
	return c.DB.Model(&model.ServiceEvent{}).Where("event_id=?", eventID).UpdateColumn("reason", reason).Error
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
			return result, nil
		}
		return nil, err
	}
	return result, nil
}

//DelEventByServiceID delete event log
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

// ListByTargetID -
func (c *EventDaoImpl) ListByTargetID(targetID string) ([]*model.ServiceEvent, error) {
	var events []*model.ServiceEvent
	if err := c.DB.Where("target_id=?", targetID).Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

// GetEventsByTarget get event by target with page
func (c *EventDaoImpl) GetEventsByTarget(target, targetID string, offset, limit int) ([]*model.ServiceEvent, int, error) {
	var result []*model.ServiceEvent
	var total int
	db := c.DB
	if target != "" && targetID != "" {
		// Compatible with previous 5.1.7 data, with null target and targetid
		if strings.TrimSpace(target) == "service" {
			db = db.Where("service_id=? or (target=? and target_id=?) ", strings.TrimSpace(targetID), strings.TrimSpace(target), strings.TrimSpace(targetID))
		} else {
			db = db.Where("target=? and target_id=?", strings.TrimSpace(target), strings.TrimSpace(targetID))
		}
	}
	if err := db.Model(&model.ServiceEvent{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Offset(offset).Limit(limit).Order("create_time DESC, ID DESC").Find(&result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return result, 0, nil
		}
		return nil, 0, err
	}

	return result, total, nil
}

// GetEventsByTenantID get event by tenantID
func (c *EventDaoImpl) GetEventsByTenantID(tenantID string, offset, limit int) ([]*model.ServiceEvent, int, error) {
	var total int
	if err := c.DB.Model(&model.ServiceEvent{}).Where("tenant_id=?", tenantID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var result []*model.ServiceEvent
	if err := c.DB.Where("tenant_id=?", tenantID).Offset(offset).Limit(limit).Order("start_time DESC, ID DESC").Find(&result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return result, 0, nil
		}
		return nil, 0, err
	}
	return result, total, nil
}

//GetLastASyncEvent get last sync event
func (c *EventDaoImpl) GetLastASyncEvent(target, targetID string) (*model.ServiceEvent, error) {
	var result model.ServiceEvent
	if err := c.DB.Where("target=? and target_id=? and syn_type=0", target, targetID).Last(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

// UnfinishedEvents returns unfinished events.
func (c *EventDaoImpl) UnfinishedEvents(target, targetID string, optTypes ...string) ([]*model.ServiceEvent, error) {
	var result []*model.ServiceEvent
	if err := c.DB.Where("target=? and target_id=? and status=? and opt_type in (?)", target, targetID, model.EventStatusFailure.String(), optTypes).
		Find(&result).Error; err != nil {
		return nil, err
	}
	return result, nil
}

// LatestFailurePodEvent returns the latest failure pod event.
func (c *EventDaoImpl) LatestFailurePodEvent(podName string) (*model.ServiceEvent, error) {
	var event model.ServiceEvent
	if err := c.DB.Where("target=? and target_id=? and status=? and final_status<>?", model.TargetTypePod, podName, model.EventStatusFailure.String(), model.EventFinalStatusEmptyComplete.String()).
		Last(&event).Error; err != nil {
		return nil, err
	}
	return &event, nil
}

func (c *EventDaoImpl) SetEventStatus(ctx context.Context, status model.EventStatus) error {
	event, _ := ctx.Value(ctxutil.ContextKey("event")).(*model.ServiceEvent)
	if event != nil {
		event.FinalStatus = "complete"
		event.Status = string(status)
		return c.UpdateModel(event)
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
		return c.DB.Save(result).Error
	}
	return gorm.ErrRecordNotFound
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
