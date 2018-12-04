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
	"encoding/json"
	"time"

	"github.com/goodrain/rainbond/cmd/api/option"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	"github.com/goodrain/rainbond/mq/client"
	"github.com/goodrain/rainbond/worker/discover/model"

	"github.com/Sirupsen/logrus"
	tsdbClient "github.com/bluebreezecf/opentsdb-goclient/client"
	tsdbConfig "github.com/bluebreezecf/opentsdb-goclient/config"
	dbModel "github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
)

//ConDB struct
type ConDB struct {
	ConnectionInfo string
	DBType         string
}

//CreateDBManager get db manager
//TODO: need to try when happend error, try 4 times
func CreateDBManager(conf option.Config) error {
	var tryTime time.Duration
	tryTime = 0
	var err error
	for tryTime < 4 {
		tryTime++
		if err = db.CreateManager(config.Config{
			MysqlConnectionInfo: conf.DBConnectionInfo,
			DBType:              conf.DBType,
		}); err != nil {
			logrus.Errorf("get db manager failed, try time is %v,%s", tryTime, err.Error())
			time.Sleep((5 + tryTime*10) * time.Second)
		} else {
			break
		}
	}
	if err != nil {
		logrus.Errorf("get db manager failed,%s", err.Error())
		return err
	}
	// api database initialization
	go dataInitialization()

	return nil
}

//CreateEventManager create event manager
func CreateEventManager(conf option.Config) error {
	var tryTime time.Duration
	tryTime = 0
	var err error
	for tryTime < 4 {
		tryTime++
		if err = event.NewManager(event.EventConfig{
			EventLogServers: conf.EventLogServers,
			DiscoverAddress: conf.EtcdEndpoint,
		}); err != nil {
			logrus.Errorf("get event manager failed, try time is %v,%s", tryTime, err.Error())
			time.Sleep((5 + tryTime*10) * time.Second)
		} else {
			break
		}
		//defer event.CloseManager()
	}
	if err != nil {
		logrus.Errorf("get event manager failed. %v", err.Error())
		return err
	}
	logrus.Debugf("init event manager success")
	return nil
}

//MQManager mq manager
type MQManager struct {
	EtcdEndpoint  []string
	DefaultServer string
}

//NewMQManager new mq manager
func (m *MQManager) NewMQManager() (client.MQClient, error) {
	client, err := client.NewMqClient(m.EtcdEndpoint, m.DefaultServer)
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

//OpentsdbManager OpentsdbManager
type OpentsdbManager struct {
	Endpoint string
}

//NewOpentsdbManager NewOpentsdbManager
func (o *OpentsdbManager) NewOpentsdbManager() (tsdbClient.Client, error) {
	opentsdbCfg := tsdbConfig.OpenTSDBConfig{
		OpentsdbHost: o.Endpoint,
	}
	tc, err := tsdbClient.NewClient(opentsdbCfg)
	if err != nil {
		return nil, err
	}
	return tc, nil
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

func GetBegin() *gorm.DB {
	return db.GetManager().Begin()
}

func dbInit() error {
	logrus.Info("api database initialization starting...")
	begin := GetBegin()
	// Permissions set
	var rac dbModel.RegionAPIClass
	if err := begin.Where("class_level=? and prefix=?", "server_source", "/v2/show").Find(&rac).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			data := map[string]string{
				"/v2/show":       "server_source",
				"/v2/opentsdb":   "server_source",
				"/v2/resources":  "server_source",
				"/v2/builder":    "server_source",
				"/v2/tenants":    "server_source",
				"/v2/app":        "server_source",
				"/api/v1":        "server_source",
				"/v2/nodes":      "node_manager",
				"/v2/job":        "node_manager",
				"/v2/tasks":      "node_manager",
				"/v2/taskgroups": "node_manager",
				"/v2/tasktemps":  "node_manager",
				"/v2/configs":    "node_manager",
			}
			tx := begin
			var rollback bool
			for k, v := range data {
				if err := db.GetManager().RegionAPIClassDaoTransactions(tx).AddModel(&dbModel.RegionAPIClass{
					ClassLevel: v,
					Prefix:     k,
				}); err != nil {
					tx.Rollback()
					rollback = true
					break
				}
			}
			if !rollback {
				tx.Commit()
			}
		} else {
			return err
		}
	}

	//Port Protocol support
	var rps dbModel.RegionProcotols
	if err := begin.Where("protocol_group=? and protocol_child=?", "http", "http").Find(&rps).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			data := map[string][]string{
				"http":   []string{"http"},
				"stream": []string{"mysql", "tcp", "udp"},
			}
			tx := begin
			var rollback bool
			for k, v := range data {
				for _, v1 := range v {
					if err := db.GetManager().RegionProcotolsDaoTransactions(tx).AddModel(&dbModel.RegionProcotols{
						ProtocolGroup: k,
						ProtocolChild: v1,
						APIVersion:    "v2",
						IsSupport:     true,
					}); err != nil {
						tx.Rollback()
						rollback = true
						break
					}
				}
			}
			if !rollback {
				tx.Commit()
			}
		} else {
			return err
		}
	}

	return nil
}

func dataInitialization() {
	timer := time.NewTimer(time.Second * 2)
	defer timer.Stop()
	for {
		err := dbInit()
		if err != nil {
			logrus.Error("Initializing database failed, ", err)
		} else {
			logrus.Info("api database initialization success!")
			return
		}
		select {
		case <-timer.C:
			timer.Reset(time.Second * 2)
		}
	}
}
