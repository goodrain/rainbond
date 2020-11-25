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
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/goodrain/rainbond/util/ansible"

	"github.com/fatih/color"

	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/goodrain/rainbond/node/nodem/client"
	coreutil "github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/util/termtables"
	"github.com/gosuri/uitable"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

func handleErr(err *util.APIHandleError) {
	if err != nil {
		if err.Err != nil {
			fmt.Printf(err.String())
			os.Exit(1)
		} else {
			fmt.Printf("API return %d", err.Code)
		}
	}
}
func showError(m string) {
	fmt.Printf("Error: %s\n", m)
	os.Exit(1)
}

func showSuccessMsg(m string) {
	fmt.Printf("Success: %s\n", m)
	os.Exit(0)
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

type nodeStatusShow struct {
	status  string
	message []string
	color   color.Attribute
}

func (n nodeStatusShow) String() string {
	color := color.New(n.color)
	if len(n.message) > 0 {
		return color.Sprintf("%s(%s)", n.status, strings.Join(n.message, ","))
	}
	return color.Sprintf("%s", n.status)
}
func getStatusShow(v *client.HostNode) (status string) {
	nss := nodeStatusShow{
		status: v.Status,
		color:  color.FgGreen,
	}
	if v.Role.HasRule("compute") && !v.NodeStatus.CurrentScheduleStatus {
		nss.message = append(nss.message, "unschedulable")
		nss.color = color.FgYellow
	}
	if !v.NodeStatus.NodeHealth {
		nss.message = append(nss.message, "unhealth")
		nss.color = color.FgRed
	}
	if v.NodeStatus.Status == client.Offline {
		nss.message = append(nss.message, client.Offline)
		nss.color = color.FgRed
	}

	result := nss.String()
	if strings.Contains(result, "unknown") {
		result = "unknown"
	}
	return result
}
func handleStatus(serviceTable *termtables.Table, v *client.HostNode) {
	serviceTable.AddRow(v.ID, v.InternalIP, v.HostName, v.Role.String(), getStatusShow(v))
}

func handleConditionResult(serviceTable *termtables.Table, conditions []client.NodeCondition) {
	for _, v := range conditions {
		if v.Type == client.NodeReady {
			continue
		}
		var formatReady string
		if v.Status == client.ConditionFalse || v.Status == client.ConditionUnknown {
			if v.Type == client.OutOfDisk || v.Type == client.MemoryPressure || v.Type == client.DiskPressure || v.Type == client.InstallNotReady {
				formatReady = "\033[0;32;32m false \033[0m"
			} else {
				formatReady = fmt.Sprintf("\033[0;31;31m %s \033[0m", v.Status)
			}
		} else {
			if v.Type == client.OutOfDisk || v.Type == client.MemoryPressure || v.Type == client.DiskPressure || v.Type == client.InstallNotReady {
				formatReady = "\033[0;31;31m true \033[0m"
			} else {
				formatReady = "\033[0;32;32m true \033[0m"
			}
		}
		serviceTable.AddRow(string(v.Type), formatReady,
			v.LastHeartbeatTime.Format(time.RFC3339)[:19],
			v.LastTransitionTime.Format(time.RFC3339)[:19],
			handleMessage(string(v.Status), v.Message),
		)
	}
}

func extractReady(serviceTable *termtables.Table, conditions []client.NodeCondition, name string) {
	for _, v := range conditions {
		if string(v.Type) == name {
			var formatReady string
			if v.Status == client.ConditionFalse {
				formatReady = "\033[0;31;31m false \033[0m"
			} else {
				formatReady = "\033[0;32;32m true \033[0m"
			}
			serviceTable.AddRow("\033[0;33;33m "+string(v.Type)+" \033[0m", formatReady,
				v.LastHeartbeatTime.Format(time.RFC3339)[:19],
				v.LastTransitionTime.Format(time.RFC3339)[:19],
				handleMessage(string(v.Status), v.Message))
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
		Usage: "rainbond node manage cmd",
		Subcommands: []cli.Command{
			{
				Name:  "get",
				Usage: "get hostID/internal ip",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name: "output,o",
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)
					id := c.Args().First()
					if id == "" {
						logrus.Errorf("need args")
						return nil
					}
					v, err := clients.RegionClient.Nodes().Get(id)
					handleErr(err)
					if c.String("output") == "json" {
						jsIndent, _ := json.MarshalIndent(v, "", "\t")
						fmt.Print(string(jsIndent))
						os.Exit(0)
					}
					table := uitable.New()
					fmt.Printf("-------------------Node Information-----------------------\n")
					table.AddRow("uuid", v.ID)
					table.AddRow("host_name", v.HostName)
					table.AddRow("health", v.NodeStatus.NodeHealth)
					table.AddRow("create_time", v.CreateTime)
					table.AddRow("internal_ip", v.InternalIP)
					table.AddRow("external_ip", v.ExternalIP)
					table.AddRow("role", v.Role)
					table.AddRow("mode", v.Mode)
					table.AddRow("available_memory", fmt.Sprintf("%d GB", v.AvailableMemory/1024/1024/1024))
					table.AddRow("available_cpu", fmt.Sprintf("%d Core", v.AvailableCPU))
					table.AddRow("status", v.Status)
					table.AddRow("schedulable(set)", !v.Unschedulable)
					table.AddRow("schedulable(current)", v.NodeStatus.CurrentScheduleStatus)
					table.AddRow("version", v.NodeStatus.Version)
					table.AddRow("up", v.NodeStatus.NodeUpdateTime)
					table.AddRow("last_down_time", v.NodeStatus.LastDownTime)
					fmt.Println(table)
					fmt.Printf("-------------------Node Labels-----------------------\n")
					labeltable := uitable.New()
					// TODO: distinguish system labels and custom labels
					for k, v := range v.Labels {
						labeltable.AddRow(k, v)
					}
					fmt.Println(labeltable)
					fmt.Printf("-------------------Service health-----------------------\n")
					serviceTable := termtables.CreateTable()
					serviceTable.AddHeaders("Condition", "Health", "LastUpdateTime", "LastChangeTime", "Message")
					extractReady(serviceTable, v.NodeStatus.Conditions, "Ready")
					handleConditionResult(serviceTable, v.NodeStatus.Conditions)
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
					serviceTable.AddHeaders("Uid", "IP", "HostName", "NodeRole", "Status")
					var rest []*client.HostNode
					for _, v := range list {
						if v.Role.HasRule("manage") {
							handleStatus(serviceTable, v)
						} else {
							rest = append(rest, v)
						}
					}
					if len(rest) > 0 {
						serviceTable.AddSeparator()
					}
					for _, v := range rest {
						handleStatus(serviceTable, v)
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
						if v.Role.HasRule("compute") && v.Status != "offline" {
							nodeResource, err := clients.RegionClient.Nodes().GetNodeResource(v.ID)
							handleErr(err)
							CPURequests := strconv.FormatFloat(float64(nodeResource.CPURequests)/float64(1000), 'f', 2, 64)
							CPULimits := strconv.FormatFloat(float64(nodeResource.CPULimits)/float64(1000), 'f', 2, 64)
							serviceTable.AddRow(v.ID, v.HostName, nodeResource.CPU, nodeResource.MemR, CPURequests,
								nodeResource.MemoryRequests, CPULimits,
								nodeResource.MemoryLimits,
								nodeResource.CPURequestsR,
								nodeResource.MemoryRequestsR)
						}
					}
					fmt.Println(serviceTable.Render())
					return nil
				},
			},
			// {
			// 	Name:  "up",
			// 	Usage: "up hostID",
			// 	Action: func(c *cli.Context) error {
			// 		Common(c)
			// 		id := c.Args().First()
			// 		if id == "" {
			// 			logrus.Errorf("need hostID")
			// 			return nil
			// 		}
			// 		err := clients.RegionClient.Nodes().Up(id)
			// 		handleErr(err)
			// 		fmt.Printf("up node %s  success\n", id)
			// 		return nil
			// 	},
			// },
			// {
			// 	Name:  "down",
			// 	Usage: "down hostID",
			// 	Action: func(c *cli.Context) error {
			// 		Common(c)
			// 		id := c.Args().First()
			// 		if id == "" {
			// 			logrus.Errorf("need hostID")
			// 			return nil
			// 		}
			// 		err := clients.RegionClient.Nodes().Down(id)
			// 		handleErr(err)
			// 		fmt.Printf("down node %s  success\n", id)
			// 		return nil
			// 	},
			// },
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
					fmt.Printf("cordon node %s  success\n", id)
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
					fmt.Printf("uncordon node %s  success\n", id)
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
					fmt.Printf("delete node %s  success\n", id)
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
					serviceTable.AddHeaders("Uid", "IP", "HostName", "NodeRole", "Status")
					for _, v := range hostnodes {
						handleStatus(serviceTable, v)
					}
					return nil
				},
			},
			{
				Name:  "label",
				Usage: "handle node labels",
				Subcommands: []cli.Command{
					cli.Command{
						Name:  "add",
						Usage: "add label for the specified node",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "key",
								Value: "",
								Usage: "the label key",
							},
							cli.StringFlag{
								Name:  "val",
								Value: "",
								Usage: "the label val",
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
							if k == "" || v == "" {
								logrus.Errorf("label key or value can not be empty")
								return nil
							}
							err := clients.RegionClient.Nodes().Label(hostID).Add(k, v)
							handleErr(err)
							return nil
						},
					},
					cli.Command{
						Name:  "delete",
						Usage: "delete the label of the specified node",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "key",
								Value: "",
								Usage: "the label key",
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
							err := clients.RegionClient.Nodes().Label(hostID).Delete(k)
							handleErr(err)
							return nil
						},
					},
					cli.Command{
						Name:   "list",
						Usage:  "list the label of the specified node",
						Flags:  []cli.Flag{},
						Action: listNodeLabelsCommand,
					},
				},
			},
			{
				Name:  "condition",
				Usage: "handle node conditions, support delete and list",
				Subcommands: []cli.Command{
					cli.Command{
						Name:  "delete",
						Usage: "delete condition for the specified node",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "name,n",
								Value: "",
								Usage: "the condition type name",
							},
						},
						Action: func(c *cli.Context) error {
							Common(c)
							hostID := c.Args().First()
							if hostID == "" {
								logrus.Errorf("need hostID")
								return nil
							}
							conditionType := c.String("name")
							_, err := clients.RegionClient.Nodes().Condition(hostID).Delete(client.NodeConditionType(conditionType))
							handleErr(err)
							showSuccessMsg("delete condition success")
							return nil
						},
					},
					cli.Command{
						Name:  "list",
						Usage: "list the conditions of the specified node",
						Action: func(c *cli.Context) error {
							Common(c)
							hostID := c.Args().First()
							if hostID == "" {
								logrus.Errorf("need hostID")
								return nil
							}
							conditions, err := clients.RegionClient.Nodes().Condition(hostID).List()
							handleErr(err)
							serviceTable := termtables.CreateTable()
							serviceTable.AddHeaders("Condition", "Health", "LastUpdateTime", "LastChangeTime", "Message")
							handleConditionResult(serviceTable, conditions)
							fmt.Println(serviceTable.Render())
							return nil
						},
					},
				},
			},
			// {
			// 	Name:  "add",
			// 	Usage: "Add a node into the cluster",
			// 	Flags: []cli.Flag{
			// 		cli.StringFlag{
			// 			Name:  "hostname,host",
			// 			Usage: "The option is required",
			// 		},
			// 		cli.StringFlag{
			// 			Name:  "hosts-file-path",
			// 			Usage: "hosts file path",
			// 			Value: "/opt/rainbond/rainbond-ansible/inventory/hosts",
			// 		},
			// 		cli.StringFlag{
			// 			Name:  "config-file-path",
			// 			Usage: "ansible global config file path",
			// 			Value: "/opt/rainbond/rainbond-ansible/scripts/installer/global.sh",
			// 		},
			// 		cli.StringFlag{
			// 			Name:  "internal-ip,iip",
			// 			Usage: "The option is required",
			// 		},
			// 		cli.StringFlag{
			// 			Name:  "external-ip,eip",
			// 			Usage: "Publish the ip address for external connection",
			// 		},
			// 		cli.StringFlag{
			// 			Name:  "root-pass,p",
			// 			Usage: "Specify the root password of the target host for login, this option conflicts with private-key",
			// 		},
			// 		cli.StringFlag{
			// 			Name:  "private-key,key",
			// 			Usage: "Specify the private key file for login, this option conflicts with root-pass",
			// 		},
			// 		cli.StringSliceFlag{
			// 			Name:  "role,r",
			// 			Usage: "The option is required, the allowed values are: [manage|compute|gateway]",
			// 		},
			// 		cli.StringFlag{
			// 			Name:  "podCIDR,cidr",
			// 			Usage: "Defines the IP assignment range for the specified node, which is automatically specified if not specified",
			// 		},
			// 		cli.StringFlag{
			// 			Name:  "id",
			// 			Usage: "Specify node ID",
			// 		},
			// 		cli.BoolFlag{
			// 			Name:  "install",
			// 			Usage: "Automatic installation after addition",
			// 		},
			// 	},
			// 	Action: addNodeCommand,
			// },
			// {

			// 	Name:  "install",
			// 	Usage: "Install a exist node into the cluster",
			// 	Flags: []cli.Flag{
			// 		cli.StringFlag{
			// 			Name:  "hosts-file-path",
			// 			Usage: "hosts file path",
			// 			Value: "/opt/rainbond/rainbond-ansible/inventory/hosts",
			// 		},
			// 		cli.StringFlag{
			// 			Name:  "config-file-path",
			// 			Usage: "ansible global config file path",
			// 			Value: "/opt/rainbond/rainbond-ansible/scripts/installer/global.sh",
			// 		},
			// 	},
			// 	Action: installNodeCommand,
			// },
		},
	}
	return c
}

func isNodeReady(node *client.HostNode) bool {
	for _, v := range node.NodeStatus.Conditions {
		if strings.ToLower(string(v.Type)) == "ready" {
			if strings.ToLower(string(v.Status)) == "true" {
				return true
			}
		}
	}
	return false
}

func installNode(node *client.HostNode) {
	// start add node script
	logrus.Infof("Begin install node %s", node.ID)
	// node stauts: installing
	if _, err := clients.RegionClient.Nodes().UpdateNodeStatus(node.ID, client.Installing); err != nil {
		logrus.Errorf("update node %s status failure %s", node.ID, err.Error())
	}
	// install node
	option := ansible.NodeInstallOption{
		HostRole:   node.Role.String(),
		HostName:   node.HostName,
		InternalIP: node.InternalIP,
		RootPass:   node.RootPass,
		KeyPath:    node.KeyPath,
		NodeID:     node.ID,
		Stdin:      os.Stdin,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}

	err := ansible.RunNodeInstallCmd(option)

	if err != nil {
		logrus.Errorf("Error executing shell script %s", err.Error())
		if _, err := clients.RegionClient.Nodes().UpdateNodeStatus(node.ID, client.InstallFailed); err != nil {
			logrus.Errorf("update node %s status failure %s", node.ID, err.Error())
		}
		return
	}

	// node status success
	if _, err := clients.RegionClient.Nodes().UpdateNodeStatus(node.ID, client.InstallSuccess); err != nil {
		logrus.Errorf("update node %s status failure %s", node.ID, err.Error())
	}
	fmt.Println("------------------------------------")
	fmt.Printf("Install node %s successful \n", node.ID)
	if node.Role.HasRule("compute") {
		fmt.Printf("You can do 'grctl node up %s' to get this compute node to join the cluster workload \n", node.ID)
	}
}

func addNodeCommand(c *cli.Context) error {
	Common(c)
	if !c.IsSet("role") {
		showError("role must not null")
	}
	if c.String("internal-ip") == "" || !coreutil.CheckIP(c.String("internal-ip")) {
		showError(fmt.Sprintf("internal ip(%s) is invalid", c.String("internal-ip")))
	}
	if c.String("root-pass") != "" && c.String("private-key") != "" {
		showError("Options private-key and root-pass are conflicting")
	}
	if c.String("root-pass") == "" && c.String("private-key") == "" {
		showError("Options private-key and root-pass must set one")
	}
	var node client.APIHostNode
	role := c.StringSlice("role")
	for _, r := range role {
		if strings.Contains(r, ",") {
			node.Role.Add(strings.Split(r, ",")...)
			continue
		}
		node.Role.Add(r)
	}
	if err := node.Role.Validation(); err != nil {
		showError(err.Error())
	}
	node.HostName = c.String("hostname")
	node.RootPass = c.String("root-pass")
	node.InternalIP = c.String("internal-ip")
	node.ExternalIP = c.String("external-ip")
	node.PodCIDR = c.String("podCIDR")
	node.Privatekey = c.String("private-key")
	node.AutoInstall = false
	node.ID = c.String("id")
	renode, err := clients.RegionClient.Nodes().Add(&node)
	handleErr(err)
	if c.Bool("install") {
		nodes, err := clients.RegionClient.Nodes().List()
		handleErr(err)
		//write ansible hosts file
		WriteHostsFile(c.String("hosts-file-path"), c.String("config-file-path"), nodes)
		installNode(renode)
	} else {
		fmt.Printf("success add %s node %s \n you install it by running: grctl node install %s \n", renode.Role, renode.ID, renode.ID)
	}
	return nil
}

func installNodeCommand(c *cli.Context) error {
	Common(c)
	nodeID := c.Args().First()
	if nodeID == "" {
		showError("node id can not be empty")
	}
	node, err := clients.RegionClient.Nodes().Get(nodeID)
	handleErr(err)
	nodes, err := clients.RegionClient.Nodes().List()
	handleErr(err)
	//write ansible hosts file
	WriteHostsFile(c.String("hosts-file-path"), c.String("config-file-path"), nodes)
	installNode(node)
	return nil
}

func listNodeLabelsCommand(c *cli.Context) error {
	Common(c)
	hostID := c.Args().First()
	if hostID == "" {
		logrus.Errorf("need hostID")
		return nil
	}
	labels, err := clients.RegionClient.Nodes().Label(hostID).List()
	handleErr(err)
	labelTable := termtables.CreateTable()
	labelTable.AddHeaders("LableKey", "LableValue")
	for k, v := range labels {
		labelTable.AddRow(k, v)
	}
	fmt.Print(labelTable.Render())
	return nil
}
