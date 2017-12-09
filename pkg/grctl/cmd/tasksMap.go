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
	"fmt"
)

func NewCmdTask() cli.Command {
	c:=cli.Command{
		Name: "tasks",
		Usage: "tasks",
		Action: func(c *cli.Context) error {

			tasks,_:=clients.NodeClient.Tasks().List()
			//var total [][]string
			for _,v:=range tasks {
				fmt.Printf("%s",v.ID)
				path:=v.ID
				getDependTask(v,path)
				fmt.Println()
			}
			return nil
		},
	}
	return c
}
func getDependTask(task *model.Task,path string)  {
	if task==nil||task.Temp==nil {
		fmt.Println("wrong task")
		return
	}
	depends:=task.Temp.Depends

	for k,v:=range depends{

		tid:=v.DependTaskID
		taskD,err:=clients.NodeClient.Tasks().Get(tid)
		if err != nil {
			logrus.Errorf("error get task,details %s",err.Error())
			return
		}
		//fmt.Print("task %s depend %s",task.ID,taskD.Task.ID)
		if k==0 {
			fmt.Print("-->"+taskD.Task.ID)

		}else{
			fmt.Println()

			for i:=0;i<len(path);i++{
				fmt.Print(" ")
			}
			fmt.Print("-->"+taskD.Task.ID)
			//path+="-->"+taskD.Task.ID

		}
		getDependTask(taskD.Task,path+"-->"+taskD.Task.ID)
	}
}



