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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/apcera/termtables"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/goodrain/rainbond/node/nodem/client"
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
			manageHosts, err := clients.RegionClient.Nodes().GetNodeByRule("manage")
			handleErr(err)
			ips := getExternalIP("/etc/goodrain/envs/.exip", manageHosts)
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
	if v.Role.HasRule("compute") && !v.Role.HasRule("manage") {
		serviceTable.AddRow(v.ID, v.InternalIP, v.HostName, v.Role.String(), v.Mode, v.Status, v.Alived, !v.Unschedulable, ready)
	} else if v.Role.HasRule("manage") && !v.Role.HasRule("compute") {
		//scheduable="n/a"
		serviceTable.AddRow(v.ID, v.InternalIP, v.HostName, v.Role.String(), v.Mode, v.Status, v.Alived, "N/A", "N/A")
	} else if v.Role.HasRule("compute") && v.Role.HasRule("manage") {
		serviceTable.AddRow(v.ID, v.InternalIP, v.HostName, v.Role.String(), v.Mode, v.Status, v.Alived, !v.Unschedulable, ready)
	}
}

//NewCmdNode NewCmdNode
func NewCmdNode() cli.Command {
	c := cli.Command{
		Name:  "node",
		Usage: "节点。grctl node",
		Subcommands: []cli.Command{
			{
				Name:  "get",
				Usage: "get hostID/internal ip",
				Action: func(c *cli.Context) error {
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
					nodeByte, _ := json.Marshal(v)
					var out bytes.Buffer
					error := json.Indent(&out, nodeByte, "", "\t")
					if error != nil {
						handleErr(util.CreateAPIHandleError(500, err))
					}
					fmt.Println(out.String())
					return nil
				},
			},
			{
				Name:  "list",
				Usage: "list",
				Action: func(c *cli.Context) error {
					list, err := clients.RegionClient.Nodes().List()
					handleErr(err)
					serviceTable := termtables.CreateTable()
					serviceTable.AddHeaders("Uid", "IP", "HostName", "NodeRole", "NodeMode", "Status", "Alived", "Schedulable", "Ready")
					var rest []*client.HostNode
					for _, v := range list {
						if v.Role.HasRule("manage") {
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
				Name:  "up",
				Usage: "up hostID",
				Action: func(c *cli.Context) error {
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
				Name:  "unscheduable",
				Usage: "unscheduable hostID",
				Action: func(c *cli.Context) error {
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
				Name:  "rescheduable",
				Usage: "rescheduable hostID",
				Action: func(c *cli.Context) error {
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
					rule := c.Args().First()
					if rule == "" {
						logrus.Errorf("need rule name")
						return nil
					}
					hostnodes, err := clients.RegionClient.Nodes().GetNodeByRule(rule)
					handleErr(err)
					serviceTable := termtables.CreateTable()
					serviceTable.AddHeaders("Uid", "IP", "HostName", "NodeRole", "NodeMode", "Status", "Alived", "Schedulable", "Ready")
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
				Usage: "add 添加节点",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "Hostname,hn",
						Value: "",
						Usage: "Hostname",
					},
					cli.StringFlag{
						Name:  "InternalIP,i",
						Value: "",
						Usage: "InternalIP|required",
					},
					cli.StringFlag{
						Name:  "ExternalIP,e",
						Value: "",
						Usage: "ExternalIP",
					},
					cli.StringFlag{
						Name:  "RootPass,p",
						Value: "",
						Usage: "RootPass",
					},
					cli.StringFlag{
						Name:  "Role,ro",
						Usage: "Role|required",
					},
				},
				Action: func(c *cli.Context) error {
					var node client.APIHostNode
					if c.IsSet("Role") {
						node.Role = append(node.Role, c.String("Role"))
						node.InternalIP = c.String("InternalIP")
						node.HostName = c.String("HostName")
						node.ExternalIP = c.String("ExternalIP")
						node.RootPass = c.String("RootPass")

						err := clients.RegionClient.Nodes().Add(&node)
						handleErr(err)
						fmt.Println("success add node")

						// var hostNode *client.HostNode
						// timer := time.NewTimer(15 * time.Second)
						// gotNode := false
						// for !gotNode {
						// 	time.Sleep(3 * time.Second)
						// 	list, err := clients.RegionClient.Nodes().List()
						// 	handleErr(err)
						// 	for _, v := range list {
						// 		if node.InternalIP == v.InternalIP {
						// 			hostNode = v
						// 			timer.Stop()
						// 			gotNode = true
						// 			//todo  初始化其它节点失败判定
						// 		}
						// 	}
						// }
						// fmt.Println("添加节点成功，正在初始化")
						// tableC := termtables.CreateTable()
						// var header []string
						// var content []string
						// for {
						// 	time.Sleep(3 * time.Second)
						// 	list, err := clients.RegionClient.Nodes().List()
						// 	handleErr(err)
						// 	select {
						// 	case <-timer.C:
						// 		fmt.Println("添加节点超时，请检查etcd")
						// 		return nil
						// 	default:
						// 		for _, v := range list {
						// 			if node.InternalIP == v.InternalIP {
						// 				hostNode = v
						// 				break
						// 			}
						// 		}
						// 		for _, val := range hostNode.NodeStatus.Conditions {
						// 			fmt.Println("正在判断节点状态，请稍等")
						// 			if hostNode.Alived || (val.Type == client.NodeInit && val.Status == client.ConditionTrue) {
						// 				fmt.Printf("节点 %s 初始化成功", hostNode.ID)
						// 				fmt.Println()
						// 				header = append(header, string(val.Type))
						// 				content = append(content, string(val.Status))
						// 				tableC.AddHeaders(header)
						// 				tableC.AddRow(content)
						// 				fmt.Println(tableC.Render())
						// 				return nil
						// 			} else if val.Type == client.NodeInit && val.Status == client.ConditionFalse {
						// 				fmt.Printf("节点 %s 初始化失败:%s", hostNode.ID, val.Reason)
						// 				return nil
						// 			} else {
						// 				fmt.Printf("..")
						// 			}
						// 		}
						// 	}
						// }

						// fmt.Println("节点初始化结束")
						return nil
					}
					return errors.New("role must not null")
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
