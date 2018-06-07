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
	"time"
	"fmt"
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

func (e *EventLogMessageDaoImpl) DeleteServiceEventLog() error {
	now := time.Now()
	var messageRaw []*model.EventLogMessage
	if err := e.DB.Find(&messageRaw).Error; err != nil{
		return err
	}
	fmt.Println(len(messageRaw),"列表长度")
	for _,v := range messageRaw{
		start_time := v.StartTime
		fmt.Println(start_time)
		tm2, _ := time.Parse("2006-02-01T15:04:05+08:00", start_time)
		fmt.Println(tm2.Unix())
		fmt.Println(tm2)
		if now.Unix() - tm2.Unix() > 2592000{
			fmt.Println("大于一个月")
			//e.DB.Delete(v)
			fmt.Println("删除成功")
		}
	}
	return nil
}