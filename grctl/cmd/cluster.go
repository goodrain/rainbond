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
	println("=====.0")
	//show cluster resource detail
	clusterInfo, err := clients.RegionClient.Cluster().GetClusterInfo()
	if err!= nil{
		println("=====>1",err.Error())
	}
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
println("=====>1.1")
	//show node detail
	list, err := clients.RegionClient.Nodes().List()
	if err!= nil{
		println("=====>2",err.Error())
	}
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
}
