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
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/goodrain/rainbond/util/watch"

	"github.com/Sirupsen/logrus"
	"github.com/pquerna/ffjson/ffjson"

	"github.com/coreos/etcd/mvcc/mvccpb"

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/api/model"
	"github.com/goodrain/rainbond/node/core/config"
	"github.com/goodrain/rainbond/node/core/store"
	"github.com/goodrain/rainbond/node/kubecache"
	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/util"
	"encoding/json"
)

const (
	Running     = "running"
	Offline     = "offline"
	Unknown     = "unknown"
	Error       = "error"
	Init        = "init"
	InitSuccess = "init_success"
	InitFailed  = "init_failed"
	Installing  = "installing"
)

//Cluster  node  controller
type Cluster struct {
	ctx              context.Context
	cancel           context.CancelFunc
	nodes            map[string]*client.HostNode
	nodeonline       map[string]string
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
		nodeonline:       make(map[string]string, 10),
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
	go n.loadAndWatchNodeOnlines(errchan)
	go n.worker()
	go n.checkNodeStatus()
	return nil
}

//Stop 停止
func (n *Cluster) Stop(i interface{}) {
	n.cancel()
}

func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return true
}

func (n *Cluster) checkNodeStatus() {
	logrus.Info("Starting check node status module.")
	var max = 30
	var unhealthyCounter = make(map[string]int, 10)
	var tick = time.Tick(time.Second * 2)

	for {
		select {
		case <-tick:
		case <-n.ctx.Done():
			return
		}
		nodes := n.GetAllNode()
		for _, node := range nodes {
			n.handleNodeStatus(node)
			ready := isReady(node.NodeStatus.Conditions)
			logrus.Debugf("Node %s status is %v %d times.",
				node.ID, ready, unhealthyCounter[node.ID])
			if !ready {
				if max <= unhealthyCounter[node.ID] {
					// delete the node in k8s if type is compute
					if node.Role.HasRule(client.ComputeNode) {
						logrus.Infof("Node %s status is %v %d times and can not scheduling.",
							node.ID, ready, unhealthyCounter[node.ID])
						_, err := n.kubecli.CordonOrUnCordon(node.ID, true)
						if err != nil {
							logrus.Error("Failed to delete node in k8s: ", err)
						}
					}
				} else {
					unhealthyCounter[node.ID]++
				}
			} else if ready {
				resp, err := store.DefalutClient.Get("/rainbond/nodes/target/" + node.ID)
				if err != nil {
					logrus.Error(err)
					continue
				}
				var targetNode client.HostNode
				err = json.Unmarshal(resp.Kvs[0].Value, &targetNode)
				if err != nil {
					logrus.Error(err)
					continue
				}
				if targetNode.NodeStatus.Status != Running || targetNode.Unschedulable {
					logrus.Info("Skip open scheduling, because target node is: ", targetNode.NodeStatus.Status)
					continue
				}
				unhealthyCounter[node.ID] = 0
				// add the node into k8s if type is compute
				if node.Role.HasRule(client.ComputeNode) {
					logrus.Infof("Node %s status is %v and can scheduling.", node.ID, ready)
					_, err := n.kubecli.CordonOrUnCordon(node.ID, false)
					if err != nil {
						logrus.Error("Failed to add node into k8s: ", err)
					}
				}
			}
		}
	}
}

func isReady(conditions []client.NodeCondition) bool {
	for _, c := range conditions {
		if c.Type == client.NodeReady {
			if c.Status == client.ConditionTrue {
				return true
			} else {
				return false
			}
		}
	}
	return false
}

//RegToHost regist node id to hosts file
func RegToHost(node *client.HostNode, opt string) {

}

func (n *Cluster) worker() {
	for {
		select {
		case <-n.ctx.Done():
			return
		case newNode := <-n.checkInstall:
			go n.checkNodeInstall(newNode)
			//其他异步任务
		}
	}
}

//UpdateNode 更新节点信息
func (n *Cluster) UpdateNode(node *client.HostNode) {
	n.lock.Lock()
	defer n.lock.Unlock()
	n.nodes[node.ID] = node
	// the value is etcd path of save node status
	// It should come from the cmd/node/option/config object, but I found it too cumbersome to pass a value.
	key := "/rainbond/nodes/target/" + node.ID
	n.client.Put(key, node.String())
}

//UpdateNode 更新节点信息
func (n *Cluster) UnlockUpdateNode(node *client.HostNode) {
	n.nodes[node.ID] = node
	n.client.Put(option.Config.NodePath+"/"+node.ID, node.String())
}

func (n *Cluster) getNodeFromKV(kv *mvccpb.KeyValue) *client.HostNode {
	var node client.HostNode
	if err := ffjson.Unmarshal(kv.Value, &node); err != nil {
		logrus.Error("parse node info error:", err.Error())
		return nil
	}
	return &node
}
func (n *Cluster) getNodeIDFromKey(key string) string {
	index := strings.LastIndex(key, "/")
	if index < 0 {
		return ""
	}
	id := key[index+1:]
	return id
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
func (n *Cluster) handleNodeStatus(v *client.HostNode) {
	if v.Role.HasRule("compute") {
		k8sNode, err := n.kubecli.GetNode(v.ID)
		status := Running
		if err != nil {
			logrus.Infof("get k8s node error:%s", err.Error())
			status = Offline
			v.Status = status
			v.NodeStatus.Status = status
			v.Unschedulable = true
			r := client.NodeCondition{
				Type:               client.NodeReady,
				Status:             client.ConditionFalse,
				LastHeartbeatTime:  time.Now(),
				LastTransitionTime: time.Now(),
				Message:            "The node has been offline",
			}
			v.UpdataCondition(r)
			return
		}
		v.Unschedulable = false
		if k8sNode != nil {
			v.UpdataK8sCondition(k8sNode.Status.Conditions)
			if v.Unschedulable == true || k8sNode.Spec.Unschedulable == true{
				v.Unschedulable = true
			}
			v.AvailableCPU = k8sNode.Status.Capacity.Cpu().Value()
			v.AvailableMemory = k8sNode.Status.Capacity.Memory().Value()
		}
		if time.Now().Sub(v.UpTime) > time.Minute*2 {
			status = Unknown
			v.Status = status
			v.NodeStatus.Status = status
			v.Unschedulable = true
			r := client.NodeCondition{
				Type:               client.NodeReady,
				Status:             client.ConditionFalse,
				LastHeartbeatTime:  time.Now(),
				LastTransitionTime: time.Now(),
				Message:            "Node lost connection, state unknown",
			}
			v.UpdataCondition(r)
			return
		}

		//var haveready bool
		for _, condiction := range v.NodeStatus.Conditions {
			if condiction.Status == "True" && (condiction.Type == "OutOfDisk" || condiction.Type == "MemoryPressure" || condiction.Type == "DiskPressure") {
				v.Status = status
				v.NodeStatus.Status = status

				r := client.NodeCondition{
					Type:               client.NodeReady,
					Status:             client.ConditionFalse,
					LastHeartbeatTime:  time.Now(),
					LastTransitionTime: time.Now(),
				}
				v.UpdataCondition(r)
				return
			}
			if condiction.Status == "False" && (condiction.Type != "OutOfDisk" && condiction.Type != "MemoryPressure" && condiction.Type != "DiskPressure" && condiction.Type != "Ready") {
				v.Status = status
				v.NodeStatus.Status = status

				r := client.NodeCondition{
					Type:               client.NodeReady,
					Status:             client.ConditionFalse,
					LastHeartbeatTime:  time.Now(),
					LastTransitionTime: time.Now(),
				}
				v.UpdataCondition(r)
				return
			}
		}
		v.Status = status
		v.NodeStatus.Status = status

		r := client.NodeCondition{
			Type:               client.NodeReady,
			Status:             client.ConditionTrue,
			LastHeartbeatTime:  time.Now(),
			LastTransitionTime: time.Now(),
		}
		v.UpdataCondition(r)

	}
	if v.Role.HasRule("manage") && !v.Role.HasRule("compute") { //manage install_success == runnint
		if v.Status == Init || v.Status == InitSuccess || v.Status == InitFailed || v.Status == Installing {
			return
		}
		if v.Alived {
			if time.Now().Sub(v.UpTime) > time.Minute*2 {
				v.Status = Unknown
				v.NodeStatus.Status = Unknown
				r := client.NodeCondition{
					Type:               client.NodeReady,
					Status:             client.ConditionFalse,
					LastHeartbeatTime:  time.Now(),
					LastTransitionTime: time.Now(),
					Message:            "Node lost connection, state unknown",
				}
				v.UpdataCondition(r)
				return
			}

			for _, condition := range v.NodeStatus.Conditions {
				if condition.Type != client.NodeReady && condition.Status == "False" {

					v.Status = Running
					v.NodeStatus.Status = Running

					r := client.NodeCondition{
						Type:               client.NodeReady,
						Status:             client.ConditionFalse,
						LastHeartbeatTime:  time.Now(),
						LastTransitionTime: time.Now(),
					}
					v.UpdataCondition(r)
					return
				}

			}
			v.Status = Running
			v.NodeStatus.Status = Running
			r := client.NodeCondition{
				Type:               client.NodeReady,
				Status:             client.ConditionTrue,
				LastHeartbeatTime:  time.Now(),
				LastTransitionTime: time.Now(),
			}
			v.UpdataCondition(r)

		} else {
			v.Status = Offline
			v.NodeStatus.Status = Offline
			r := client.NodeCondition{
				Type:               client.NodeReady,
				Status:             client.ConditionFalse,
				LastHeartbeatTime:  time.Now(),
				LastTransitionTime: time.Now(),
				Message:            "The node has been offline",
			}
			v.UpdataCondition(r)
		}
	}
}
func (n *Cluster) loadAndWatchNodeOnlines(errChan chan error) {
	watcher := watch.New(store.DefalutClient.Client, "")
	nodeonlinewatchChan, err := watcher.WatchList(n.ctx, option.Config.OnlineNodePath, "")
	if err != nil {
		errChan <- err
	}
	defer nodeonlinewatchChan.Stop()
	for event := range nodeonlinewatchChan.ResultChan() {
		switch event.Type {
		case watch.Added, watch.Modified:
			nodeID := n.getNodeIDFromKey(event.GetKey())
			if node := n.GetNode(nodeID); node != nil {
				if !node.Alived {
					node.Alived = true
					node.UpTime = time.Now()
				}
			} else {
				n.lock.Lock()
				n.nodeonline[nodeID] = "online"
				n.lock.Unlock()
			}
		case watch.Deleted:
			nodeID := n.getNodeIDFromKey(event.GetKey())
			if node := n.GetNode(nodeID); node != nil {
				if node.Alived {
					node.Alived = false
					node.UpTime = time.Now()
					n.UpdateNode(node)
				}
			} else {
				n.lock.Lock()
				n.nodeonline[nodeID] = "offline"
				n.lock.Unlock()
			}
		case watch.Error:
			errChan <- event.Error
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
			RegToHost(node, "add")
		case watch.Deleted:
			node := new(client.HostNode)
			if err := node.Decode(ev.GetPreValue()); err != nil {
				logrus.Errorf("decode node info error :%s", err)
				continue
			}
			n.RemoveNode(node.ID)
			RegToHost(node, "del")
		case watch.Error:
			errChan <- ev.Error
		}
	}
}

//InstallNode 安装节点
func (n *Cluster) InstallNode() {

}

//CheckNodeInstall 简称节点是否安装 rainbond node
//如果未安装，尝试安装
func (n *Cluster) CheckNodeInstall(node *client.HostNode) {
	n.checkInstall <- node
}
func (n *Cluster) checkNodeInstall(node *client.HostNode) {
	initCondition := client.NodeCondition{
		Type: client.NodeInit,
	}
	defer func() {
		node.UpdataCondition(initCondition)
		n.UpdateNode(node)
	}()
	node.Status = "init"
	node.NodeStatus.Status = "init"
	errorCondition := func(reason string, err error) {
		initCondition.Status = client.ConditionFalse
		initCondition.LastTransitionTime = time.Now()
		initCondition.LastHeartbeatTime = time.Now()
		initCondition.Reason = reason
		if err != nil {
			initCondition.Message = err.Error()
		}
		node.NodeStatus.Conditions = append(node.NodeStatus.Conditions, initCondition)
		node.Status = "init_failed"
		node.NodeStatus.Status = "init_failed"
	}
	if node.Role == nil {
		node.Role = []string{"compute"}
	}
	role := strings.Join(node.Role, ",")
	etcdConfig := n.datacenterConfig.GetConfig("ETCD_ADDRS")
	etcd := n.currentNode.InternalIP
	if etcdConfig != nil && etcdConfig.Value != nil {
		logrus.Infof("etcd address is %v when install node", etcdConfig.Value)
		switch etcdConfig.Value.(type) {
		case string:
			if etcdConfig.Value.(string) != "" {
				etcd = etcdConfig.Value.(string)
			}
		case []string:
			etcd = strings.Join(etcdConfig.Value.([]string), ",")
		}
	}
	initshell := "repo.goodrain.com/release/3.5/gaops/jobs/install/prepare/init.sh"
	etcd = etcd + ","
	cmd := fmt.Sprintf("bash -c \"set %s %s %s;$(curl -s %s)\"", node.ID, etcd, role, initshell)
	logrus.Infof("init endpoint node cmd is %s", cmd)

	//日志输出文件
	if err := util.CheckAndCreateDir("/var/log/event"); err != nil {
		logrus.Errorf("check and create dir /var/log/event error,%s", err.Error())
	}
	logFile := "/var/log/event/install_node_" + node.ID + ".log"
	logfile, err := util.OpenOrCreateFile(logFile)
	if err != nil {
		logrus.Errorf("check and create install node logfile error,%s", err.Error())
	}
	if logfile == nil {
		logfile = os.Stdout
	}
	//结果输出buffer
	var stderr bytes.Buffer
	client := util.NewSSHClient(node.InternalIP, "root", node.RootPass, cmd, 22, logfile, &stderr)
	if err := client.Connection(); err != nil {
		logrus.Error("init endpoint node error:", err.Error())
		errorCondition("SSH登陆初始化目标节点失败", err)
		return
	}
	//设置init的结果
	result := stderr.String()
	index := strings.Index(result, "{")
	jsonOutPut := result
	if index > -1 {
		jsonOutPut = result[index:]
	}
	fmt.Println("Init node Result:" + jsonOutPut)
	output, err := model.ParseTaskOutPut(jsonOutPut)
	if err != nil {
		errorCondition("节点初始化输出数据错误", err)
		logrus.Errorf("get init current node result error:%s", err.Error())
	}
	node.Status = "init_success"
	node.NodeStatus.Status = "init_success"
	if output.Global != nil {
		for k, v := range output.Global {
			if strings.Index(v, ",") > -1 {
				values := strings.Split(v, ",")
				util.Deweight(&values)
				n.datacenterConfig.PutConfig(&model.ConfigUnit{
					Name:           strings.ToUpper(k),
					Value:          values,
					ValueType:      "array",
					IsConfigurable: false,
				})
			} else {
				n.datacenterConfig.PutConfig(&model.ConfigUnit{
					Name:           strings.ToUpper(k),
					Value:          v,
					ValueType:      "string",
					IsConfigurable: false,
				})
			}
		}
	}
	node.Update()
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
	if status, ok := n.nodeonline[node.ID]; ok {
		if status == "online" {
			node.Alived = true
		} else {
			node.Alived = false
		}
		delete(n.nodeonline, node.ID)
	}
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

//UpdateNodeCondition 更新节点状态
func (n *Cluster) UpdateNodeCondition(nodeID, ctype, cvalue string) {
	node := n.GetNode(nodeID)
	if node == nil {
		return
	}
	node.UpdataCondition(client.NodeCondition{
		Type:               client.NodeConditionType(ctype),
		Status:             client.ConditionStatus(cvalue),
		LastHeartbeatTime:  time.Now(),
		LastTransitionTime: time.Now(),
		Message:            "",
		Reason:             "",
	})
	n.UpdateNode(node)
}

//GetLabelsNode 返回匹配labels的节点ID
func (n *Cluster) GetLabelsNode(labels map[string]string) []string {
	var nodes []string
	for _, node := range n.nodes {
		if checkLables(node, labels) {
			nodes = append(nodes, node.ID)
		}
	}
	return nodes
}

func checkLables(node *client.HostNode, labels map[string]string) bool {
	for k, v := range labels {
		if nodev := node.Labels[k]; nodev != v {
			return false
		}
	}
	return true
}
