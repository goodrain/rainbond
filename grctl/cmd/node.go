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
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/apcera/termtables"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/gosuri/uitable"
	"github.com/urfave/cli"
)

func handleErr(err *util.APIHandleError) {
	if err != nil && err.Err != nil {
		fmt.Printf("%v\n", err.String())
		os.Exit(1)
	}
}
func NewCmdShow() cli.Command {
	c := cli.Command{
		Name:  "show",
		Usage: "显示region安装完成后访问地址",
		Action: func(c *cli.Context) error {
			Common(c)
			manageHosts, err := clients.RegionClient.Nodes().GetNodeByRule("manage")
			handleErr(err)
			ips := getExternalIP("/opt/rainbond/envs/.exip", manageHosts)
			fmt.Println("Manage your apps with webui：")
			for _, v := range ips {
				url := v + ":7070"
				fmt.Print(url + "  ")
			}
			fmt.Println()
			fmt.Println("The webui use websocket to provide more feture：")
			for _, v := range ips {
				url := v + ":6060"
				fmt.Print(url + "  ")
			}
			fmt.Println()
			fmt.Println("Your web apps use nginx for reverse proxy:")
			for _, v := range ips {
				url := v + ":80"
				fmt.Print(url + "  ")
			}
			fmt.Println()
			return nil
		},
	}
	return c
}

func getExternalIP(path string, node []*client.HostNode) []string {
	var result []string
	if fileExist(path) {
		externalIP, err := ioutil.ReadFile(path)
		if err != nil {
			return nil
		}
		strings.TrimSpace(string(externalIP))
		result = append(result, strings.TrimSpace(string(externalIP)))
	} else {
		for _, v := range node {
			result = append(result, v.InternalIP)
		}
	}
	return result
}
func fileExist(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func handleStatus(serviceTable *termtables.Table, ready bool, v *client.HostNode) {
	var status string
	if ready == true {
		status = "\033[0;32;32m running(healthy) \033[0m"
	}
	if v.Unschedulable == true {
		status = "\033[0;33;33m running(unschedulable) \033[0m"
	}
	if ready == false {
		status = "\033[0;33;33m running(unhealthy) \033[0m"
	}
	if ready == false && v.Unschedulable == true {
		status = "\033[0;33;33m running(unhealthy,unschedulable) \033[0m"
	}
	if v.Status == "unknown" {
		status = "\033[0;31;31m unknown \033[0m"
	}
	if v.Status == "offline" {
		status = "\033[0;31;31m offline \033[0m"
	}
	if v.Role.HasRule("compute") && !v.Role.HasRule("manage") {
		serviceTable.AddRow(v.ID, v.InternalIP, v.HostName, v.Role.String(), v.Mode, status)
	} else if v.Role.HasRule("manage") && !v.Role.HasRule("compute") {
		//scheduable="n/a"
		serviceTable.AddRow(v.ID, v.InternalIP, v.HostName, v.Role.String(), v.Mode, status)
	} else if v.Role.HasRule("compute") && v.Role.HasRule("manage") {
		serviceTable.AddRow(v.ID, v.InternalIP, v.HostName, v.Role.String(), v.Mode, status)
	}
}

func handleResult(serviceTable *termtables.Table, v *client.HostNode) {

	for _, v := range v.NodeStatus.Conditions {
		if v.Type == client.NodeReady {
			continue
		}
		var formatReady string
		if v.Status == client.ConditionFalse {
			if v.Type == client.OutOfDisk || v.Type == client.MemoryPressure || v.Type == client.DiskPressure || v.Type == client.InstallNotReady {
				formatReady = "\033[0;32;32m false \033[0m"
			} else {
				formatReady = "\033[0;31;31m true \033[0m"
			}
		} else {
			formatReady = "\033[0;32;32m true \033[0m"
		}
		serviceTable.AddRow(string(v.Type), formatReady, handleMessage(string(v.Status), v.Message))
	}
}

func extractReady(serviceTable *termtables.Table, v *client.HostNode, name string) {
	for _, v := range v.NodeStatus.Conditions {
		if string(v.Type) == name {
			var formatReady string
			if v.Status == client.ConditionFalse {
				formatReady = "\033[0;31;31m false \033[0m"
			} else {
				formatReady = "\033[0;32;32m true \033[0m"
			}
			serviceTable.AddRow("\033[0;33;33m "+string(v.Type)+" \033[0m", formatReady, handleMessage(string(v.Status), v.Message))
		}
	}
}

func handleMessage(status string, message string) string {
	if status == "True" {
		return ""
	}
	return message
}

//NewCmdNode NewCmdNode
func NewCmdNode() cli.Command {
	c := cli.Command{
		Name:  "node",
		Usage: "节点管理相关操作",
		Subcommands: []cli.Command{
			{
				Name:  "get",
				Usage: "get hostID/internal ip",
				Action: func(c *cli.Context) error {
					Common(c)
					id := c.Args().First()
					if id == "" {
						logrus.Errorf("need args")
						return nil
					}
					nodes, err := clients.RegionClient.Nodes().List()
					handleErr(err)
					for _, v := range nodes {
						if v.InternalIP == id {
							id = v.ID
							break
						}
					}

					v, err := clients.RegionClient.Nodes().Get(id)
					handleErr(err)
					table := uitable.New()
					fmt.Printf("-------------------Node information-----------------------\n")
					table.AddRow("status", v.NodeStatus.Status)
					table.AddRow("unschedulable", v.Unschedulable)
					table.AddRow("alived", v.Alived)
					table.AddRow("uuid", v.ID)
					table.AddRow("host_name", v.HostName)
					table.AddRow("create_time", v.CreateTime)
					table.AddRow("internal_ip", v.InternalIP)
					table.AddRow("external_ip", v.ExternalIP)
					table.AddRow("role", v.Role)
					table.AddRow("mode", v.Mode)
					table.AddRow("available_memory", v.AvailableMemory)
					table.AddRow("available_cpu", v.AvailableCPU)
					table.AddRow("pid", v.PID)
					table.AddRow("version", v.Version)
					table.AddRow("up", v.UpTime)
					table.AddRow("down", v.DownTime)
					table.AddRow("connected", v.Connected)
					fmt.Println(table)
					fmt.Printf("-------------------Service health-----------------------\n")
					serviceTable := termtables.CreateTable()
					serviceTable.AddHeaders("Title", "Result", "Message")
					extractReady(serviceTable, v, "Ready")
					handleResult(serviceTable, v)

					fmt.Println(serviceTable.Render())
					return nil
				},
			},
			{
				Name:  "list",
				Usage: "list",
				Action: func(c *cli.Context) error {
					Common(c)
					list, err := clients.RegionClient.Nodes().List()
					handleErr(err)
					serviceTable := termtables.CreateTable()
					serviceTable.AddHeaders("Uid", "IP", "HostName", "NodeRole", "NodeMode", "Status")
					var rest []*client.HostNode
					for _, v := range list {
						if v.Role.HasRule("manage") || !v.Role.HasRule("compute") {
							handleStatus(serviceTable, isNodeReady(v), v)
						} else {
							rest = append(rest, v)
						}
					}
					if len(rest) > 0 {
						serviceTable.AddSeparator()
					}
					for _, v := range rest {
						handleStatus(serviceTable, isNodeReady(v), v)
					}
					fmt.Println(serviceTable.Render())
					return nil
				},
			},
			{
				Name:  "resource",
				Usage: "resource",
				Action: func(c *cli.Context) error {
					Common(c)
					list, err := clients.RegionClient.Nodes().List()
					handleErr(err)
					serviceTable := termtables.CreateTable()
					serviceTable.AddHeaders("Uid", "HostName", "CapCpu(核)", "CapMemory(M)", "UsedCpu(核)", "UsedMemory(M)", "CpuLimits(核)", "MemoryLimits(M)", "CpuUsageRate(%)", "MemoryUsedRate(%)")
					for _, v := range list {
						if v.Role.HasRule("compute") && v.NodeStatus.Status != "offline" {
							nodeResource, err := clients.RegionClient.Nodes().GetNodeResource(v.ID)
							handleErr(err)
							CPURequests := strconv.FormatFloat(float64(nodeResource.CPURequests)/float64(1000), 'f', 2, 64)
							CPULimits := strconv.FormatFloat(float64(nodeResource.CPULimits)/float64(1000), 'f', 2, 64)
							serviceTable.AddRow(v.ID, v.HostName, nodeResource.CpuR, nodeResource.MemR, CPURequests, nodeResource.MemoryRequests, CPULimits, nodeResource.MemoryLimits, nodeResource.CPURequestsR, nodeResource.MemoryRequestsR)
						}
					}
					fmt.Println(serviceTable.Render())
					return nil
				},
			},
			{
				Name:  "up",
				Usage: "up hostID",
				Action: func(c *cli.Context) error {
					Common(c)
					id := c.Args().First()
					if id == "" {
						logrus.Errorf("need hostID")
						return nil
					}
					err := clients.RegionClient.Nodes().Up(id)
					handleErr(err)
					return nil
				},
			},
			{
				Name:  "down",
				Usage: "down hostID",
				Action: func(c *cli.Context) error {
					Common(c)
					id := c.Args().First()
					if id == "" {
						logrus.Errorf("need hostID")
						return nil
					}
					err := clients.RegionClient.Nodes().Down(id)
					handleErr(err)
					return nil
				},
			},
			{
				Name:  "cordon",
				Usage: "Mark node as unschedulable",
				Action: func(c *cli.Context) error {
					Common(c)
					id := c.Args().First()
					if id == "" {
						logrus.Errorf("need hostID")
						return nil
					}
					node, err := clients.RegionClient.Nodes().Get(id)
					handleErr(err)
					if !node.Role.HasRule("compute") {
						logrus.Errorf("管理节点不支持此功能")
						return nil
					}
					err = clients.RegionClient.Nodes().UnSchedulable(id)
					handleErr(err)
					return nil
				},
			},
			{
				Name:  "uncordon",
				Usage: "Mark node as schedulable",
				Action: func(c *cli.Context) error {
					Common(c)
					id := c.Args().First()
					if id == "" {
						logrus.Errorf("need hostID")
						return nil
					}
					node, err := clients.RegionClient.Nodes().Get(id)
					handleErr(err)
					if !node.Role.HasRule("compute") {
						logrus.Errorf("管理节点不支持此功能")
						return nil
					}
					err = clients.RegionClient.Nodes().ReSchedulable(id)
					handleErr(err)
					return nil
				},
			},
			{
				Name:  "delete",
				Usage: "delete hostID",
				Action: func(c *cli.Context) error {
					Common(c)
					id := c.Args().First()
					if id == "" {
						logrus.Errorf("need hostID")
						return nil
					}
					err := clients.RegionClient.Nodes().Delete(id)
					handleErr(err)
					return nil
				},
			},
			{
				Name:  "rule",
				Usage: "rule ruleName",
				Action: func(c *cli.Context) error {
					Common(c)
					rule := c.Args().First()
					if rule == "" {
						logrus.Errorf("need rule name")
						return nil
					}
					hostnodes, err := clients.RegionClient.Nodes().GetNodeByRule(rule)
					handleErr(err)
					serviceTable := termtables.CreateTable()
					serviceTable.AddHeaders("Uid", "IP", "HostName", "NodeRole", "NodeMode", "Status")
					for _, v := range hostnodes {
						handleStatus(serviceTable, isNodeReady(v), v)
					}
					return nil
				},
			},
			{
				Name:  "label",
				Usage: "label hostID",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "key",
						Value: "",
						Usage: "key",
					},
					cli.StringFlag{
						Name:  "val",
						Value: "",
						Usage: "val",
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)
					hostID := c.Args().First()
					if hostID == "" {
						logrus.Errorf("need hostID")
						return nil
					}
					k := c.String("key")
					v := c.String("val")
					label := make(map[string]string)
					label[k] = v
					err := clients.RegionClient.Nodes().Label(hostID, label)
					handleErr(err)
					return nil
				},
			},
			{
				Name:  "add",
				Usage: "Add a node into the cluster",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "hostname,host",
						Usage: "The option is required",
					},
					cli.StringFlag{
						Name:  "internal-ip,iip",
						Usage: "The option is required",
					},
					cli.StringFlag{
						Name:  "external-ip,eip",
						Usage: "Publish the ip address for external connection",
					},
					cli.StringFlag{
						Name:  "root-pass,p",
						Usage: "Specify the root password of the target host for login, this option conflicts with private-key",
					},
					cli.StringFlag{
						Name:  "private-key,key",
						Usage: "Specify the private key file for login, this option conflicts with root-pass",
					},
					cli.StringFlag{
						Name:  "role,r",
						Usage: "The option is required, the allowed values are: [manage|compute|storage]",
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)
					if !c.IsSet("role") {
						println("role must not null")
						return nil
					}

					if c.String("root-pass") != "" && c.String("private-key") != "" {
						println("Options private-key and root-pass are conflicting")
						return nil
					}

					model := "pass"
					if c.String("private-key") != "" {
						model = "key"
					}

					// start add node script
					fmt.Println("Begin add node, please don't exit")
					line := fmt.Sprintf("cd /opt/rainbond/install/scripts; ./%s.sh %s %s %s %s %s", c.String("role"), c.String("hostname"),
						c.String("internal-ip"), model, c.String("root-pass"), c.String("private-key"))
					cmd := exec.Command("bash", "-c", line)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr

					err := cmd.Run()
					if err != nil {
						println(err.Error())
						return nil
					}

					fmt.Println("Add node successful, next you can:")
					fmt.Println("	check cluster status: grctl node list")
					//fmt.Println("	online node: grctl node up --help")

					//var node client.APIHostNode
					//node.Role = append(node.Role, c.String("role"))
					//node.HostName = c.String("hostname")
					//node.RootPass = c.String("root-pass")
					//node.InternalIP = c.String("internal-ip")
					//node.ExternalIP = c.String("external-ip")

					//err := clients.RegionClient.Nodes().Add(&node)
					//handleErr(err)
					//fmt.Println("success add node")

					return nil
				},
			},
		},
	}
	return c
}

func isNodeReady(node *client.HostNode) bool {
	if node.NodeStatus == nil {
		return false
	}
	for _, v := range node.NodeStatus.Conditions {
		if strings.ToLower(string(v.Type)) == "ready" {
			if strings.ToLower(string(v.Status)) == "true" {
				return true
			}
		}
	}

	return false
}
