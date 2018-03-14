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
	"github.com/urfave/cli"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/pkg/grctl/clients"
	"strings"
	"errors"
	"github.com/tidwall/gjson"
	"fmt"
	"net/url"
	"github.com/gorilla/websocket"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"encoding/json"
	"github.com/apcera/termtables"
	"github.com/gosuri/uitable"
	"time"
	"github.com/goodrain/rainbond/pkg/api/util"
	"os"
	"strconv"
	"crypto/sha256"
	"os/exec"
	"github.com/goodrain/rainbond/cmd/grctl/option"
	"path"
)

func NewCmdService() cli.Command {
	c := cli.Command{
		Name:  "service",
		Usage: "服务相关，grctl service -h",
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
				Usage: "获取应用运行详细信息。grctl service get PATH",
				Action: func(c *cli.Context) error {
					Common(c)
					return getAppInfoV2(c)
				},
			},
			cli.Command{
				Name:  "start",
				Usage: "启动应用 grctl service start goodrain/gra564a1 eventID",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "f",
						Usage: "添加此参数日志持续输出。",
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
				Usage: "停止应用 grctl service stop goodrain/gra564a1 eventID",
				Flags: []cli.Flag{
					cli.BoolFlag{
						Name:  "f",
						Usage: "添加此参数日志持续输出。",
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
						Usage: "添加此参数日志持续输出。",
					},
					cli.StringFlag{
						Name:  "event_log_server",
						Usage: "event log server address",
					},
				},
				Usage: "获取某个操作的日志 grctl service event eventID 123/gr2a2e1b ",
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
						Usage: "添加此参数日志持续输出。",
					},
				},
				Usage: "获取服务的日志。grctl service log SERVICE_ID",
				Action: func(c *cli.Context) error {
					Common(c)
					return getLogInfo(c)
				},
			},
		},

	}
	return c
}


func GetEventLogf(eventID ,server string) {

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
		ts:=c.Args().Get(1)
		tas:=strings.Split(ts,"/")
		dl,err:=clients.RegionClient.Tenants().Get(tas[0]).Services().EventLog(tas[1],eventID,"debug")
		if err != nil {
			return err
		}

		for _,v:=range dl{
			aa,_:=json.Marshal(v)
			fmt.Println(string(aa))
		}
	}
	return nil
}


func stopTenantService(c *cli.Context) error  {
	//GET /v2/tenants/{tenant_name}/services/{service_alias}
	//POST /v2/tenants/{tenant_name}/services/{service_alias}/stop

	tenantID := c.Args().First()
	eventID:=c.Args().Get(1)
	services,err:=clients.RegionClient.Tenants().Get(tenantID).Services().List()
	handleErr(err)
	for _,service:=range services{
		err:=clients.RegionClient.Tenants().Get(tenantID).Services().Stop(service.ServiceAlias,eventID)
		if c.Bool("f") {
			server := "127.0.0.1:6363"
			if c.String("event_log_server") != "" {
				server = c.String("event_log_server")
			}
			GetEventLogf(eventID,server)
		}
		//err = region.StopService(service["service_id"].(string), service["deploy_version"].(string))
		if err != nil {
			logrus.Error("停止应用失败:" + err.Error())
			return err
		}
	}
	return nil
}

func startService(c *cli.Context) error  {
	//GET /v2/tenants/{tenant_name}/services/{service_alias}
	//POST /v2/tenants/{tenant_name}/services/{service_alias}/stop

	// goodrain/gra564a1
	serviceAlias := c.Args().First()
	info := strings.Split(serviceAlias, "/")

	eventID:=c.Args().Get(1)


	service,err:=clients.RegionClient.Tenants().Get(info[0]).Services().Get(info[1])
	handleErr(err)
	if service==nil {
		return errors.New("应用不存在:"+info[1])
	}
	err=clients.RegionClient.Tenants().Get(info[0]).Services().Start(info[1],eventID)
	handleErr(err)
	if c.Bool("f") {
		server := "127.0.0.1:6363"
		if c.String("event_log_server") != "" {
			server = c.String("event_log_server")
		}
		GetEventLogf(eventID,server)
	}

	//err = region.StopService(service["service_id"].(string), service["deploy_version"].(string))
	if err != nil {
		logrus.Error("启动应用失败:" + err.Error())
		return err
	}
	return nil
}


func stopService(c *cli.Context) error {

	serviceAlias := c.Args().First()
	info := strings.Split(serviceAlias, "/")

	eventID:=c.Args().Get(1)
	service,err:=clients.RegionClient.Tenants().Get(info[0]).Services().Get(info[1])
	handleErr(err)
	if service==nil {
		return errors.New("应用不存在:"+info[1])
	}
	err=clients.RegionClient.Tenants().Get(info[0]).Services().Stop(info[1],eventID)
	handleErr(err)
	if c.Bool("f") {
		server := "127.0.0.1:6363"
		if c.String("event_log_server") != "" {
			server = c.String("event_log_server")
		}
		GetEventLogf(eventID,server)
	}
	return nil
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
	service,err := clients.RegionClient.Tenants().Get(tenantName).Services().Get(serviceAlias)
	handleErr(err)
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
	handleErr(util.CreateAPIHandleError(500,error))

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