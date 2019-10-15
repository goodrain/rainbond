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

package node

import (
	"context"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/core/config"
	"github.com/goodrain/rainbond/node/core/store"
	"github.com/goodrain/rainbond/node/kubecache"
	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/util/watch"
	"k8s.io/apimachinery/pkg/api/errors"
)

//Cluster  node  controller
type Cluster struct {
	ctx              context.Context
	cancel           context.CancelFunc
	nodes            map[string]*client.HostNode
	lock             sync.Mutex
	client           *store.Client
	kubecli          kubecache.KubeClient
	currentNode      *client.HostNode
	checkInstall     chan *client.HostNode
	datacenterConfig *config.DataCenterConfig
}

//CreateCluster create node controller
func CreateCluster(kubecli kubecache.KubeClient, node *client.HostNode, datacenterConfig *config.DataCenterConfig) *Cluster {
	ctx, cancel := context.WithCancel(context.Background())
	nc := Cluster{
		ctx:              ctx,
		cancel:           cancel,
		nodes:            make(map[string]*client.HostNode, 5),
		client:           store.DefalutClient,
		kubecli:          kubecli,
		currentNode:      node,
		checkInstall:     make(chan *client.HostNode, 4),
		datacenterConfig: datacenterConfig,
	}
	return &nc
}

//Start 启动
func (n *Cluster) Start(errchan chan error) error {
	go n.loadAndWatchNodes(errchan)
	go n.installWorker()
	return nil
}

//Stop 停止
func (n *Cluster) Stop(i interface{}) {
	n.cancel()
}

func (n *Cluster) installWorker() {
	for {
		select {
		case <-n.ctx.Done():
			return
		case node := <-n.checkInstall:
			n.installNode(node)
		}
	}
}

//UpdateNode update node info
func (n *Cluster) UpdateNode(node *client.HostNode) {
	n.nodes[node.ID] = node
	saveNode := *node
	saveNode.NodeStatus.KubeNode = nil
	_, err := n.client.Put(option.Config.NodePath+"/"+node.ID, saveNode.String())
	if err != nil {
		logrus.Errorf("update node config failure %s", err.Error())
	}
}

//GetNode get rainbond node info
func (n *Cluster) GetNode(id string) *client.HostNode {
	n.lock.Lock()
	defer n.lock.Unlock()
	if node, ok := n.nodes[id]; ok {
		n.handleNodeStatus(node)
		return node
	}
	return nil
}
func (n *Cluster) getKubeNodeCount() int {
	kubeNodes, _ := n.kubecli.GetNodes()
	return len(kubeNodes)
}

//handleNodeStatus Master integrates node status and kube node status
func (n *Cluster) handleNodeStatus(v *client.HostNode) {
	if v.Status == client.NotInstalled || v.Status == client.Installing || v.Status == client.InstallFailed {
		if v.NodeStatus.Status != "running" {
			return
		}
	}
	if time.Since(v.NodeStatus.NodeUpdateTime) > time.Minute*1 {
		v.Status = client.Unknown
		v.NodeStatus.Status = client.Unknown
		r := client.NodeCondition{
			Type:               client.NodeUp,
			Status:             client.ConditionFalse,
			LastHeartbeatTime:  time.Now(),
			LastTransitionTime: time.Now(),
			Message:            "Node lost connection, state unknown",
		}
		v.UpdataCondition(r)
		v.NodeStatus.AdviceAction = append(v.NodeStatus.AdviceAction, "offline")
	} else {
		r := client.NodeCondition{
			Type:               client.NodeUp,
			Status:             client.ConditionTrue,
			LastHeartbeatTime:  time.Now(),
			LastTransitionTime: time.Now(),
			Message:            "Node lost connection, state unknown",
		}
		v.UpdataCondition(r)
		v.NodeStatus.CurrentScheduleStatus = !v.Unschedulable
		if v.Role.HasRule("compute") {
			k8sNode, err := n.kubecli.GetNode(v.ID)
			if err != nil && !errors.IsNotFound(err) {
				logrus.Errorf("get kube node %s failure %s", v.ID, err.Error())
			}
			// Update k8s node status to node status
			if k8sNode != nil {
				v.UpdataK8sCondition(k8sNode.Status.Conditions)
				if v.AvailableCPU == 0 {
					v.AvailableCPU = k8sNode.Status.Capacity.Cpu().Value()
				}
				if v.AvailableMemory == 0 {
					v.AvailableMemory = k8sNode.Status.Capacity.Memory().Value()
				}
				v.NodeStatus.KubeNode = k8sNode
				v.NodeStatus.KubeUpdateTime = time.Now()
				v.NodeStatus.CurrentScheduleStatus = !k8sNode.Spec.Unschedulable
				v.NodeStatus.NodeInfo.ContainerRuntimeVersion = k8sNode.Status.NodeInfo.ContainerRuntimeVersion
			}
		}
		if (v.Role.HasRule("manage") || v.Role.HasRule("gateway")) && !v.Role.HasRule("compute") { //manage install_success == runnint
			if v.AvailableCPU == 0 {
				v.AvailableCPU = v.NodeStatus.NodeInfo.NumCPU
			}
			if v.AvailableMemory == 0 {
				v.AvailableMemory = int64(v.NodeStatus.NodeInfo.MemorySize)
			}
		}
		//handle status
		v.Status = v.NodeStatus.Status
		if v.Role.HasRule("compute") && v.NodeStatus.KubeNode == nil {
			v.Status = "offline"
		}
		for _, con := range v.NodeStatus.Conditions {
			if con.Type == client.NodeReady {
				v.NodeStatus.NodeHealth = con.Status == client.ConditionTrue
				break
			}
		}
	}
	if v.NodeStatus.AdviceAction != nil {
		for _, action := range v.NodeStatus.AdviceAction {
			if action == "unscheduler" {
				if v.NodeStatus.KubeNode != nil && !v.NodeStatus.KubeNode.Spec.Unschedulable {
					if n.getKubeNodeCount() > 1 {
						logrus.Infof("node %s is advice set unscheduler,will do this action", v.ID)
						_, err := n.kubecli.CordonOrUnCordon(v.ID, true)
						if err != nil {
							logrus.Errorf("auto set node is unscheduler failure.")
						}
					} else {
						logrus.Warningf("node %s is advice set unscheduler,but only have one node,can not do it", v.ID)
					}
				}
			}
			if action == "scheduler" && !v.Unschedulable {
				//if node status is not scheduler
				if v.NodeStatus.KubeNode != nil && v.NodeStatus.KubeNode.Spec.Unschedulable {
					logrus.Infof("node %s is advice set scheduler,will do this action", v.ID)
					_, err := n.kubecli.CordonOrUnCordon(v.ID, false)
					if err != nil {
						logrus.Errorf("auto set node is scheduler failure.")
					}
				}
			}
			if action == "offline" {
				logrus.Warningf("node %s is advice set offline", v.ID)
				// k8s will offline node itself.
				// remove the endpoints associated with the node from etcd
				v.DelEndpoints()
			}
		}
	}
}

func (n *Cluster) loadAndWatchNodes(errChan chan error) {
	watcher := watch.New(n.client.Client, "")
	nodewatchChan, err := watcher.WatchList(n.ctx, option.Config.NodePath, "")
	if err != nil {
		errChan <- err
	}
	defer nodewatchChan.Stop()
	for ev := range nodewatchChan.ResultChan() {
		switch ev.Type {
		case watch.Added, watch.Modified:
			node := new(client.HostNode)
			if err := node.Decode(ev.GetValue()); err != nil {
				logrus.Errorf("decode node info error :%s", err)
				continue
			}
			n.handleNodeStatus(node)
			n.CacheNode(node)
		case watch.Deleted:
			node := new(client.HostNode)
			if err := node.Decode(ev.GetPreValue()); err != nil {
				logrus.Errorf("decode node info error :%s", err)
				continue
			}
			n.RemoveNode(node.ID)
		case watch.Error:
			errChan <- ev.Error
		}
	}
}

//installNode install node
//Call the ansible installation script
func (n *Cluster) installNode(node *client.HostNode) {
	//TODO:
}

//GetAllNode 获取全部节点
func (n *Cluster) GetAllNode() (nodes []*client.HostNode) {
	n.lock.Lock()
	defer n.lock.Unlock()
	for _, v := range n.nodes {
		n.handleNodeStatus(v)
		nodes = append(nodes, v)
	}
	return
}

//CacheNode 添加节点到缓存
func (n *Cluster) CacheNode(node *client.HostNode) {
	n.lock.Lock()
	defer n.lock.Unlock()
	logrus.Debugf("add or update a rainbon node id:%s hostname:%s ip:%s", node.ID, node.HostName, node.InternalIP)
	n.nodes[node.ID] = node
}

//RemoveNode 从缓存移除节点
func (n *Cluster) RemoveNode(nodeID string) {
	n.lock.Lock()
	defer n.lock.Unlock()
	if _, ok := n.nodes[nodeID]; ok {
		delete(n.nodes, nodeID)
	}
}

//GetLabelsNode 返回匹配labels的节点ID
func (n *Cluster) GetLabelsNode(labels map[string]string) []string {
	var nodes []string
	for _, node := range n.nodes {
		if checkLabels(node, labels) {
			nodes = append(nodes, node.ID)
		}
	}
	return nodes
}

func checkLabels(node *client.HostNode, labels map[string]string) bool {
	existLabels := node.MergeLabels()
	for k, v := range labels {
		if nodev := existLabels[k]; nodev != v {
			return false
		}
	}
	return true
}
