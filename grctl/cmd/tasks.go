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
	"encoding/json"
	"fmt"
	"time"

	"github.com/apcera/termtables"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/goodrain/rainbond/node/api/model"
	"github.com/gosuri/uitable"
	"github.com/urfave/cli"
)

//NewCmdTasks 任务相关命令
func NewCmdTasks() cli.Command {
	c := cli.Command{
		Name:  "tasks",
		Usage: "系统任务相关命令，grctl tasks -h",
		Subcommands: []cli.Command{
			cli.Command{
				Name:  "list",
				Usage: "List all task",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "g",
						Usage: "show all task longitudinal",
					},
				},
				Action: func(c *cli.Context) error {
					tasks, err := clients.RegionClient.Tasks().List()
					if err != nil {
						return err
					}
					if len(tasks) > 0 {
						if c.Bool("g") {
							for _, v := range tasks {
								table := uitable.New()
								table.Wrap = true // wrap columns
								table.AddRow("ID", v.ID)
								table.AddRow("GroupID", v.GroupID)
								var depstr string
								for _, dep := range v.Temp.Depends {
									depstr += fmt.Sprintf("%s(%s)\n", dep.DependTaskID, dep.DetermineStrategy)
								}
								var status string
								for k, v := range v.Status {
									status += fmt.Sprintf("%s:%s(%s)\n", k, v.Status, v.CompleStatus)
								}
								var scheduler = v.Scheduler.Mode + "\n"
								if len(v.Scheduler.Status) == 0 {
									scheduler += "暂未调度"
								} else {
									for k, v := range v.Scheduler.Status {
										scheduler += fmt.Sprintf("%s:%s(%s)\n", k, v.Status, v.SchedulerTime.Format(time.RFC3339))
									}
								}
								table.AddRow("DepTask", depstr)
								table.AddRow("Status", status)
								table.AddRow("Scheduler", scheduler)
								fmt.Println(table.String())
								fmt.Println("--------------------------------------------")
							}
						} else {
							taskTable := termtables.CreateTable()
							taskTable.AddHeaders("ID", "GroupID", "DepTask", "Status", "Scheduler")
							for _, v := range tasks {
								var depstr string
								for _, dep := range v.Temp.Depends {
									depstr += fmt.Sprintf("%s(%s);", dep.DependTaskID, dep.DetermineStrategy)
								}
								var status string
								for k, v := range v.Status {
									status += fmt.Sprintf("%s:%s(%s);", k, v.Status, v.CompleStatus)
								}
								var scheduler = v.Scheduler.Mode + ";"
								if len(v.Scheduler.Status) == 0 {
									scheduler += "暂未调度"
								} else {
									for k, v := range v.Scheduler.Status {
										scheduler += fmt.Sprintf("%s:%s(%s);", k, v.Status, v.SchedulerTime.Format(time.RFC3339))
									}
								}
								taskTable.AddRow(v.ID, v.GroupID, depstr, status, scheduler)
							}
							fmt.Println(taskTable.Render())
						}
						return nil
					}
					fmt.Println("not found tasks")
					return nil
				},
			},
			cli.Command{
				Name:  "get",
				Usage: "Displays the specified task details",
				Action: func(c *cli.Context) error {
					taskID := c.Args().First()
					if taskID == "" {
						fmt.Println("Please specified task id")
					}
					task, err := clients.RegionClient.Tasks().Get(taskID)
					if err != nil {
						return fmt.Errorf("get task error,%s", err.Error())
					}
					taskStr, _ := json.MarshalIndent(&task, "", "\t")
					fmt.Println(string(taskStr))
					return nil
				},
			},
			cli.Command{
				Name:  "exec",
				Usage: "Exec the specified task",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "n",
						Usage: "exec task nodeid",
					},
					cli.StringFlag{
						Name:  "f",
						Usage: "exec task and return status",
					},
				},
				Action: func(c *cli.Context) error {
					taskID := c.Args().First()
					if taskID == "" {
						fmt.Println("Please specified task id")
					}
					nodeID := c.String("n")
					if nodeID == "" {
						fmt.Println("Please specified nodeid use `-n`")
					}
					//fmt.Printf("node is %s task id is %s",nodeID,taskID)
					err := clients.RegionClient.Tasks().Exec(taskID, []string{nodeID})
					handleErr(err)
					return nil
				},
			},
		},
	}
	return c
}

func getDependTask(task *model.Task, path string) {
	if task == nil || task.Temp == nil {
		fmt.Println("wrong task")
		return
	}
	depends := task.Temp.Depends

	for k, v := range depends {

		tid := v.DependTaskID
		taskD, err := clients.RegionClient.Tasks().Get(tid)
		handleErr(err)
		//fmt.Print("task %s depend %s",task.ID,taskD.Task.ID)
		if k == 0 {
			fmt.Print("-->" + taskD.ID)

		} else {
			fmt.Println()

			for i := 0; i < len(path); i++ {
				fmt.Print(" ")
			}
			fmt.Print("-->" + taskD.ID)
			//path+="-->"+taskD.Task.ID

		}
		getDependTask(taskD, path+"-->"+taskD.ID)
	}
}

//ExecTaskAndCheckStatus 执行一个任务并检测状态
func ExecTaskAndCheckStatus(taskID string, nodes []string) {

}
