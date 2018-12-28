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

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/api"
	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/node/nodem/controller"
	"github.com/goodrain/rainbond/node/nodem/healthy"
	"github.com/goodrain/rainbond/node/nodem/info"
	"github.com/goodrain/rainbond/node/nodem/monitor"
	"github.com/goodrain/rainbond/node/nodem/service"
	"github.com/goodrain/rainbond/node/nodem/taskrun"
	"github.com/goodrain/rainbond/util"
)

//NodeManager node manager
type NodeManager struct {
	currentNode *client.HostNode
	ctx         context.Context
	cancel      context.CancelFunc
	cluster     client.ClusterClient
	monitor     monitor.Manager
	healthy     healthy.Manager
	controller  controller.Manager
	taskrun     taskrun.Manager
	cfg         *option.Conf
	apim        *api.Manager
	clm         *logger.ContainerLogManage
}

//NewNodeManager new a node manager
func NewNodeManager(conf *option.Conf) (*NodeManager, error) {
	healthyManager := healthy.CreateManager()
	cluster := client.NewClusterClient(conf)
	taskrun, err := taskrun.Newmanager(conf)
	if err != nil {
		return nil, err
	}
	monitor, err := monitor.CreateManager(conf)
	if err != nil {
		return nil, err
	}
	clm := logger.CreatContainerLogManage(conf)
	controller := controller.NewManagerService(conf, healthyManager, cluster)
	uid, err := util.ReadHostID(conf.HostIDFile)
	if err != nil {
		return nil, fmt.Errorf("Get host id error:%s", err.Error())
	}
	ctx, cancel := context.WithCancel(context.Background())
	nodem := &NodeManager{
		cfg:         conf,
		ctx:         ctx,
		cancel:      cancel,
		taskrun:     taskrun,
		cluster:     cluster,
		monitor:     monitor,
		healthy:     healthyManager,
		controller:  controller,
		clm:         clm,
		currentNode: &client.HostNode{ID: uid},
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
	if n.currentNode.Role.HasRule("compute") {
		if err := n.clm.Start(); err != nil {
			return err
		}
	}
	go n.monitor.Start(errchan)
	//go n.taskrun.Start(errchan)
	go n.heartbeat()
	return nil
}

//Stop Stop
func (n *NodeManager) Stop() {
	n.cancel()
	n.cluster.DownNode(n.currentNode)
	if n.taskrun != nil {
		n.taskrun.Stop()
	}
	if n.controller != nil {
		n.controller.Stop()
	}
	if n.monitor != nil {
		n.monitor.Stop()
	}
	if n.healthy != nil {
		n.healthy.Stop()
	}
	if n.clm != nil {
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
	for _, v := range *services {
		result, ok := n.healthy.GetServiceHealthy(v.Name)
		if ok {
			if result.Status != service.Stat_healthy {
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
		//TODO:Judge state
		allServiceHealth := n.healthy.GetServiceHealth()
		allHealth := true
		n.currentNode.NodeStatus.AdviceAction = nil
		n.currentNode.NodeStatus.Conditions = nil
		for k, v := range allServiceHealth {
			if ser := n.controller.GetService(k); ser != nil {
				if ser.ServiceHealth != nil {
					maxNum := ser.ServiceHealth.MaxErrorsNum
					if maxNum < 2 {
						maxNum = 2
					}
					if v.Status != service.Stat_healthy && v.ErrorNumber > maxNum {
						allHealth = false
						n.currentNode.UpdataCondition(
							client.NodeCondition{
								Type:               client.NodeConditionType(ser.Name),
								Status:             client.ConditionFalse,
								LastHeartbeatTime:  time.Now(),
								LastTransitionTime: time.Now(),
								Message:            v.Info,
								Reason:             "NotHealth",
							})
					}
					if v.Status == service.Stat_healthy {
						old := n.currentNode.GetCondition(client.NodeConditionType(ser.Name))
						if old == nil || old.Status == client.ConditionFalse {
							n.currentNode.UpdataCondition(
								client.NodeCondition{
									Type:               client.NodeConditionType(ser.Name),
									Status:             client.ConditionTrue,
									LastHeartbeatTime:  time.Now(),
									LastTransitionTime: time.Now(),
									Reason:             "Health",
								})
						}
					}
					if n.cfg.AutoUnschedulerUnHealthDuration == 0 {
						continue
					}
					if v.ErrorDuration > n.cfg.AutoUnschedulerUnHealthDuration && n.cfg.AutoScheduler {
						n.currentNode.NodeStatus.AdviceAction = []string{"unscheduler"}
					}
				} else {
					old := n.currentNode.GetCondition(client.NodeConditionType(ser.Name))
					if old == nil {
						n.currentNode.UpdataCondition(
							client.NodeCondition{
								Type:               client.NodeConditionType(ser.Name),
								Status:             client.ConditionTrue,
								LastHeartbeatTime:  time.Now(),
								LastTransitionTime: time.Now(),
							})
					}
				}
			}
		}
		if allHealth && n.cfg.AutoScheduler {
			n.currentNode.NodeStatus.AdviceAction = []string{"scheduler"}
		}
		n.currentNode.NodeStatus.Status = "running"
		if err := n.cluster.UpdateStatus(n.currentNode, n.getInitLable(n.currentNode)); err != nil {
			logrus.Errorf("update node status error %s", err.Error())
		}
		logrus.Infof("Send node %s heartbeat to master:%s ", n.currentNode.ID, n.currentNode.NodeStatus.Status)
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
	n.setNodeLabels(node)
	if node.NodeStatus.NodeInfo.OperatingSystem == "" {
		node.NodeStatus.NodeInfo = info.GetSystemInfo()
	}
	if node.AvailableMemory == 0 {
		node.AvailableMemory = int64(node.NodeStatus.NodeInfo.MemorySize)
	}
	if node.AvailableCPU == 0 {
		node.AvailableCPU = int64(runtime.NumCPU())
	}
	//update node mode
	node.Mode = n.cfg.RunMode
	*n.currentNode = *node
	return nil
}

func (n *NodeManager) setNodeLabels(node *client.HostNode) {
	if node.Labels == nil {
		node.Labels = n.getInitLable(node)
		return
	}
	for k, v := range n.getInitLable(node) {
		node.Labels[k] = v
	}
}
func (n *NodeManager) getInitLable(node *client.HostNode) map[string]string {
	node.Role = strings.Split(n.cfg.NodeRule, ",")
	lables := map[string]string{}
	for _, rule := range node.Role {
		lables["rainbond_node_rule_"+rule] = "true"
	}
	lables[client.LabelOS] = runtime.GOOS
	hostname, _ := os.Hostname()
	if node.HostName != hostname && hostname != "" {
		node.HostName = hostname
	}
	lables["rainbond_node_hostname"] = node.HostName
	lables["rainbond_node_ip"] = node.InternalIP
	return lables
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
	node.UpdataCondition(client.NodeCondition{
		Type:               client.NodeInit,
		Status:             client.ConditionTrue,
		LastHeartbeatTime:  time.Now(),
		LastTransitionTime: time.Now(),
	})
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
