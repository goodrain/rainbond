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
	"fmt"

	"github.com/apcera/termtables"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/gosuri/uitable"
	"github.com/urfave/cli"
	"strconv"
)

//NewCmdCluster cmd for cluster
func NewCmdCluster() cli.Command {
	c := cli.Command{
		Name:  "cluster",
		Usage: "show curren cluster datacenter info",
		Action: func(c *cli.Context) error {
			Common(c)
			return getClusterInfo(c)
		},
	}
	return c
}

func getClusterInfo(c *cli.Context) error {
	//show cluster resource detail
	clusterInfo, err := clients.RegionClient.Cluster().GetClusterInfo()
	handleErr(err)
	table := uitable.New()
	table.AddRow("", "Used/Total", "Use of")
	table.AddRow("CPU", fmt.Sprintf("%2.f/%d", clusterInfo.ReqCPU, clusterInfo.CapCPU),
		fmt.Sprintf("%d", int(clusterInfo.ReqCPU*100/float32(clusterInfo.CapCPU)))+"%")
	table.AddRow("Memory", fmt.Sprintf("%d/%d", clusterInfo.ReqMem, clusterInfo.CapMem),
		fmt.Sprintf("%d", int(float32(clusterInfo.ReqMem*100)/float32(clusterInfo.CapMem)))+"%")
	table.AddRow("DistributedDisk", fmt.Sprintf("%dGb/%dGb", clusterInfo.ReqDisk/1024/1024/1024, clusterInfo.CapDisk/1024/1024/1024),
		fmt.Sprintf("%.2f", float32(clusterInfo.ReqDisk*100)/float32(clusterInfo.CapDisk))+"%")
	fmt.Println(table)

	//show services health status
	list, err := clients.RegionClient.Nodes().List()
	handleErr(err)
	serviceTable2 := termtables.CreateTable()
	serviceTable2.AddHeaders("Service", "HealthyQuantity/Total", "Message")
	serviceStatusInfo := getServicesHealthy(list)
	status, message := clusterStatus(serviceStatusInfo["Role"], serviceStatusInfo["Ready"])
	serviceTable2.AddRow("\033[0;33;33mClusterStatus\033[0m", status, message)
	for name, v := range serviceStatusInfo {
		if name == "Role" {
			continue
		}
		status, message := summaryResult(v)
		serviceTable2.AddRow(name, status, message)
	}
	fmt.Println(serviceTable2.Render())
	//show node detail
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
}

func getServicesHealthy(nodes []*client.HostNode) (map[string][]map[string]string) {

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
				upNum += 1
			} else {
				err = ""
				err = err + v["hostname"] + ":" + v["message"] + "/"
			}
		} else {
			if v["status"] == "True" {
				upNum += 1
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

func handleRoleAndStatus(list []map[string]string) bool {
	var computeFlag bool
	var manageFlag bool
	for _, v := range list {
		if v["role"] == "compute" && v["status"] == "running" {
			computeFlag = true
		}
		if v["role"] == "manage" && v["status"] == "running" {
			manageFlag = true
		}
		if (v["role"] == "compute,manage" || v["role"] == "manage,compute") && v["status"] == "running" {
			computeFlag = true
			manageFlag = true
		}
	}
	if computeFlag && manageFlag {
		return true
	} else {
		return false
	}

}

func handleNodeReady(list []map[string]string) bool {
	trueNum := 0
	for _, v := range list {
		if v["status"] == "True" {
			trueNum += 1
		}
	}
	if trueNum == len(list) {
		return true
	} else {
		return false
	}

}

func clusterStatus(roleList []map[string]string, ReadyList []map[string]string) (string, string) {
	var clusterStatus string
	var errMessage string
	readyStatus := handleNodeReady(ReadyList)
	roleStatus := handleRoleAndStatus(roleList)
	if readyStatus {
		clusterStatus = "\033[0;32;32mhealthy\033[0m"
		errMessage = ""
	} else {
		clusterStatus = "\033[0;31;31munhealthy\033[0m"
		errMessage = "There is a service exception in the cluster"
	}
	if !roleStatus {
		clusterStatus = "\033[0;33;33munavailable\033[0m"
		errMessage = "No compute nodes or management nodes are available in the cluster"
	}
	return clusterStatus, errMessage
}
