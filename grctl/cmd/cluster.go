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
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	rainbondv1alpha1 "github.com/goodrain/rainbond-operator/api/v1alpha1"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/goodrain/rainbond/grctl/cluster"
	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/util/termtables"
	"github.com/gosuri/uitable"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/types"
)

//NewCmdCluster cmd for cluster
func NewCmdCluster() cli.Command {
	c := cli.Command{
		Name:  "cluster",
		Usage: "show curren cluster datacenter info",
		Subcommands: []cli.Command{
			{
				Name:  "config",
				Usage: "prints the current cluster configuration",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "namespace, ns",
						Usage: "rainbond default namespace",
						Value: "rbd-system",
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)
					return printConfig(c)
				},
			},
			{
				Name:  "upgrade",
				Usage: "upgrade cluster",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "namespace, ns",
						Usage: "rainbond default namespace",
						Value: "rbd-system",
					},
					cli.StringFlag{
						Name:     "new-version",
						Usage:    "the new version of rainbond cluster",
						Required: true,
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)
					cluster, err := cluster.NewCluster(c.String("namespace"), c.String("new-version"))
					if err != nil {
						return err
					}
					return cluster.Upgrade()
				},
			},
		},
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "namespace,ns",
				Usage: "rainbond default namespace",
				Value: "rbd-system",
			},
		},
		Action: func(c *cli.Context) error {
			Common(c)
			return getClusterInfo(c)
		},
	}
	return c
}

func getClusterInfo(c *cli.Context) error {
	namespace := c.String("namespace")
	//show cluster resource detail
	clusterInfo, err := clients.RegionClient.Cluster().GetClusterInfo()
	if err != nil {
		if err.Code == 502 {
			fmt.Println("The current cluster node manager is not working properly.")
			fmt.Println("You can query the service log for troubleshooting.")
			os.Exit(1)
		}
		fmt.Println("The current cluster api server is not working properly.")
		fmt.Println("You can query the service log for troubleshooting.")
		os.Exit(1)
	}
	healthCPUFree := fmt.Sprintf("%.2f", float32(clusterInfo.HealthCapCPU)-clusterInfo.HealthReqCPU)
	unhealthCPUFree := fmt.Sprintf("%.2f", float32(clusterInfo.UnhealthCapCPU)-clusterInfo.UnhealthReqCPU)
	healthMemFree := fmt.Sprintf("%d", clusterInfo.HealthCapMem-clusterInfo.HealthReqMem)
	unhealthMemFree := fmt.Sprintf("%d", clusterInfo.UnhealthCapMem-clusterInfo.UnhealthReqMem)
	table := uitable.New()
	table.AddRow("", "Used/Total", "Use of", "Health free", "Unhealth free")
	table.AddRow("CPU(Core)", fmt.Sprintf("%.2f/%d", clusterInfo.ReqCPU, clusterInfo.CapCPU),
		fmt.Sprintf("%d", func() int {
			if clusterInfo.CapCPU == 0 {
				return 0
			}
			return int(clusterInfo.ReqCPU * 100 / float32(clusterInfo.CapCPU))
		}())+"%", "\033[0;32;32m"+healthCPUFree+"\033[0m \t\t", unhealthCPUFree)
	table.AddRow("Memory(Mb)", fmt.Sprintf("%d/%d", clusterInfo.ReqMem, clusterInfo.CapMem),
		fmt.Sprintf("%d", func() int {
			if clusterInfo.CapMem == 0 {
				return 0
			}
			return int(float32(clusterInfo.ReqMem*100) / float32(clusterInfo.CapMem))
		}())+"%", "\033[0;32;32m"+healthMemFree+" \033[0m \t\t", unhealthMemFree)
	table.AddRow("DistributedDisk(Gb)", fmt.Sprintf("%d/%d", clusterInfo.ReqDisk/1024/1024/1024, clusterInfo.CapDisk/1024/1024/1024),
		fmt.Sprintf("%.2f", func() float32 {
			if clusterInfo.CapDisk == 0 {
				return 0
			}
			return float32(clusterInfo.ReqDisk*100) / float32(clusterInfo.CapDisk)
		}())+"%")
	fmt.Println(table)

	//show component health status
	printComponentStatus(namespace)
	//show node detail
	serviceTable := termtables.CreateTable()
	serviceTable.AddHeaders("Uid", "IP", "HostName", "NodeRole", "Status")
	list, err := clients.RegionClient.Nodes().List()
	handleErr(err)
	var rest []*client.HostNode
	for _, v := range list {
		if v.Role.HasRule("manage") || !v.Role.HasRule("compute") {
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
}

func getServicesHealthy(nodes []*client.HostNode) map[string][]map[string]string {

	StatusMap := make(map[string][]map[string]string, 30)
	roleList := make([]map[string]string, 0, 10)

	for _, n := range nodes {
		for _, v := range n.NodeStatus.Conditions {
			status, ok := StatusMap[string(v.Type)]
			if !ok {
				StatusMap[string(v.Type)] = []map[string]string{map[string]string{"type": string(v.Type), "status": string(v.Status), "message": string(v.Message), "hostname": n.HostName}}
			} else {
				list := status
				list = append(list, map[string]string{"type": string(v.Type), "status": string(v.Status), "message": string(v.Message), "hostname": n.HostName})
				StatusMap[string(v.Type)] = list
			}

		}
		roleList = append(roleList, map[string]string{"role": n.Role.String(), "status": n.NodeStatus.Status})

	}
	StatusMap["Role"] = roleList
	return StatusMap
}

func summaryResult(list []map[string]string) (status string, errMessage string) {
	upNum := 0
	err := ""
	for _, v := range list {
		if v["type"] == "OutOfDisk" || v["type"] == "DiskPressure" || v["type"] == "MemoryPressure" || v["type"] == "InstallNotReady" {
			if v["status"] == "False" {
				upNum++
			} else {
				err = ""
				err = err + v["hostname"] + ":" + v["message"] + "/"
			}
		} else {
			if v["status"] == "True" {
				upNum++
			} else {
				err = ""
				err = err + v["hostname"] + ":" + v["message"] + "/"
			}
		}
	}
	if upNum == len(list) {
		status = "\033[0;32;32m" + strconv.Itoa(upNum) + "/" + strconv.Itoa(len(list)) + " \033[0m"
	} else {
		status = "\033[0;31;31m " + strconv.Itoa(upNum) + "/" + strconv.Itoa(len(list)) + " \033[0m"
	}
	errMessage = err
	return
}

func handleNodeReady(list []map[string]string) bool {
	trueNum := 0
	for _, v := range list {
		if v["status"] == "True" {
			trueNum++
		}
	}
	if trueNum == len(list) {
		return true
	}
	return false
}

func clusterStatus(roleList []map[string]string, ReadyList []map[string]string) (string, string) {
	var clusterStatus string
	var errMessage string
	readyStatus := handleNodeReady(ReadyList)
	if readyStatus {
		clusterStatus = "\033[0;32;32mhealthy\033[0m"
		errMessage = ""
	} else {
		clusterStatus = "\033[0;31;31munhealthy\033[0m"
		errMessage = "There is a service exception in the cluster"
	}
	var computeFlag bool
	var manageFlag bool
	var gatewayFlag bool
	for _, v := range roleList {
		if strings.Contains(v["role"], "compute") && v["status"] == "running" {
			computeFlag = true
		}
		if strings.Contains(v["role"], "manage") && v["status"] == "running" {
			manageFlag = true
		}
		if strings.Contains(v["role"], "gateway") && v["status"] == "running" {
			gatewayFlag = true
		}
	}
	if !manageFlag {
		clusterStatus = "\033[0;33;33munavailable\033[0m"
		errMessage = "No management nodes are available in the cluster"
	}
	if !computeFlag {
		clusterStatus = "\033[0;33;33munavailable\033[0m"
		errMessage = "No compute nodes are available in the cluster"
	}
	if !gatewayFlag {
		clusterStatus = "\033[0;33;33munavailable\033[0m"
		errMessage = "No gateway nodes are available in the cluster"
	}
	return clusterStatus, errMessage
}

func printComponentStatus(namespace string) {
	fmt.Println("----------------------------------------------------------------------------------")
	fmt.Println()
	cmd := exec.Command("kubectl", "get", "pod", "-n", namespace, "-o", "wide")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
	fmt.Println()
}

func printConfig(c *cli.Context) error {
	var config rainbondv1alpha1.RainbondCluster
	err := clients.RainbondKubeClient.Get(context.Background(),
		types.NamespacedName{Namespace: c.String("namespace"), Name: "rainbondcluster"}, &config)
	if err != nil {
		showError(err.Error())
	}
	out, _ := yaml.Marshal(config)
	fmt.Println(string(out))
	return nil
}
