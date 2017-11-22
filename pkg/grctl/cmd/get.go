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
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/apcera/termtables"
	"github.com/goodrain/rainbond/pkg/grctl/clients"
	"github.com/gosuri/uitable"
	"github.com/urfave/cli"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"encoding/json"
	//"encoding/json"
)

//NewCmdGet NewCmdGet
func NewCmdGet() cli.Command {
	c := cli.Command{
		Name: "get",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "url",
				Value: "",
				Usage: "URL of the app. eg. https://user.goodrain.com/apps/goodrain/dev-debug/detail/",
			},
		},
		Usage: "获取应用运行详细信息。grctl get PATH",
		Action: func(c *cli.Context) error {
			Common(c)
			return getAppInfoV2(c)
		},
	}
	return c
}
func getAppInfoV2(c *cli.Context) error {
	value := c.Args().First()
	// https://user.goodrain.com/apps/goodrain/dev-debug/detail/
	var tenantName, serviceAlias string
	if strings.HasPrefix(value, "http") {
		url, err := url.Parse(value)
		if err != nil {
			logrus.Error("Parse the app url error.", err.Error())
		}
		paths := strings.Split(url.Path[1:], "/")
		if len(paths) < 2 {
			logrus.Error("参数错误")
			return errors.New("参数错误")
		}
		if paths[0] == "apps" {
			tenantName = paths[1]
			serviceAlias = paths[2]
		} else {
			logrus.Error("The app url is not valid", paths[0])
		}
	} else if strings.Contains(value, "/") {
		paths := strings.Split(value, "/")
		if len(paths) < 2 {
			logrus.Error("参数错误")
			return errors.New("参数错误")
		}
		tenantName = paths[0]
		serviceAlias = paths[1]
	} else {
		serviceAlias = value
	}
	service := clients.RegionClient.Tenants().Get(tenantName).Services().Get(serviceAlias)
	//result, err := db.FindTenantServiceMulti(tenantName, serviceAlias)
	//if err != nil {
	//	logrus.Error("Don't Find the service info .", err.Error())
	//	return err
	//}
	if service == nil {
		fmt.Println("not found")
		return nil
	}

	table := uitable.New()
	table.Wrap = true // wrap columns
	tenantID := service["tenantId"]
	serviceID := service["serviceId"]
	//volumes:=service[""]

	table.AddRow("Namespace:", tenantID)
	table.AddRow("ServiceID:", serviceID)
	//table.AddRow("Volume:", volumes)

	option := metav1.ListOptions{LabelSelector: "name=" + serviceAlias}
	ps, err := clients.RegionClient.Tenants().Get(tenantName).Services().Pods(serviceAlias)
	if err != nil {
		logrus.Errorf("error get pods info ,details %s", err.Error())
		return err
	}

	var rcMap = make(map[string]string)
	for _, v := range ps {
		rcMap["Type"] = v.ReplicationType
		rcMap["ID"] = v.ReplicationID
		break
	}
	table.AddRow("ReplicationType:", rcMap["Type"])
	table.AddRow("ReplicationID:", rcMap["ID"])
	serviceOption := metav1.ListOptions{}
	//grf1cdd7Service
	//serviceOption := metav1.ListOptions{LabelSelector: "spec.selector.name="+"gr2a2e1b" }

	services, err := clients.K8SClient.Core().Services(tenantID).List(serviceOption)
	if err != nil {
		logrus.Errorf("err get service by namespace %s,details %s", tenantID, err.Error())
		return err
	}

	serviceTable := termtables.CreateTable()
	serviceTable.AddHeaders("Name", "IP", "Port")

	var serviceMap = make(map[string]string)
	for _, service := range services.Items {
		if service.Spec.Selector["name"] == serviceAlias {
			serviceMap["Name"] = service.Name
			var ports string
			if service.Spec.Ports != nil && len(service.Spec.Ports) > 0 {
				for _, p := range service.Spec.Ports {
					ports += fmt.Sprintf("(%s:%s)", p.Protocol, p.TargetPort.String())
				}
			}
			serviceMap["Ports"] = ports
			serviceMap["ClusterIP"] = service.Spec.ClusterIP
			serviceTable.AddRow(service.Name, service.Spec.ClusterIP, ports)
		}
	}
	table.AddRow("Services:", "")
	fmt.Println(table)
	fmt.Println(serviceTable.Render())

	//"ServiceID": "92fdfe7e22639be491953c1fd92a2e1b",
	//	"ReplicationID": "695cdb83147041bd9b2777659e981a9a",
	//	"ReplicationType": "replicationcontroller",
	//	"PodName": "695cdb83147041bd9b2777659e981a9a-gh4pn"

	if clients.K8SClient == nil {

		for i, v := range ps {

			table := uitable.New()
			table.Wrap = true // wrap columns
			fmt.Printf("-------------------Pod_%d-----------------------\n", i)
			table.AddRow("PodName:", v.PodName)
			table.AddRow("ServiceID:", v.ServiceID)
			table.AddRow("ReplicationType:", v.ReplicationType)
			table.AddRow("ReplicationID:", v.ReplicationID)

			fmt.Println(table)
		}
	} else {

		pods, err := clients.K8SClient.Core().Pods(tenantID).List(option)
		if err != nil {
			return err
		}
		for i, pod := range pods.Items {

			table := uitable.New()
			table.Wrap = true // wrap columns
			fmt.Printf("-------------------Pod_%d-----------------------\n", i)
			table.AddRow("PodName:", pod.Name)
			status := ""
			for _, con := range pod.Status.Conditions {
				status += fmt.Sprintf("%s : %s", con.Type, con.Status) + "  "
			}
			table.AddRow("PodStatus:", status)
			table.AddRow("PodIP:", pod.Status.PodIP)
			table.AddRow("PodHostIP:", pod.Status.HostIP)
			table.AddRow("PodHostName:", pod.Spec.NodeName)
			if pod.Spec.Volumes != nil && len(pod.Spec.Volumes) > 0 {
				value := ""
				for _, v := range pod.Spec.Volumes {
					if v.HostPath != nil {
						value += v.HostPath.Path
						for _, vc := range pod.Spec.Containers {
							m := vc.VolumeMounts
							for _, v2 := range m {
								if v2.Name == v.Name {
									value += ":" + string(v2.MountPath)
								}
							}
						}
						value += "\n"
					}
				}
				table.AddRow("PodVolumePath:", value)

			}
			if pod.Status.StartTime != nil {
				table.AddRow("PodStratTime:", pod.Status.StartTime.Format(time.RFC3339))
			}
			table.AddRow("Containers:", "")
			fmt.Println(table)
			containerTable := termtables.CreateTable()
			containerTable.AddHeaders("ID", "Name", "Image", "State")
			for j := 0; j < len(pod.Status.ContainerStatuses); j++ {
				var t string
				con := pod.Status.ContainerStatuses[j]
				if con.State.Running != nil {
					t = con.State.Running.StartedAt.Format(time.RFC3339)
				}
				var conID string
				if con.ContainerID != "" {
					conID = con.ContainerID[9:21]
				}
				containerTable.AddRow(conID, con.Name, con.Image, t)
			}
			fmt.Println(containerTable.Render())
		}
	}
	return nil
}
