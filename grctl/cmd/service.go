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
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	eventdb "github.com/goodrain/rainbond/eventlog/db"
	"github.com/goodrain/rainbond/grctl/clients"
	coreutil "github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/util/constants"
	"github.com/goodrain/rainbond/util/termtables"
	"github.com/gorilla/websocket"
	"github.com/gosuri/uitable"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/urfave/cli"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
						Name:     "tenantAlias,t",
						Value:    "",
						Usage:    "Specify the tenant alias",
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
						Name:     "tenantAlias,t",
						Value:    "",
						Usage:    "Specify the tenant alias",
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
						Name:     "tenantAlias,t",
						Value:    "",
						Usage:    "Specify the tenant alias",
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
						Name:     "tenantAlias,t",
						Value:    "",
						Usage:    "Specify the tenant alias",
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
						Name:     "tenantAlias,t",
						Value:    "",
						Usage:    "Specify the tenant short id",
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
			HomePath: constants.GrdataLogPath,
		}
		list, err := logdb.GetMessages(eventID, "debug", 0)
		if err != nil {
			return err
		}
		if list != nil {
			for _, l := range list.(eventdb.MessageDataList) {
				fmt.Println(l.Time + ":" + l.Message)
			}
		}
	}
	return nil
}

func stopTenantService(c *cli.Context) error {
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
	tenant, err := clients.RegionClient.Tenants(tenantName).Get()
	handleErr(err)
	if tenant == nil {
		return errors.New("Tenant not exist:" + tenantName)
	}
	table := uitable.New()
	table.Wrap = true // wrap columns
	serviceID := service.ServiceID
	table.AddRow("Namespace:", tenant.Namespace)
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
			service, _ := clients.K8SClient.CoreV1().Services(tenant.Namespace).Get(context.Background(), serviceID, metav1.GetOptions{})
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
	//show endpoints
	if deployInfo.Endpoints != nil && len(deployInfo.Endpoints) > 0 {
		epTable := termtables.CreateTable()
		epTable.AddHeaders("Name", "IP", "Port", "Protocol")
		for epname := range deployInfo.Endpoints {
			if clients.K8SClient != nil {
				ep, _ := clients.K8SClient.CoreV1().Endpoints(tenant.Namespace).Get(context.Background(), epname, metav1.GetOptions{})
				if ep != nil {
					for i := range ep.Subsets {
						ss := &ep.Subsets[i]
						for j := range ss.Ports {
							port := &ss.Ports[j]
							for k := range ss.Addresses {
								address := &ss.Addresses[k]
								epTable.AddRow(ep.Name, address.IP, port.Port, port.Protocol)
							}
							for k := range ss.NotReadyAddresses {
								address := &ss.NotReadyAddresses[k]
								epTable.AddRow(ep.Name, address.IP, port.Port, port.Protocol)
							}
						}
					}
				}
			} else {
				epTable.AddRow(epname, "-", "-", "-")
			}
		}
		fmt.Println("------------endpoints------------")
		fmt.Println(epTable.Render())
	}
	//show ingress
	ingressTable := termtables.CreateTable()
	ingressTable.AddHeaders("Name", "Host")
	for ingressID := range deployInfo.Ingresses {
		if clients.K8SClient != nil {
			ingress, _ := clients.K8SClient.ExtensionsV1beta1().Ingresses(tenant.Namespace).Get(context.Background(), ingressID, metav1.GetOptions{})
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
			pod, err := clients.K8SClient.CoreV1().Pods(tenant.Namespace).Get(context.Background(), podID, metav1.GetOptions{})
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

			name2Path := make(map[string]string)
			if len(pod.Spec.Containers) > 0 {
				container := pod.Spec.Containers[0]
				for _, cvm := range container.VolumeMounts {
					name2Path[cvm.Name] = cvm.MountPath
				}
			}

			if pod.Status.StartTime != nil {
				table.AddRow("PodStratTime:", pod.Status.StartTime.Format(time.RFC3339))
			}
			fmt.Println(table)

			fmt.Println("PodVolume:")
			volumeTable := termtables.CreateTable()
			volumeTable.AddHeaders("Volume", "Type", "Volume Mount")
			for _, vol := range pod.Spec.Volumes {
				// only PersistentVolumeClaim
				if vol.PersistentVolumeClaim == nil {
					continue
				}

				claimName := vol.PersistentVolumeClaim.ClaimName
				pvc, _ := clients.K8SClient.CoreV1().PersistentVolumeClaims(tenant.Namespace).Get(context.Background(), claimName, metav1.GetOptions{})
				if pvc != nil {
					pvn := pvc.Spec.VolumeName
					volumeMount := name2Path[vol.Name]
					pv, _ := clients.K8SClient.CoreV1().PersistentVolumes().Get(context.Background(), pvn, metav1.GetOptions{})
					if pv != nil {
						switch {
						case pv.Spec.HostPath != nil:
							volumeTable.AddRow(volumeMount, "hostPath", pv.Spec.HostPath.Path)
						case pv.Spec.NFS != nil:
							volumeTable.AddRow(volumeMount, "nfs", "server: "+pv.Spec.NFS.Server)
							volumeTable.AddRow("", "", "path: "+pv.Spec.NFS.Path)
						case pv.Spec.Glusterfs != nil:
							volumeTable.AddRow(volumeMount, "glusterfs", "endpoints: "+pv.Spec.Glusterfs.EndpointsName)
							volumeTable.AddRow("", "", "path: "+pv.Spec.Glusterfs.Path)
							if pv.Spec.Glusterfs.EndpointsNamespace != nil {
								volumeTable.AddRow("", "", "endpointsNamespace: "+*pv.Spec.Glusterfs.EndpointsNamespace)
							}
						case pv.Spec.CSI != nil:
							switch pv.Spec.CSI.Driver {
							case "nasplugin.csi.alibabacloud.com":
								volumeTable.AddRow(volumeMount, pv.Spec.CSI.Driver, "server: "+pv.Spec.CSI.VolumeAttributes["server"])
								volumeTable.AddRow("", "", "path: "+pv.Spec.CSI.VolumeAttributes["path"])
							case "diskplugin.csi.alibabacloud.com":
								volumeTable.AddRow(volumeMount, pv.Spec.CSI.Driver, "type: "+pv.Spec.CSI.VolumeAttributes["type"])
								volumeTable.AddRow("", "", "storage.kubernetes.io/csiProvisionerIdentity"+pv.Spec.CSI.VolumeAttributes["storage.kubernetes.io/csiProvisionerIdentity"])
							}
						}
					}
				}
			}
			fmt.Println(volumeTable.Render())

			fmt.Println("Containers:")
			containerTable := termtables.CreateTable()
			containerTable.AddHeaders("ID", "Name", "Image", "State")
			for j := 0; j < len(pod.Status.ContainerStatuses); j++ {
				cstatus := pod.Status.ContainerStatuses[j]
				cid, s := getContainerIDAndState(cstatus)
				containerTable.AddRow(cid, cstatus.Name, cstatus.Image, s)
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

func getContainerIDAndState(status corev1.ContainerStatus) (cid, s string) {
	state := status.State
	containerID := status.ContainerID
	if state.Running != nil {
		s = fmt.Sprintf("Running(%s)", state.Running.StartedAt.Format(time.RFC3339))
	}
	if state.Waiting != nil {
		s = "Waiting"
	}
	if state.Terminated != nil {
		s = "Terminated"
		containerID = state.Terminated.ContainerID
	}
	if containerID != "" {
		cid = containerID[9:21]
	}
	return
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
