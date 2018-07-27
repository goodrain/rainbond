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
	//show node detail
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

	serviceTable2 := termtables.CreateTable()
	serviceTable2.AddHeaders("Service", "Status", "Message")
	serviceStatusInfo := getServicesHealthy(list)
	for name, v := range serviceStatusInfo {
		status, message := summaryResult(v)
		serviceTable2.AddRow(name, status, message)
	}
	fmt.Println(serviceTable2.Render())

	return nil
}

func getServicesHealthy(nodes []*client.HostNode) (map[string][]map[string]string) {

	StatusMap := make(map[string][]map[string]string, 30)
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
	}
	return StatusMap
}

func summaryResult(list []map[string]string) (status string, errMessage string) {
	upNum := 0
	err := "N/A"
	for _, v := range list {
		if v["status"] == "True" {
			upNum += 1
		} else {
			err = ""
			err = err + v["hostname"] + ":" + v["message"] + "/"
		}
	}

	status = strconv.Itoa(upNum) + "/" + strconv.Itoa(len(list))
	errMessage = err
	return
}
