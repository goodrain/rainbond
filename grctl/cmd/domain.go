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

package cmd

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/goodrain/rainbond/node/api/model"
	"github.com/urfave/cli"
)

func NewCmdDomain() cli.Command {
	c := cli.Command{
		Name: "domain",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "ip",
				Usage: "ip address",
			},
			cli.StringFlag{
				Name:  "domain",
				Usage: "domain",
			},
		},
		Usage: "",
		Action: func(c *cli.Context) error {
			ip := c.String("ip")
			if len(ip) == 0 {
				fmt.Println("ip must not null")
				return nil
			}
			domain := c.String("domain")
			cmd := exec.Command("bash", "/opt/rainbond/bin/.domain.sh", ip, domain)
			outbuf := bytes.NewBuffer(nil)
			cmd.Stdout = outbuf
			cmd.Run()
			out := outbuf.String()
			fmt.Println(out)
			return nil
		},
	}
	return c
}
func NewCmdCheckTask() cli.Command {
	c := cli.Command{
		Name: "checkTask",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "uuid",
				Usage: "uuid",
			},
		},
		Usage: "",
		Action: func(c *cli.Context) error {
			uuid := c.String("uuid")
			if len(uuid) == 0 {
				fmt.Println("uuid must not null")
				return nil
			}
			tasks, err := clients.RegionClient.Tasks().List()
			if err != nil {
				logrus.Errorf("error get task list,details %s", err.Error())
				return err
			}
			var result []*ExecedTask
			for _, v := range tasks {
				taskStatus, ok := v.Status[uuid]
				if ok {
					status := strings.ToLower(taskStatus.Status)
					if status == "complete" || status == "start" {
						var taskentity = &ExecedTask{}
						taskentity.ID = v.ID
						taskentity.Status = taskStatus.Status
						taskentity.Depends = []string{}
						dealDepend(taskentity, v)
						dealNext(taskentity, tasks)
						result = append(result, taskentity)
						continue
					}

				} else {
					_, scheduled := v.Scheduler.Status[uuid]
					if scheduled {
						var taskentity = &ExecedTask{}
						taskentity.ID = v.ID
						taskentity.Depends = []string{}
						dealDepend(taskentity, v)
						dealNext(taskentity, tasks)
						allDepDone := true

						for _, dep := range taskentity.Depends {
							task, _ := clients.RegionClient.Tasks().Get(dep)

							_, depOK := task.Status[uuid]
							if !depOK {
								allDepDone = false
								break
							}
						}
						if allDepDone {
							taskentity.Status = "start"
							result = append(result, taskentity)
						}

					}
				}
			}
			for _, v := range result {
				fmt.Printf("task %s is %s,depends is %v\n", v.ID, v.Status, v.Depends)
			}
			return nil
		},
	}
	return c
}
func dealDepend(result *ExecedTask, task *model.Task) {
	if task.Temp.Depends != nil {
		for _, v := range task.Temp.Depends {
			result.Depends = append(result.Depends, v.DependTaskID)
		}
	}
}
func dealNext(task *ExecedTask, tasks []*model.Task) {
	for _, v := range tasks {
		if v.Temp.Depends != nil {
			for _, dep := range v.Temp.Depends {
				if dep.DependTaskID == task.ID {
					task.Next = append(task.Next, v.ID)
				}
			}
		}
	}
}

type ExecedTask struct {
	ID      string
	Status  string
	Depends []string
	Next    []string
}
