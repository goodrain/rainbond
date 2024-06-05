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
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"
	"github.com/goodrain/rainbond/pkg/gogo"
	"time"

	tsdbClient "github.com/bluebreezecf/opentsdb-goclient/client"
	tsdbConfig "github.com/bluebreezecf/opentsdb-goclient/config"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/config"
	dbModel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/worker/discover/model"
	"github.com/jinzhu/gorm"
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
func (d *ConDB) Start(ctx context.Context, cfg *configs.Config) error {
	logrus.Info("start db client...")
	dbCfg := config.Config{
		MysqlConnectionInfo: cfg.APIConfig.DBConnectionInfo,
		DBType:              cfg.APIConfig.DBType,
		ShowSQL:             cfg.APIConfig.ShowSQL,
	}
	if err := db.CreateManager(dbCfg); err != nil {
		logrus.Errorf("get db manager failed,%s", err.Error())
		return err
	}

	// api database initialization
	_ = gogo.Go(func(ctx context.Context) error {
		timer := time.NewTimer(time.Second * 2)
		defer timer.Stop()
		for {
			err := dbInit()
			if err != nil {
				logrus.Error("Initializing database failed, ", err)
			} else {
				logrus.Info("api database initialization success!")
				return nil
			}
			select {
			case <-timer.C:
				timer.Reset(time.Second * 2)
			}
		}
	})
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

// GetBegin get db transaction
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
				"/v2/show":           "server_source",
				"/v2/cluster":        "server_source",
				"/v2/resources":      "server_source",
				"/v2/builder":        "server_source",
				"/v2/tenants":        "server_source",
				"/v2/app":            "server_source",
				"/v2/port":           "server_source",
				"/v2/volume-options": "server_source",
				"/api/v1":            "server_source",
				"/v2/events":         "server_source",
				"/v2/gateway/ips":    "server_source",
				"/v2/gateway/ports":  "server_source",
				"/v2/nodes":          "node_manager",
				"/v2/job":            "node_manager",
				"/v2/configs":        "node_manager",
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

	return nil
}
