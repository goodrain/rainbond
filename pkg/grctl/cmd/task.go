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
	"strings"
	"github.com/pquerna/ffjson/ffjson"
	"io/ioutil"
	"github.com/goodrain/rainbond/pkg/node/api/model"

	"time"
	"github.com/goodrain/rainbond/pkg/grctl/clients"
)


func NewCmdAddTask() cli.Command {
	c:=cli.Command{
		Name:  "add_task",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "taskfile",
				Usage: "file path",
			},
		},
		Usage: "添加task。grctl add_task",
		Action: func(c *cli.Context) error {
			file:=c.String("filepath")
			if file!="" {
				err:=loadFile(file)
				if err != nil {
					logrus.Errorf("error add task from file,details %s",err.Error())
					return err
				}
			}else {
				logrus.Errorf("error get task from path")
			}
			return nil
		},
	}
	return c
}
func  ScheduleGroup(nodes []string, nextGroups ...*model.TaskGroup) error{
	for _, group := range nextGroups {
		if group.Tasks == nil || len(group.Tasks) < 1 {
			group.Status = &model.TaskGroupStatus{
				StartTime: time.Now(),
				EndTime:   time.Now(),
				Status:    "NotDefineTask",
			}
			//create group
			err:=clients.NodeClient.Tasks().AddGroup(group)
			if err!=nil{
				logrus.Errorf("error add group,details %s",err.Error())
				return  err
			}
		}
		for _, task := range group.Tasks {
			task.GroupID = group.ID
			err:=clients.NodeClient.Tasks().Add(task)
			if err!=nil{
				logrus.Errorf("error add group,details %s",err.Error())
				return err
			}
		}
		group.Status = &model.TaskGroupStatus{
			StartTime: time.Now(),
			Status:    "Start",
		}
		//create group
		err:=clients.NodeClient.Tasks().AddGroup(group)
		if err!=nil{
			logrus.Errorf("error add group,details %s",err.Error())
			return err
		}
	}
	return nil
}
func loadFile(path string) error{
	taskBody, err := ioutil.ReadFile(path)
	if err != nil {
		logrus.Errorf("read static task file %s error.%s", path, err.Error())
		return nil
	}
	var filename string
	index := strings.LastIndex(path, "/")
	if index < 0 {
		filename = path
	}
	filename = path[index+1:]

	if strings.Contains(filename, "group") {
		var group model.TaskGroup
		if err := ffjson.Unmarshal(taskBody, &group); err != nil {
			logrus.Errorf("unmarshal static task file %s error.%s", path, err.Error())
			return nil
		}
		if group.ID == "" {
			group.ID = group.Name
		}
		if group.Name == "" {
			logrus.Errorf("task group name can not be empty. file %s", path)
			return nil
		}
		if group.Tasks == nil {
			logrus.Errorf("task group tasks can not be empty. file %s", path)
			return nil
		}
		ScheduleGroup(nil, &group)
		logrus.Infof("Load a static group %s.", group.Name)
	}
	if strings.Contains(filename, "task") {
		var task model.Task
		if err := ffjson.Unmarshal(taskBody, &task); err != nil {
			logrus.Errorf("unmarshal static task file %s error.%s", path, err.Error())
			return err
		}
		if task.ID == "" {
			task.ID = task.Name
		}
		if task.Name == "" {
			logrus.Errorf("task name can not be empty. file %s", path)
			return err
		}
		if task.Temp == nil {
			logrus.Errorf("task [%s] temp can not be empty.", task.Name)
			return err
		}
		if task.Temp.ID == "" {
			task.Temp.ID = task.Temp.Name
		}
		err:=clients.NodeClient.Tasks().Add(&task)
		if err!=nil{
			logrus.Errorf("error add task,details %s",err.Error())
			return err
		}
		logrus.Infof("Load a static group %s.", task.Name)
		return nil
	}
	return nil
}