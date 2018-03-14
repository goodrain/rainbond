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

package db

import (
	"github.com/goodrain/rainbond/pkg/api/grpc/client"
	"github.com/goodrain/rainbond/pkg/api/grpc/pb"
	"github.com/goodrain/rainbond/pkg/db"
	"github.com/goodrain/rainbond/pkg/db/config"
	"github.com/goodrain/rainbond/pkg/worker/discover/model"
	"encoding/json"
	"time"

	"github.com/Sirupsen/logrus"
)

//ConDB struct
type ConDB struct {
	ConnectionInfo string
	DBType         string
}

//GetDBManager get db manager
//TODO: need to try when happend error, try 4 times
func (d *ConDB) GetDBManager() (db.Manager, error) {
	var tryTime time.Duration
	tryTime = 0
	var err error
	for tryTime < 4 {
		tryTime++
		if err = db.CreateManager(config.Config{
			MysqlConnectionInfo: d.ConnectionInfo,
			DBType:              d.DBType,
		}); err != nil {
			logrus.Errorf("get db manager failed, try time is %v,%s", tryTime, err.Error())
			time.Sleep((5 + tryTime*10) * time.Second)
		} else {
			break
		}
	}
	if err != nil {
		logrus.Errorf("get db manager failed,%s", err.Error())
		return db.GetManager(), err
	}
	return db.GetManager(), nil
}

//MQManager mq manager
type MQManager struct {
	Endpoint string
}

//NewMQManager new mq manager
func (m *MQManager) NewMQManager() (pb.TaskQueueClient, error) {
	client, err := client.NewMqClient(m.Endpoint)
	if err != nil {
		logrus.Errorf("new mq manager error, %v", err)
		return client, err
	}
	return client, nil
}

//TaskStruct task struct
type TaskStruct struct {
	TaskType string
	TaskBody model.TaskBody
	User     string
}

//BuildTask build task
func BuildTask(t *TaskStruct) (*pb.EnqueueRequest, error) {
	var er pb.EnqueueRequest
	taskJSON, err := json.Marshal(t.TaskBody)
	if err != nil {
		logrus.Errorf("tran task json error")
		return &er, err
	}
	er.Topic = "worker"
	er.Message = &pb.TaskMessage{
		TaskType:   t.TaskType,
		CreateTime: time.Now().Format(time.RFC3339),
		TaskBody:   taskJSON,
		User:       t.User,
	}
	return &er, nil
}
