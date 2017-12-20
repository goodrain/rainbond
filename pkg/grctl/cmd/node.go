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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/apcera/termtables"
	"github.com/goodrain/rainbond/pkg/grctl/clients"
	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/urfave/cli"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
)

func NewCmdShow() cli.Command {
	c := cli.Command{
		Name:  "show",
		Usage: "显示region安装完成后访问地址",
		Action: func(c *cli.Context) error {
			manageHosts := clients.NodeClient.Nodes().Rule("manage")
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

func getExternalIP(path string, node []*model.HostNode) []string {
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
func handleStatus(serviceTable *termtables.Table, ready bool, v *model.HostNode) {
	if v.Role.HasRule("compute") && !v.Role.HasRule("manage") {
		if ready {
			//	 true of false
			serviceTable.AddRow(v.ID, v.InternalIP, v.HostName, v.Role.String(), v.Alived, !v.Unschedulable, ready)
		} else {
			//scheduable==false

			serviceTable.AddRow(v.ID, v.InternalIP, v.HostName, v.Role.String(), v.Alived, false, ready)
		}
	} else if v.Role.HasRule("manage") && !v.Role.HasRule("compute") {
		//scheduable="n/a"
		serviceTable.AddRow(v.ID, v.InternalIP, v.HostName, v.Role.String(), v.Alived, "N/A", ready)
	} else if v.Role.HasRule("compute") && v.Role.HasRule("manage") {
		if !ready {
			//n/a
			serviceTable.AddRow(v.ID, v.InternalIP, v.HostName, v.Role.String(), v.Alived, "N/A", ready)
		} else {
			serviceTable.AddRow(v.ID, v.InternalIP, v.HostName, v.Role.String(), v.Alived, !v.Unschedulable, ready)
		}
	}
}
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

					nodes := clients.NodeClient.Nodes().List()
					for _, v := range nodes {
						if v.InternalIP == id {
							id = v.ID
							break
						}
					}

					n := clients.NodeClient.Nodes().Get(id)
					v := n.Node
					nodeByte, _ := json.Marshal(v)
					var out bytes.Buffer
					err := json.Indent(&out, nodeByte, "", "\t")
					if err != nil {
						logrus.Error("error format json details %s", err.Error())
						return err
					}
					fmt.Println(out.String())
					return nil
				},
			},
			{
				Name:  "list",
				Usage: "list",
				Action: func(c *cli.Context) error {
					list := clients.NodeClient.Nodes().List()
					serviceTable := termtables.CreateTable()
					serviceTable.AddHeaders("uid", "IP", "HostName", "role", "alived", "schedulable", "ready")
					var rest []*model.HostNode
					for _, v := range list {
						var ready bool
						if isNodeReady(v) {
							ready = true
						}
						if v.Role.HasRule("manage") {
							handleStatus(serviceTable, ready, v)
						} else {
							rest = append(rest, v)
						}
					}
					if len(rest) > 0 {
						serviceTable.AddSeparator()
					}
					for _, v := range rest {
						var ready bool = false
						if v.NodeStatus != nil {
							ready = true
						}
						handleStatus(serviceTable, ready, v)
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
					clients.NodeClient.Nodes().Get(id).Up()
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
					clients.NodeClient.Nodes().Get(id).Down()
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
					node := clients.NodeClient.Nodes().Get(id)
					if !node.Node.Role.HasRule("compute") {
						logrus.Errorf("计算节点支持此功能，请检查角色")
						return nil
					}
					clients.NodeClient.Nodes().Get(id).UnSchedulable()
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
					node := clients.NodeClient.Nodes().Get(id)
					if !node.Node.Role.HasRule("compute") {
						logrus.Errorf("计算节点支持此功能，请检查角色")
						return nil
					}
					clients.NodeClient.Nodes().Get(id).ReSchedulable()
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
					clients.NodeClient.Nodes().Get(id).Delete()
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
					clients.NodeClient.Nodes().Rule(rule)
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
					clients.NodeClient.Nodes().Get(hostID).Label(label)
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
					var node model.APIHostNode
					if c.IsSet("Role") {
						node.Role = append(node.Role, c.String("Role"))
						node.InternalIP = c.String("InternalIP")
						node.HostName = c.String("HostName")
						node.ExternalIP = c.String("ExternalIP")
						node.RootPass = c.String("RootPass")

						clients.NodeClient.Nodes().Add(&node)
						fmt.Println("开始初始化节点")
						for true {
							time.Sleep(3 * time.Second)
							list := clients.NodeClient.Nodes().List()
							for _, v := range list {
								if node.InternalIP == v.InternalIP {

									tableC := termtables.CreateTable()
									var header []string
									var content []string
									for _, val := range v.Conditions {
										header = append(header, string(val.Type))
										content = append(content, string(val.Status))
									}
									if v.Alived {
										fmt.Printf("节点 %s 初始化成功", v.ID)
										fmt.Println()
										tableC.AddHeaders(header)
										tableC.AddRow(content)
										fmt.Println(tableC.Render())
										return nil
									} else {
										fmt.Printf("..")
									}

									//todo  初始化其它节点失败判定
								}
							}
						}
						return nil
					}

					return errors.New("role must not null")
				},
			},
		},
	}
	return c
}

func NewCmdNodeRes() cli.Command {
	c := cli.Command{
		Name:  "noderes",
		Usage: "获取计算节点资源信息  grctl noderes",
		Action: func(c *cli.Context) error {
			Common(c)
			return getNodeWithResource(c)
		},
	}
	return c
}

func getNodeWithResource(c *cli.Context) error {
	ns, err := clients.K8SClient.Core().Nodes().List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("获取节点列表失败,details: %s", err.Error())
		return err
	}

	table := termtables.CreateTable()
	table.AddHeaders("NodeName", "Version", "CapCPU(核)", "AllocatableCPU(核)", "UsedCPU(核)", "CapMemory(M)", "AllocatableMemory(M)", "UsedMemory(M)")
	for _, v := range ns.Items {

		podList, err := clients.K8SClient.Core().Pods(metav1.NamespaceAll).List(metav1.ListOptions{FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": v.Name}).String()})
		if err != nil {

		}
		var cpuPerNode = 0
		memPerNode := 0
		for _, p := range podList.Items {
			status := string(p.Status.Phase)

			if status != "Running" {
				continue
			}
			memPerPod := 0

			memPerPod += int(p.Spec.Containers[0].Resources.Requests.Memory().Value())
			cpuOfPod := p.Spec.Containers[0].Resources.Requests.Cpu().String()
			if strings.Contains(cpuOfPod, "m") {
				cpuOfPod = strings.Replace(cpuOfPod, "m", "", -1)
			}
			cpuI, _ := strconv.Atoi(cpuOfPod)
			cpuPerNode += cpuI
			memPerNode += memPerPod
		}
		capCPU := v.Status.Capacity.Cpu().Value()
		capMem := v.Status.Capacity.Memory().Value()
		allocCPU := v.Status.Allocatable.Cpu().Value()
		allocMem := v.Status.Allocatable.Memory().Value()
		table.AddRow(v.Name, v.Status.NodeInfo.KubeletVersion, capCPU, allocCPU, float32(cpuPerNode)/1000, capMem/1024/1024, allocMem/1024/1024, memPerNode/1024/1024)
	}
	fmt.Println(table.Render())
	return nil
}
func isNodeReady(node *model.HostNode) bool {
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
func getNode(c *cli.Context) error {
	ns, err := clients.K8SClient.Core().Nodes().List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("获取节点列表失败,details: %s", err.Error())
		return err
	}
	table := termtables.CreateTable()
	table.AddHeaders("Name", "Status", "Namespace", "Unschedulable", "KubeletVersion", "Labels")

	for _, v := range ns.Items {
		cs := v.Status.Conditions
		status := "unknown"
		for _, cv := range cs {
			status = string(cv.Status)
			if strings.Contains(status, "rue") {
				status = string(cv.Type)
				break
			}
		}
		m := v.Labels
		labels := ""
		for k := range m {
			labels += k
			labels += " "
		}
		table.AddRow(v.Name, status, v.Namespace, v.Spec.Unschedulable, v.Status.NodeInfo.KubeletVersion, labels)
	}
	fmt.Println(table.Render())
	return nil
}
