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
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/apcera/termtables"
	eventdb "github.com/goodrain/rainbond/eventlog/db"
	"github.com/goodrain/rainbond/grctl/clients"
	coreutil "github.com/goodrain/rainbond/util"
	"github.com/gorilla/websocket"
	"github.com/gosuri/uitable"
	"github.com/tidwall/gjson"
	"github.com/urfave/cli"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//conf "github.com/goodrain/rainbond/cmd/grctl/option"
)

//NewCmdService application service command
func NewCmdService() cli.Command {
	c := cli.Command{
		Name:  "service",
		Usage: "about  application service operation，grctl service -h",
		Subcommands: []cli.Command{
			cli.Command{
				Name: "list",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "tenantAlias,t",
						Value: "",
						Usage: "Specify the tenant alias",
						FilePath: GetTenantNamePath(),
					},
				},
				Usage: "list show application services runtime detail info。For example <grctl service list -t goodrain>",
				Action: func(c *cli.Context) error {
					//logrus.Warn(conf.TenantNamePath)
					Common(c)
					return showTenantServices(c)
				},
			},
			cli.Command{
				Name: "get",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "tenantAlias,t",
						Value: "",
						Usage: "Specify the tenant alias",
						FilePath: GetTenantNamePath(),
					},
				},
				Usage: "Get application service runtime detail info。For example <grctl service get <service_alias> -t goodrain>",
				Action: func(c *cli.Context) error {
					Common(c)
					return showServiceDeployInfo(c)
				},
			},
			cli.Command{
				Name:  "start",
				Usage: "Start an application service, For example <grctl service start goodrain/gra564a1>",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "f",
						Usage: "Blocks the output operation log",
					},
					cli.StringFlag{
						Name:  "tenantAlias,t",
						Value: "",
						Usage: "Specify the tenant alias",
						FilePath: GetTenantNamePath(),
					},
					cli.StringFlag{
						Name:  "event_log_server",
						Usage: "event log server address",
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)
					return startService(c)
				},
			},
			cli.Command{
				Name:  "stop",
				Usage: "Stop an application service, For example <grctl service stop goodrain/gra564a1>",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "f",
						Usage: "Blocks the output operation log",
					},
					cli.StringFlag{
						Name:  "tenantAlias,t",
						Value: "",
						Usage: "Specify the tenant alias",
						FilePath: GetTenantNamePath(),
					},
					cli.StringFlag{
						Name:  "event_log_server",
						Usage: "event log server address",
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)
					return stopService(c)
				},
			},
			cli.Command{
				Name: "event",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "f",
						Usage: "Blocks the output operation log",
					},
					cli.StringFlag{
						Name:  "tenantAlias,t",
						Value: "",
						Usage: "Specify the tenant short id",
						FilePath: GetTenantNamePath(),
					},
					cli.StringFlag{
						Name:  "event_log_server",
						Usage: "event log server address",
					},
				},
				Usage: "Blocks the output operation log, For example <grctl service event eventID 123/gr2a2e1b>",
				Action: func(c *cli.Context) error {
					Common(c)
					return getEventLog(c)
				},
			},
		},
	}
	return c
}

//GetEventLogf get event log from websocket
func GetEventLogf(eventID, server string) error {
	//if c.String("event_log_server") != "" {
	//	server = c.String("event_log_server")
	//}
	u := url.URL{Scheme: "ws", Host: server, Path: "event_log"}
	logrus.Infof("connecting to %s", u.String())
	con, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		logrus.Errorf("dial websocket endpoint %s error. %s", u.String(), err.Error())
		return err
	}
	defer con.Close()

	con.WriteMessage(websocket.TextMessage, []byte("event_id="+eventID))
	defer con.Close()
	for {
		_, message, err := con.ReadMessage()
		if err != nil {
			logrus.Println("read proxy websocket message error: ", err)
			return err
		}
		time := gjson.GetBytes(message, "time").String()
		m := gjson.GetBytes(message, "message").String()
		level := gjson.GetBytes(message, "level").String()
		fmt.Printf("[%s](%s) %s \n", strings.ToUpper(level), time, m)
	}
}
func getEventLog(c *cli.Context) error {
	eventID := c.Args().First()
	if c.Bool("f") {
		server := "127.0.0.1:6363"
		if c.String("event_log_server") != "" {
			server = c.String("event_log_server")
		}
		u := url.URL{Scheme: "ws", Host: server, Path: "event_log"}
		logrus.Infof("connecting to %s", u.String())
		con, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			logrus.Errorf("dial websocket endpoint %s error. %s", u.String(), err.Error())
			return err
		}
		defer con.Close()
		done := make(chan struct{})
		con.WriteMessage(websocket.TextMessage, []byte("event_id="+eventID))
		defer con.Close()
		defer close(done)
		for {
			_, message, err := con.ReadMessage()
			if err != nil {
				logrus.Println("read proxy websocket message error: ", err)
				return err
			}
			time := gjson.GetBytes(message, "time").String()
			m := gjson.GetBytes(message, "message").String()
			level := gjson.GetBytes(message, "level").String()
			fmt.Printf("[%s](%s) %s \n", strings.ToUpper(level), time, m)
		}
	} else {
		logdb := &eventdb.EventFilePlugin{
			HomePath: "/grdata/downloads/log/",
		}
		list, err := logdb.GetMessages(eventID, "debug")
		if err != nil {
			return err
		}
		for _, l := range list {
			fmt.Println(l.Time + ":" + l.Message)
		}
	}
	return nil
}

func stopTenantService(c *cli.Context) error {
	//GET /v2/tenants/{tenant_name}/services/{service_alias}
	//POST /v2/tenants/{tenant_name}/services/{service_alias}/stop

	tenantName := c.Args().First()
	if tenantName == "" {
		fmt.Println("Please provide tenant name")
		os.Exit(1)
	}
	eventID := coreutil.NewUUID()
	services, err := clients.RegionClient.Tenants(tenantName).Services("").List()
	handleErr(err)
	for _, service := range services {
		if service.CurStatus != "closed" && service.CurStatus != "closing" {
			_, err := clients.RegionClient.Tenants(tenantName).Services(service.ServiceAlias).Stop(eventID)
			if c.Bool("f") {
				server := "127.0.0.1:6363"
				if c.String("event_log_server") != "" {
					server = c.String("event_log_server")
				}
				return GetEventLogf(eventID, server)
			}
			if err != nil {
				logrus.Error("停止应用失败:" + err.Error())
				return err
			}
		}
	}
	fmt.Println("EventID:", eventID)
	return nil
}

func startService(c *cli.Context) error {
	//GET /v2/tenants/{tenant_name}/services/{service_alias}
	//POST /v2/tenants/{tenant_name}/services/{service_alias}/stop

	// goodrain/gra564a1
	serviceAlias := c.Args().First()
	tenantName := c.String("tenantAlias")
	info := strings.Split(serviceAlias, "/")
	if len(info) >= 2 {
		tenantName = info[0]
		serviceAlias = info[1]
	}
	if serviceAlias == "" {
		showError("tenant alias can not be empty")
	}
	if serviceAlias == "" {
		showError("service alias can not be empty")
	}
	eventID := coreutil.NewUUID()
	service, err := clients.RegionClient.Tenants(tenantName).Services(serviceAlias).Get()
	handleErr(err)
	if service == nil {
		return errors.New("Service not exist:" + serviceAlias)
	}
	_, err = clients.RegionClient.Tenants(tenantName).Services(serviceAlias).Start(eventID)
	handleErr(err)
	if c.Bool("f") {
		server := "127.0.0.1:6363"
		if c.String("event_log_server") != "" {
			server = c.String("event_log_server")
		}
		return GetEventLogf(eventID, server)
	}
	//err = region.StopService(service["service_id"].(string), service["deploy_version"].(string))
	if err != nil {
		logrus.Error("启动应用失败:" + err.Error())
		return err
	}
	fmt.Println("EventID:", eventID)
	return nil
}

func stopService(c *cli.Context) error {
	serviceAlias := c.Args().First()
	tenantName := c.String("tenantAlias")
	info := strings.Split(serviceAlias, "/")
	if len(info) >= 2 {
		tenantName = info[0]
		serviceAlias = info[1]
	}
	if serviceAlias == "" {
		showError("tenant alias can not be empty")
	}
	if serviceAlias == "" {
		showError("service alias can not be empty")
	}
	eventID := coreutil.NewUUID()
	service, err := clients.RegionClient.Tenants(tenantName).Services(serviceAlias).Get()
	handleErr(err)
	if service == nil {
		return errors.New("Service not exist:" + serviceAlias)
	}
	_, err = clients.RegionClient.Tenants(tenantName).Services(serviceAlias).Stop(eventID)
	handleErr(err)
	if c.Bool("f") {
		server := "127.0.0.1:6363"
		if c.String("event_log_server") != "" {
			server = c.String("event_log_server")
		}
		GetEventLogf(eventID, server)
	}
	fmt.Println("EventID:", eventID)
	return nil
}
func showServiceDeployInfo(c *cli.Context) error {
	serviceAlias := c.Args().First()
	tenantName := c.String("tenantAlias")
	info := strings.Split(serviceAlias, "/")
	if len(info) >= 2 {
		tenantName = info[0]
		serviceAlias = info[1]
	}
	if tenantName == "" {
		showError("tenant alias can not be empty")
	}
	if serviceAlias == "" {
		showError("service alias can not be empty")
	}
	service, err := clients.RegionClient.Tenants(tenantName).Services(serviceAlias).Get()
	handleErr(err)
	if service == nil {
		return errors.New("Service not exist:" + serviceAlias)
	}
	deployInfo, err := clients.RegionClient.Tenants(tenantName).Services(serviceAlias).GetDeployInfo()
	handleErr(err)

	table := uitable.New()
	table.Wrap = true // wrap columns
	tenantID := service.TenantId
	serviceID := service.ServiceId
	table.AddRow("Namespace:", tenantID)
	table.AddRow("ServiceID:", serviceID)
	if deployInfo.Deployment != "" {
		table.AddRow("ReplicationType:", "deployment")
		table.AddRow("ReplicationID:", deployInfo.Deployment)
	} else if deployInfo.Statefuleset != "" {
		table.AddRow("ReplicationType:", "statefulset")
		table.AddRow("ReplicationID:", deployInfo.Statefuleset)
	}
	table.AddRow("Status:", deployInfo.Status)
	fmt.Println(table)
	//show services
	serviceTable := termtables.CreateTable()
	serviceTable.AddHeaders("Name", "IP", "Port")
	for serviceID := range deployInfo.Services {
		if clients.K8SClient != nil {
			service, _ := clients.K8SClient.Core().Services(tenantID).Get(serviceID, metav1.GetOptions{})
			if service != nil {
				var ports string
				if service.Spec.Ports != nil && len(service.Spec.Ports) > 0 {
					for _, p := range service.Spec.Ports {
						ports += fmt.Sprintf("(%s:%s)", p.Protocol, p.TargetPort.String())
					}
				}
				serviceTable.AddRow(service.Name, service.Spec.ClusterIP, ports)
			}
		} else {
			serviceTable.AddRow(serviceID, "-", "-")
		}
	}
	fmt.Println("------------Service------------")
	fmt.Println(serviceTable.Render())
	//show ingress
	ingressTable := termtables.CreateTable()
	ingressTable.AddHeaders("Name", "Host")
	for ingressID := range deployInfo.Ingresses {
		if clients.K8SClient != nil {
			ingress, _ := clients.K8SClient.Extensions().Ingresses(tenantID).Get(ingressID, metav1.GetOptions{})
			if ingress != nil {
				for _, rule := range ingress.Spec.Rules {
					ingressTable.AddRow(ingress.Name, rule.Host)
				}
			}
		} else {
			ingressTable.AddRow(ingressID, "-")
		}
	}
	fmt.Println("------------Ingress------------")
	fmt.Println(ingressTable.Render())
	//show pods
	var i = 0
	for podID := range deployInfo.Pods {
		i++
		if clients.K8SClient != nil {
			pod, err := clients.K8SClient.Core().Pods(tenantID).Get(podID, metav1.GetOptions{})
			if err != nil {
				return err
			}
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
		} else {
			fmt.Printf("-------------------Pod_%d-----------------------\n", i)
			tablepod := uitable.New()
			tablepod.AddRow("PodName:", podID)
			fmt.Println(tablepod)
		}
	}
	return nil
}

func showTenantServices(ctx *cli.Context) error {
	tenantAlias := ctx.String("tenantAlias")
	if tenantAlias == "" {
		showError("tenant alias can not be empty")
	}
	services, err := clients.RegionClient.Tenants(tenantAlias).Services("").List()
	handleErr(err)
	if services != nil {
		runtable := termtables.CreateTable()
		closedtable := termtables.CreateTable()
		runtable.AddHeaders("服务别名", "应用状态", "Deploy版本", "实例数量", "内存占用")
		closedtable.AddHeaders("服务ID", "服务别名", "应用状态", "Deploy版本")
		for _, service := range services {
			if service.CurStatus != "closed" && service.CurStatus != "closing" && service.CurStatus != "undeploy" && service.CurStatus != "deploying" {
				runtable.AddRow(service.ServiceAlias, service.CurStatus, service.DeployVersion, service.Replicas, fmt.Sprintf("%d Mb", service.ContainerMemory*service.Replicas))
			} else {
				closedtable.AddRow(service.ServiceID, service.ServiceAlias, service.CurStatus, service.DeployVersion)
			}
		}
		fmt.Println("运行中的应用：")
		fmt.Println(runtable.Render())
		fmt.Println("不在运行的应用：")
		fmt.Println(closedtable.Render())
	}
	return nil
}
