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

package executor

import (
	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/worker/discover/model"
	"os"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
)

func init() {
	if err := db.CreateManager(option.Config{
		MysqlConnectionInfo: "root:admin@tcp(127.0.0.1:3306)/region",
		DBType:              "mysql",
	}); err != nil {
		logrus.Error(err)
	}
	event.NewManager(option.Config{
		EventLogServers: []string{"tcp://127.0.0.1:6366"},
	})
	os.Setenv("CUR_NET", "midonet")
	os.Setenv("EX_DOMAIN", "test-ali.goodrain.net:10080")
}
func TestAddStartTask(t *testing.T) {
	manager, err := NewManager(option.Config{
		KubeConfig: "../../admin.kubeconfig",
	})
	if err != nil {
		t.Fatal(err)
	}
	task := manager.TaskManager().NewStartTask(&model.Task{
		Body: model.StartTaskBody{
			TenantID:      "",
			ServiceID:     "889bb1f028f655bebd545f24aa184a0b",
			DeployVersion: "",
			EventID:       "system",
		},
	}, event.GetManager().GetLogger("system"))
	manager.AddTask(task)
}
func TestAddStopTask(t *testing.T) {
	manager, err := NewManager(option.Config{
		KubeConfig: "../../admin.kubeconfig",
	})
	if err != nil {
		t.Fatal(err)
	}

	task := manager.TaskManager().NewStopTask(&model.Task{
		Body: model.StopTaskBody{
			TenantID:      "",
			ServiceID:     "889bb1f028f655bebd545f24aa184a0b",
			DeployVersion: "",
			EventID:       "system",
		},
	}, event.GetManager().GetLogger("system"))
	manager.AddTask(task)
	time.Sleep(15 * time.Second)
}
