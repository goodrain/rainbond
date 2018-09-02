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
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/apcera/termtables"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/cmd/grctl/option"
	"github.com/goodrain/rainbond/grctl/clients"
	coreutil "github.com/goodrain/rainbond/util"
	"github.com/gorilla/websocket"
	"github.com/gosuri/uitable"
	"github.com/tidwall/gjson"
	"github.com/urfave/cli"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//NewCmdService application service command
func NewCmdService() cli.Command {
	c := cli.Command{
		Name:  "service",
		Usage: "about  application service operation，grctl service -h",
		Subcommands: []cli.Command{
			cli.Command{
				Name: "get",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "url",
						Value: "",
						Usage: "URL of the app. eg. https://user.goodrain.com/apps/goodrain/dev-debug/detail/ or goodrain/dev-debug",
					},
				},
				Usage: "Get application service runtime detail ifno。For example <grctl service get PATH>",
				Action: func(c *cli.Context) error {
					Common(c)
					return getAppInfoV2(c)
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
			cli.Command{
				Name: "log",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "f",
						Usage: "Blocks the output service log",
					},
				},
				Usage: "Get application service run log。For example <grctl service log SERVICE_ID>",
				Action: func(c *cli.Context) error {
					Common(c)
					return getLogInfo(c)
				},
			},
		},
	}
	return c
}

func GetEventLogf(eventID, server string) {

	//if c.String("event_log_server") != "" {
	//	server = c.String("event_log_server")
	//}
	u := url.URL{Scheme: "ws", Host: server, Path: "event_log"}
	logrus.Infof("connecting to %s", u.String())
	con, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		logrus.Errorf("dial websocket endpoint %s error. %s", u.String(), err.Error())
		//return err
	}
	defer con.Close()

	con.WriteMessage(websocket.TextMessage, []byte("event_id="+eventID))
	defer con.Close()
	for {
		_, message, err := con.ReadMessage()
		if err != nil {
			logrus.Println("read proxy websocket message error: ", err)
			return
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
		ts := c.Args().Get(1)
		tas := strings.Split(ts, "/")
		dl, err := clients.RegionClient.Tenants(tas[0]).Services(tas[1]).EventLog(eventID, "debug")
		if err != nil {
			return err
		}
		for _, v := range dl {
			aa, _ := json.Marshal(v)
			fmt.Println(string(aa))
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
				GetEventLogf(eventID, server)
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
	info := strings.Split(serviceAlias, "/")

	eventID := coreutil.NewUUID()

	service, err := clients.RegionClient.Tenants(info[0]).Services(info[1]).Get()
	handleErr(err)
	if service == nil {
		return errors.New("应用不存在:" + info[1])
	}
	_, err = clients.RegionClient.Tenants(info[0]).Services(info[1]).Start(eventID)
	handleErr(err)
	if c.Bool("f") {
		server := "127.0.0.1:6363"
		if c.String("event_log_server") != "" {
			server = c.String("event_log_server")
		}
		GetEventLogf(eventID, server)
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
	info := strings.Split(serviceAlias, "/")

	eventID := coreutil.NewUUID()
	service, err := clients.RegionClient.Tenants(info[0]).Services(info[1]).Get()
	handleErr(err)
	if service == nil {
		return errors.New("应用不存在:" + info[1])
	}
	_, err = clients.RegionClient.Tenants(info[0]).Services(info[1]).Stop(eventID)
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

func getAppInfoV2(c *cli.Context) error {
	value := c.Args().First()
	// https://user.goodrain.com/apps/goodrain/dev-debug/detail/
	// http://dev.goodrain.org/#/team/x749pdls/region/private-center/app/gr7c6929/overview
	var tenantName, serviceAlias string
	if strings.HasPrefix(value, "http") {
		fmt.Println(value)
		info := strings.Split(value, "#")
		if len(info) < 2 {
			return errors.New("参数错误")
		}
		paths := strings.Split(info[1], "/")
		if len(paths) < 7 {
			logrus.Error("参数错误")
			return errors.New("参数错误")
		}
		tenantName = paths[2]
		serviceAlias = paths[6]
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
	service, err := clients.RegionClient.Tenants(tenantName).Services(serviceAlias).Get()
	handleErr(err)
	if service == nil {
		fmt.Println("not found")
		return nil
	}

	table := uitable.New()
	table.Wrap = true // wrap columns
	tenantID := service.TenantId
	serviceID := service.ServiceId
	//volumes:=service[""]

	table.AddRow("Namespace:", tenantID)
	table.AddRow("ServiceID:", serviceID)
	//table.AddRow("Volume:", volumes)

	option := metav1.ListOptions{LabelSelector: "name=" + serviceAlias}
	ps, err := clients.RegionClient.Tenants(tenantName).Services(serviceAlias).Pods()
	handleErr(err)

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

	services, error := clients.K8SClient.Core().Services(tenantID).List(serviceOption)
	handleErr(util.CreateAPIHandleError(500, error))

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
func GetServiceAliasID(ServiceID string) string {
	if len(ServiceID) > 11 {
		newWord := strconv.Itoa(int(ServiceID[10])) + ServiceID + strconv.Itoa(int(ServiceID[3])) + "log" + strconv.Itoa(int(ServiceID[2])/7)
		ha := sha256.New224()
		ha.Write([]byte(newWord))
		return fmt.Sprintf("%x", ha.Sum(nil))[0:16]
	}
	return ServiceID
}

// grctrl log SERVICE_ID
func getLogInfo(c *cli.Context) error {
	value := c.Args().Get(0)
	// tenantID, err := db.FindNamespaceByServiceID(value)
	// if err != nil {
	// 	logrus.Error(err.Error())
	// 	return err
	// }
	alias := GetServiceAliasID(value)
	config := option.GetConfig()
	logFilePath := path.Join(config.DockerLogPath, alias, "stdout.log")

	//logrus.Info(logFilePath)
	var cmd exec.Cmd

	if c.Bool("f") {
		tail, err := exec.LookPath("tail")
		if err != nil {
			logrus.Error("Don't find the tail.", err.Error())
			return err
		}
		cmd = exec.Cmd{
			Env:    os.Environ(),
			Path:   tail,
			Args:   []string{tail, "-f", logFilePath},
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}
	} else {
		cat, err := exec.LookPath("cat")
		if err != nil {
			logrus.Error("Don't find the cat.", err.Error())
			return err
		}
		cmd = exec.Cmd{
			Env:    os.Environ(),
			Path:   cat,
			Args:   []string{cat, logFilePath},
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}
	}
	if err := cmd.Run(); err != nil {
		logrus.Error("Log error.", err.Error())
		return err
	}
	return nil
}
