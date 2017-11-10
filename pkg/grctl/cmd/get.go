package cmd

import (
	"github.com/urfave/cli"
	"fmt"
	"github.com/apcera/termtables"
	"net/url"
	"github.com/Sirupsen/logrus"
	"strings"
	"errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/goodrain/rainbond/pkg/grctl/clients"
	"github.com/gosuri/uitable"
	"time"
)

func NewCmdGet() cli.Command {
	c:=cli.Command{
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
func getAppInfoV2(c *cli.Context)error  {
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
	service:=clients.RegionClient.Tenants().Get(tenantName).Services().Get(serviceAlias)
	//result, err := db.FindTenantServiceMulti(tenantName, serviceAlias)
	//if err != nil {
	//	logrus.Error("Don't Find the service info .", err.Error())
	//	return err
	//}


	table := uitable.New()
	table.Wrap = true // wrap columns
	tenantID :=service.TenantID
	serviceID:=service.ServiceID
	volumes:=service.VolumePath

	//for _,item:=range result{
	//	tenantID=item["tenant_id"].(string)
	//
	//	serviceIds+=item["service_id"].(string)
	//	volumes+=item["volume_path"].(string)
	//	if i < len(result) {
	//		serviceIds+=" & "
	//		volumes+=" & "
	//		i++
	//	}
	//
	//}
	table.AddRow("Namespace:", tenantID)
	table.AddRow("ServiceID:", serviceID)
	table.AddRow("Volume:", volumes)

	option := metav1.ListOptions{LabelSelector: "name=" + serviceAlias}
	rcList,err:=clients.K8SClient.Core().ReplicationControllers(tenantID).List(option)
	//info, pods, err := kubernetes.GetServiceInfo(tenantID, serviceAlias)
	if err != nil {
		logrus.Error("Don't Find the ReplicationController info .", err.Error())
		return err
	}
	var rcMap = make(map[string]string)
	for _, rc := range rcList.Items {
		rcMap["Name"] = rc.Name
		rcMap["Namespace"] = rc.Namespace
		rcMap["Replicas"] = fmt.Sprintf("%d/%d", rc.Status.ReadyReplicas, rc.Status.Replicas)
		rcMap["Date"] = rc.CreationTimestamp.Format(time.RFC3339)
	}

	table.AddRow("RcName:", rcMap["Name"])
	table.AddRow("RcCreateTime:", rcMap["Date"])
	table.AddRow("PodNumber:", rcMap["Replicas"])
	serviceOption := metav1.ListOptions{LabelSelector: "name=" + serviceAlias + "Service"}
	services, err := clients.K8SClient.Core().Services(tenantID).List(serviceOption)

	var serviceMap = make(map[string]string)
	for _, service := range services.Items {
		fmt.Sprintln(service)
		serviceMap["Name"] = service.Name
		var ports string
		if service.Spec.Ports != nil && len(service.Spec.Ports) > 0 {
			for _, p := range service.Spec.Ports {
				ports += fmt.Sprintf("(%s:%s)", p.Protocol, p.TargetPort.String())
			}
		}
		serviceMap["Ports"] = ports
		serviceMap["ClusterIP"] = service.Spec.ClusterIP
	}
	if service, ok := info["service"]; ok {
		table.AddRow("K8sServiceName:", service["Name"])
		table.AddRow("K8sServiceClusterIP:", service["ClusterIP"])
		table.AddRow("K8sServicePorts:", service["Ports"])
	}
	fmt.Println(table)
	pods, err := clients.K8SClient.Core().Pods(tenantID).List(option)
	if err != nil {

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
		if pod.Spec.Volumes != nil && len(pod.Spec.Volumes) > 0 && pod.Spec.Volumes[0].HostPath != nil {
			table.AddRow("PodVolumePath:", pod.Spec.Volumes[0].HostPath.Path)
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
	return nil
}
