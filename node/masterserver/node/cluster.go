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

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"

	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/api/model"
	"github.com/goodrain/rainbond/node/core/config"
	"github.com/goodrain/rainbond/node/core/store"
	"github.com/goodrain/rainbond/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8swatch "k8s.io/apimachinery/pkg/watch"
)

//Cluster  node  controller
type Cluster struct {
	ctx              context.Context
	cancel           context.CancelFunc
	nodes            map[string]*model.HostNode
	nodeonline       map[string]string
	lock             sync.Mutex
	client           *store.Client
	k8sClient        *kubernetes.Clientset
	currentNode      *model.HostNode
	checkInstall     chan *model.HostNode
	datacenterConfig *config.DataCenterConfig
}

//CreateCluster create node controller
func CreateCluster(k8sClient *kubernetes.Clientset, node *model.HostNode, datacenterConfig *config.DataCenterConfig) *Cluster {
	ctx, cancel := context.WithCancel(context.Background())
	nc := Cluster{
		ctx:              ctx,
		cancel:           cancel,
		nodes:            make(map[string]*model.HostNode, 5),
		nodeonline:       make(map[string]string, 10),
		client:           store.DefalutClient,
		k8sClient:        k8sClient,
		currentNode:      node,
		checkInstall:     make(chan *model.HostNode, 4),
		datacenterConfig: datacenterConfig,
	}
	return &nc
}

//Start 启动
func (n *Cluster) Start(errchan chan error) error {
	go n.loadAndWatchNodes(errchan)
	go n.loadAndWatchNodeOnlines(errchan)
	go n.loadAndWatchK8sNodes()
	go n.worker()
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

//RegToHost regist node id to hosts file
func RegToHost(node *model.HostNode, opt string) {

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
func (n *Cluster) UpdateNode(node *model.HostNode) {
	n.lock.Lock()
	defer n.lock.Unlock()
	n.nodes[node.ID] = node
	n.client.Put(option.Config.NodePath+"/"+node.ID, node.String())
}
func (n *Cluster) getNodeFromKV(kv *mvccpb.KeyValue) *model.HostNode {
	var node model.HostNode
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

//GetNode 从缓存获取节点信息
func (n *Cluster) GetNode(id string) *model.HostNode {
	n.lock.Lock()
	defer n.lock.Unlock()
	if node, ok := n.nodes[id]; ok {
		n.handleNodeStatus(node)
		return node
	}
	return nil
}
func (n *Cluster) handleNodeStatus(v *model.HostNode) {
	if v.Role.HasRule("compute") {
		if v.NodeStatus != nil {
			if v.Unschedulable {
				v.Status = "unschedulable"
				return
			}
			if v.AvailableCPU == 0 {
				v.AvailableCPU = v.NodeStatus.Allocatable.Cpu().Value()
			}
			if v.AvailableMemory == 0 {
				v.AvailableMemory = v.NodeStatus.Allocatable.Memory().Value()
			}
			var haveready bool
			for _, condiction := range v.Conditions {
				if condiction.Status == "True" && (condiction.Type == "OutOfDisk" || condiction.Type == "MemoryPressure" || condiction.Type == "DiskPressure") {
					v.Status = "error"
					return
				}
				if v.Status == "unschedulable" || v.Status == "init" || v.Status == "init_success" || v.Status == "init_failed" || v.Status == "installing" || v.Status == "install_success" || v.Status == "install_failed" {

				}
				if condiction.Type == "Ready" {
					haveready = true
					if condiction.Status == "True" {
						v.Status = "running"
					} else {
						v.Status = "notready"
					}
				}
			}
			if !haveready {
				v.Status = "notready"
			}
		} else {
			v.Status = "down"
		}
	}
	if v.Role.HasRule("manage") && !v.Role.HasRule("compute") { //manage install_success == runnint
		if v.Status == "init" || v.Status == "init_success" || v.Status == "init_failed" || v.Status == "installing" || v.Status == "install_failed" {
			return
		}
		if v.Alived {
			for _, condition := range v.Conditions {
				if condition.Type == "NodeInit" && condition.Status == "True" {
					v.Status = "running"
				}
			}
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
					n.UpdateNode(node)
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
			node := new(model.HostNode)
			if err := node.Decode(ev.GetValue()); err != nil {
				logrus.Errorf("decode node info error :%s", err)
				continue
			}
			n.CacheNode(node)
			RegToHost(node, "add")
		case watch.Deleted:
			node := new(model.HostNode)
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

func (n *Cluster) loadAndWatchK8sNodes() {
	for {
		list, err := n.k8sClient.Core().Nodes().List(metav1.ListOptions{})
		if err != nil {
			logrus.Warnf("load k8s nodes from k8s api error:%s", err.Error())
			time.Sleep(time.Second * 3)
			continue
		}
		for _, node := range list.Items {
			if cn, ok := n.nodes[node.Name]; ok {
				cn.NodeStatus = &node.Status
				cn.NodeStatus.Images = nil
				cn.Unschedulable = node.Spec.Unschedulable
				cn.UpdataK8sCondition(node.Status.Conditions)
				n.UpdateNode(cn)
			} else {
				logrus.Warningf("k8s node %s can not exist in rainbond cluster.", node.Name)
			}
		}
		break
	}
	for {
		wc, err := n.k8sClient.Core().Nodes().Watch(metav1.ListOptions{})
		if err != nil {
			logrus.Warningf("watch k8s node error.", err.Error())
			time.Sleep(time.Second * 5)
			continue
		}
		defer func() {
			if wc != nil {
				wc.Stop()
			}
		}()
	loop:
		for {
			select {
			case event, ok := <-wc.ResultChan():
				if !ok {
					time.Sleep(time.Second * 3)
					break loop
				}
				switch {
				case event.Type == k8swatch.Added, event.Type == k8swatch.Modified:
					if node, ok := event.Object.(*v1.Node); ok {
						//k8s node name is rainbond node id
						if rbnode := n.GetNode(node.Name); rbnode != nil {
							rbnode.NodeStatus = &node.Status
							rbnode.NodeStatus.Images = nil
							rbnode.UpdataK8sCondition(node.Status.Conditions)
							if rbnode.AvailableCPU == 0 {
								rbnode.AvailableCPU, _ = node.Status.Allocatable.Cpu().AsInt64()
							}
							if rbnode.AvailableMemory == 0 {
								rbnode.AvailableMemory, _ = node.Status.Allocatable.Memory().AsInt64()
							}
							rbnode.Unschedulable = node.Spec.Unschedulable
							n.UpdateNode(rbnode)
						}
					}
				case event.Type == k8swatch.Deleted:
					if node, ok := event.Object.(*v1.Node); ok {
						if rbnode := n.GetNode(node.Name); rbnode != nil {
							rbnode.NodeStatus = nil
							rbnode.DeleteCondition(model.NodeReady, model.OutOfDisk, model.MemoryPressure, model.DiskPressure)
							n.UpdateNode(rbnode)
						}
					}
				default:
					logrus.Warning("don't know the kube api watch event type when watch node.")
				}
			case <-n.ctx.Done():
				return
			}
		}
	}
}

//InstallNode 安装节点
func (n *Cluster) InstallNode() {

}

//CheckNodeInstall 简称节点是否安装 rainbond node
//如果未安装，尝试安装
func (n *Cluster) CheckNodeInstall(node *model.HostNode) {
	n.checkInstall <- node
}
func (n *Cluster) checkNodeInstall(node *model.HostNode) {
	initCondition := model.NodeCondition{
		Type: model.NodeInit,
	}
	defer func() {
		node.UpdataCondition(initCondition)
		n.UpdateNode(node)
	}()
	node.Status = "init"
	errorCondition := func(reason string, err error) {
		initCondition.Status = model.ConditionFalse
		initCondition.LastTransitionTime = time.Now()
		initCondition.LastHeartbeatTime = time.Now()
		initCondition.Reason = reason
		if err != nil {
			initCondition.Message = err.Error()
		}
		node.Conditions = append(node.Conditions, initCondition)
		node.Status = "init_failed"
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
func (n *Cluster) GetAllNode() (nodes []*model.HostNode) {
	n.lock.Lock()
	defer n.lock.Unlock()
	for _, v := range n.nodes {
		n.handleNodeStatus(v)
		nodes = append(nodes, v)
	}
	return
}

//CacheNode 添加节点到缓存
func (n *Cluster) CacheNode(node *model.HostNode) {
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
	node.UpdataCondition(model.NodeCondition{
		Type:               model.NodeConditionType(ctype),
		Status:             model.ConditionStatus(cvalue),
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

func checkLables(node *model.HostNode, labels map[string]string) bool {
	for k, v := range labels {
		if nodev := node.Labels[k]; nodev != v {
			return false
		}
	}
	return true
}
