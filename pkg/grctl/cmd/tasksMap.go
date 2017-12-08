// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

package cmd
import (
	"github.com/urfave/cli"
	"github.com/Sirupsen/logrus"


	"github.com/goodrain/rainbond/pkg/grctl/clients"
	"github.com/goodrain/rainbond/pkg/node/api/model"
)

func NewCmdTask() cli.Command {
	c:=cli.Command{
		Name: "tasks",
		Usage: "tasks",
		Action: func(c *cli.Context) error {
			v:=clients.NodeClient.Tasks().Get("check_compute_services").Task
			getDependTask(v)
			v2:=clients.NodeClient.Tasks().Get("check_manage_base_services").Task
			getDependTask(v2)
			v3:=clients.NodeClient.Tasks().Get("check_manage_services").Task
			getDependTask(v3)
			return nil
		},
	}
	return c
}
func getDependTask(task *model.Task) []*model.Task {
	logrus.Infof(task.ID+"--->")
	depends:=task.Temp.Depends
	result:=[]*model.Task{}
	for _,v:=range depends{
		tid:=v.DependTaskID
		task:=clients.NodeClient.Tasks().Get(tid)
		result=append(result,task.Task)
	}
	for _,Deptask:=range result {
		if len(Deptask.Temp.Depends)==0||Deptask.Temp==nil {
			return nil
		}
		getDependTask(Deptask)
	}
	return result
}



