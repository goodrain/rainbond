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
	"context"
	"encoding/json"
	"time"

	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/db/config"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"

	tsdbClient "github.com/bluebreezecf/opentsdb-goclient/client"
	tsdbConfig "github.com/bluebreezecf/opentsdb-goclient/config"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/worker/discover/model"
	"github.com/sirupsen/logrus"
)

// ConDB struct
type ConDB struct {
	ConnectionInfo string
	DBType         string
}

// New -
func New() *ConDB {
	return &ConDB{}
}

// Start -
func (d *ConDB) Start(ctx context.Context) error {
	dbConfig := configs.Default().DBConfig
	logrus.Info("start db client...")
	dbCfg := config.Config{
		MysqlConnectionInfo: dbConfig.DBConnectionInfo,
		DBType:              dbConfig.DBType,
		ShowSQL:             dbConfig.ShowSQL,
	}
	if err := db.CreateManager(dbCfg); err != nil {
		logrus.Errorf("get db manager failed,%s", err.Error())
		return err
	}
	return nil
}

// CloseHandle -
func (d *ConDB) CloseHandle() {
	err := db.CloseManager()
	if err != nil {
		logrus.Errorf("close db manager failed,%s", err.Error())
	}
}

// TaskStruct task struct
type TaskStruct struct {
	TaskType string
	TaskBody model.TaskBody
	User     string
}

// OpentsdbManager OpentsdbManager
type OpentsdbManager struct {
	Endpoint string
}

// NewOpentsdbManager NewOpentsdbManager
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

// BuildTask build task
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
