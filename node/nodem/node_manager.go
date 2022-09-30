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

package nodem

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/goodrain/rainbond/node/nodem/logger"

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/api"
	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/node/nodem/controller"
	"github.com/goodrain/rainbond/node/nodem/gc"
	"github.com/goodrain/rainbond/node/nodem/healthy"
	"github.com/goodrain/rainbond/node/nodem/info"
	"github.com/goodrain/rainbond/node/nodem/monitor"
	"github.com/goodrain/rainbond/node/nodem/service"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
)

var sandboxImage = "k8s.gcr.io/pause-amd64:latest"

//NodeManager node manager
type NodeManager struct {
	currentNode *client.HostNode
	ctx         context.Context
	cluster     client.ClusterClient
	monitor     monitor.Manager
	healthy     healthy.Manager
	controller  controller.Manager
	cfg         *option.Conf
	apim        *api.Manager
	clm         *logger.ContainerLogManage

	imageGCManager gc.ImageGCManager
}

//NewNodeManager new a node manager
func NewNodeManager(ctx context.Context, conf *option.Conf) (*NodeManager, error) {
	healthyManager := healthy.CreateManager()
	cluster := client.NewClusterClient(conf)
	monitor, err := monitor.CreateManager(ctx, conf)
	if err != nil {
		return nil, err
	}
	clm := logger.CreatContainerLogManage(conf)
	controller := controller.NewManagerService(conf, healthyManager, cluster)
	if err != nil {
		return nil, fmt.Errorf("Get host id error:%s", err.Error())
	}

	imageGCPolicy := gc.ImageGCPolicy{
		MinAge:               conf.ImageMinimumGCAge,
		ImageGCPeriod:        conf.ImageGCPeriod,
		HighThresholdPercent: int(conf.ImageGCHighThresholdPercent),
		LowThresholdPercent:  int(conf.ImageGCLowThresholdPercent),
	}
	dockerCli, _ := conf.ContainerImageCli.GetDockerClient()
	// TODO: add image gc manager using containerd client
	imageGCManager, err := gc.NewImageGCManager(dockerCli, imageGCPolicy, sandboxImage)
	if err != nil {
		return nil, fmt.Errorf("create new imageGCManager: %v", err)
	}

	nodem := &NodeManager{
		cfg:            conf,
		ctx:            ctx,
		cluster:        cluster,
		monitor:        monitor,
		healthy:        healthyManager,
		controller:     controller,
		clm:            clm,
		currentNode:    &client.HostNode{ID: conf.HostID},
		imageGCManager: imageGCManager,
	}
	return nodem, nil
}

//AddAPIManager AddApiManager
func (n *NodeManager) AddAPIManager(apim *api.Manager) error {
	n.apim = apim
	n.controller.SetAPIRoute(apim)
	return n.monitor.SetAPIRoute(apim)
}

//InitStart init start is first start module.
//it would not depend etcd
func (n *NodeManager) InitStart() error {
	if err := n.controller.Start(n.currentNode); err != nil {
		return fmt.Errorf("start node controller error,%s", err.Error())
	}
	return nil
}

//Start start
func (n *NodeManager) Start(errchan chan error) error {
	if n.cfg.EtcdCli == nil {
		return fmt.Errorf("etcd client is nil")
	}
	if err := n.init(); err != nil {
		return err
	}
	services, err := n.controller.GetAllService()
	if err != nil {
		return fmt.Errorf("get all services error,%s", err.Error())
	}
	if err := n.healthy.AddServices(services); err != nil {
		return fmt.Errorf("get all services error,%s", err.Error())
	}
	if err := n.healthy.Start(n.currentNode); err != nil {
		return fmt.Errorf("node healty start error,%s", err.Error())
	}
	if err := n.controller.Online(); err != nil {
		return err
	}
	if n.currentNode.Role.HasRule(client.ComputeNode) && n.cfg.EnableCollectLog {
		logrus.Infof("this node is %s node and enable collect conatiner log", n.currentNode.Role)
		if err := n.clm.Start(); err != nil {
			return err
		}
	} else {
		logrus.Infof("this node(%s) is not compute node or disable collect container log ,do not start container log manage", n.currentNode.Role)
	}
	//TODO: imageGCManager with containerd
	if n.cfg.EnableImageGC && n.cfg.ContainerRuntime == "docker" {
		logrus.Info("Start the image garbage collection mechanism")
		if n.currentNode.Role.HasRule(client.ManageNode) && !n.currentNode.Role.HasRule(client.ComputeNode) {
			n.imageGCManager.SetServiceImages(n.controller.ListServiceImages())
			go n.imageGCManager.Start()
		}
	}

	go n.monitor.Start(errchan)
	go n.heartbeat()
	return nil
}

//Stop Stop
func (n *NodeManager) Stop() {
	n.cluster.DownNode(n.currentNode)
	if n.controller != nil {
		n.controller.Stop()
	}
	if n.monitor != nil {
		n.monitor.Stop()
	}
	if n.healthy != nil {
		n.healthy.Stop()
	}
	if n.clm != nil && n.currentNode.Role.HasRule(client.ComputeNode) && n.cfg.EnableCollectLog {
		n.clm.Stop()
	}
}

//CheckNodeHealthy check current node healthy.
//only healthy can controller other service start
func (n *NodeManager) CheckNodeHealthy() (bool, error) {
	services, err := n.controller.GetAllService()
	if err != nil {
		return false, fmt.Errorf("get all services error,%s", err.Error())
	}
	for _, v := range services {
		result, ok := n.healthy.GetServiceHealthy(v.Name)
		if ok {
			if result.Status != service.Stat_healthy && result.Status != service.Stat_Unknow {
				return false, fmt.Errorf(result.Info)
			}
		} else {
			return false, fmt.Errorf("The data is not ready yet")
		}
	}
	return true, nil
}

func (n *NodeManager) heartbeat() {
	util.Exec(n.ctx, func() error {
		allServiceHealth := n.healthy.GetServiceHealth()
		allHealth := true
		for k, v := range allServiceHealth {
			if ser := n.controller.GetService(k); ser != nil {
				status := client.ConditionTrue
				message := ""
				reason := ""
				if ser.ServiceHealth != nil {
					maxNum := ser.ServiceHealth.MaxErrorsNum
					if maxNum < 2 {
						maxNum = 2
					}
					if v.Status != service.Stat_healthy && v.Status != service.Stat_Unknow && v.ErrorNumber > maxNum {
						allHealth = false
						status = client.ConditionFalse
						message = v.Info
						reason = "NotHealth"
					}
				}
				n.currentNode.GetAndUpdateCondition(client.NodeConditionType(ser.Name), status, reason, message)
				if n.cfg.AutoUnschedulerUnHealthDuration == 0 {
					continue
				}
				// disable from 5.2.0 2020 06 17
				// if v.ErrorDuration > n.cfg.AutoUnschedulerUnHealthDuration && n.cfg.AutoScheduler && n.currentNode.Role.HasRule(client.ComputeNode) {
				// 	n.currentNode.NodeStatus.AdviceAction = []string{"unscheduler"}
				// 	logrus.Warningf("node unhealth more than %s(service %s unhealth), will send unscheduler advice action to master", n.cfg.AutoUnschedulerUnHealthDuration.String(), ser.Name)
				// } else {
				// 	n.currentNode.NodeStatus.AdviceAction = []string{}
				// }

				n.currentNode.NodeStatus.AdviceAction = []string{}

			} else {
				logrus.Errorf("can not find service %s", k)
			}
		}
		//remove old condition
		var deleteCondition []client.NodeConditionType
		for _, con := range n.currentNode.NodeStatus.Conditions {
			if n.controller.GetService(string(con.Type)) == nil && !client.IsMasterCondition(con.Type) {
				deleteCondition = append(deleteCondition, con.Type)
			}
		}
		//node ready condition update
		n.currentNode.UpdateReadyStatus()

		if allHealth && n.cfg.AutoScheduler {
			n.currentNode.NodeStatus.AdviceAction = []string{"scheduler"}
		}
		n.currentNode.Status = "running"
		n.currentNode.NodeStatus.Status = "running"
		if err := n.cluster.UpdateStatus(n.currentNode, deleteCondition); err != nil {
			logrus.Errorf("update node status error %s", err.Error())
		}
		if n.currentNode.NodeStatus.Status != "running" {
			logrus.Infof("Send node %s heartbeat to master:%s ", n.currentNode.ID, n.currentNode.NodeStatus.Status)
		}
		return nil
	}, time.Second*time.Duration(n.cfg.TTL))
}

//init node init
func (n *NodeManager) init() error {
	node, err := n.cluster.GetNode(n.currentNode.ID)
	if err != nil {
		if err == client.ErrorNotFound {
			logrus.Warningf("do not found node %s from cluster", n.currentNode.ID)
			if n.cfg.AutoRegistNode {
				node, err = n.getCurrentNode(n.currentNode.ID)
				if err != nil {
					return err
				}
				if err := n.cluster.RegistNode(node); err != nil {
					return fmt.Errorf("node regist failure %s", err.Error())
				}
				logrus.Infof("Regist node %s hostnmae %s to cluster success", node.ID, node.HostName)
			} else {
				return fmt.Errorf("do not found node %s and AutoRegistNode parameter is false", n.currentNode.ID)
			}
		} else {
			return fmt.Errorf("find node %s from cluster failure %s", n.currentNode.ID, err.Error())
		}
	}
	if n.cfg.HostIP != "" && node.InternalIP != n.cfg.HostIP {
		node.InternalIP = n.cfg.HostIP
	}
	//update node mode
	node.Mode = n.cfg.RunMode
	//update node rule
	node.Role = strings.Split(n.cfg.NodeRule, ",")
	//update system info
	if !node.Role.HasRule("compute") {
		node.NodeStatus.NodeInfo = info.GetSystemInfo()
	}
	//set node labels
	n.setNodeLabels(node)
	*(n.currentNode) = *node
	return nil
}

func (n *NodeManager) setNodeLabels(node *client.HostNode) {
	// node info comes from etcd
	if node.Labels == nil {
		node.Labels = n.getInitLabel(node)
		return
	}
	if node.CustomLabels == nil {
		node.CustomLabels = make(map[string]string)
	}
	var newLabels = map[string]string{}
	//remove node rule labels
	for k, v := range node.Labels {
		if !strings.HasPrefix(k, "rainbond_node_rule_") {
			newLabels[k] = v
		}
	}
	for k, v := range n.getInitLabel(node) {
		newLabels[k] = v
	}
	node.Labels = newLabels
}

//getInitLabel update node role and return new lables
func (n *NodeManager) getInitLabel(node *client.HostNode) map[string]string {
	labels := map[string]string{}
	for _, rule := range node.Role {
		labels["rainbond_node_rule_"+rule] = "true"
	}
	labels[client.LabelOS] = runtime.GOOS
	hostname, _ := os.Hostname()
	if node.HostName != hostname && hostname != "" {
		node.HostName = hostname
	}
	labels["rainbond_node_hostname"] = node.HostName
	labels["kubernetes.io/hostname"] = node.InternalIP
	return labels
}

//getCurrentNode get current node info
func (n *NodeManager) getCurrentNode(uid string) (*client.HostNode, error) {
	if n.cfg.HostIP == "" {
		ip, err := util.LocalIP()
		if err != nil {
			return nil, err
		}
		n.cfg.HostIP = ip.String()
	}
	node := CreateNode(uid, n.cfg.HostIP)
	n.setNodeLabels(&node)
	node.NodeStatus.NodeInfo = info.GetSystemInfo()
	node.Mode = n.cfg.RunMode
	node.NodeStatus.Status = "running"
	return &node, nil
}

//GetCurrentNode get current node
func (n *NodeManager) GetCurrentNode() *client.HostNode {
	return n.currentNode
}

//CreateNode new node
func CreateNode(nodeID, ip string) client.HostNode {
	return client.HostNode{
		ID:         nodeID,
		InternalIP: ip,
		ExternalIP: ip,
		CreateTime: time.Now(),
		NodeStatus: client.NodeStatus{},
	}
}

//StartService start a define service
func (n *NodeManager) StartService(serviceName string) error {
	return n.controller.StartService(serviceName)
}

//StopService stop a define service
func (n *NodeManager) StopService(serviceName string) error {
	return n.controller.StopService(serviceName)
}

//UpdateConfig update service config
func (n *NodeManager) UpdateConfig() error {
	return n.controller.ReLoadServices()
}

//GetMonitorManager get monitor manager
func (n *NodeManager) GetMonitorManager() monitor.Manager {
	return n.monitor
}
