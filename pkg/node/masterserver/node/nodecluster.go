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

package node

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/pquerna/ffjson/ffjson"

	"github.com/coreos/etcd/mvcc/mvccpb"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"

	client "github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/core/config"
	"github.com/goodrain/rainbond/pkg/node/core/store"
	"github.com/goodrain/rainbond/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

//NodeCluster 节点管理器
type NodeCluster struct {
	ctx              context.Context
	cancel           context.CancelFunc
	nodes            map[string]*model.HostNode
	lock             sync.Mutex
	client           *store.Client
	k8sClient        *kubernetes.Clientset
	currentNode      *model.HostNode
	checkInstall     chan *model.HostNode
	datacenterConfig *config.DataCenterConfig
}

//CreateNodeCluster 创建节点管理器
func CreateNodeCluster(k8sClient *kubernetes.Clientset, node *model.HostNode, datacenterConfig *config.DataCenterConfig) *NodeCluster {
	ctx, cancel := context.WithCancel(context.Background())
	nc := NodeCluster{
		ctx:              ctx,
		cancel:           cancel,
		nodes:            make(map[string]*model.HostNode, 5),
		client:           store.DefalutClient,
		k8sClient:        k8sClient,
		currentNode:      node,
		checkInstall:     make(chan *model.HostNode, 4),
		datacenterConfig: datacenterConfig,
	}
	return &nc
}

//Start 启动
func (n *NodeCluster) Start() error {
	if err := n.loadAndWatchNodes(); err != nil {
		return err
	}
	go n.loadAndWatchK8sNodes()
	go n.worker()
	return nil
}

//Stop 停止
func (n *NodeCluster) Stop(i interface{}) {
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

//RegToHost 注册节点
func RegToHost(node *model.HostNode, opt string) {
	if !exists("/usr/share/gr-rainbond-node/gaops/jobs/cron/common/node_update_hosts.sh") {
		return
	}
	uuid := node.ID
	internalIP := node.InternalIP
	logrus.Infof("node 's hostname is %s", node.HostName)
	cmd := exec.Command("bash", "/usr/share/gr-rainbond-node/gaops/jobs/cron/common/node_update_hosts.sh", uuid, internalIP, opt)
	outbuf := bytes.NewBuffer(nil)
	cmd.Stdout = outbuf
	err := cmd.Run()
	if err != nil {
		logrus.Errorf("error update /etc/hosts,details %s", err.Error())
		return
	}
	logrus.Infof("update /etc/hosts %s %s success", uuid, internalIP)
}
func (n *NodeCluster) worker() {
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
func (n *NodeCluster) UpdateNode(node *model.HostNode) {
	n.lock.Lock()
	defer n.lock.Unlock()
	n.nodes[node.ID] = node
	n.client.Put(option.Config.NodePath+"/"+node.ID, node.String())
}
func (n *NodeCluster) getNodeFromKV(kv *mvccpb.KeyValue) *model.HostNode {
	var node model.HostNode
	if err := ffjson.Unmarshal(kv.Value, &node); err != nil {
		logrus.Error("parse node info error:", err.Error())
		return nil
	}
	return &node
}
func (n *NodeCluster) getNodeFromKey(key string) *model.HostNode {
	index := strings.LastIndex(key, "/")
	if index < 0 {
		return nil
	}
	id := key[index+1:]
	return n.GetNode(id)
}

//GetNode 从缓存获取节点信息
func (n *NodeCluster) GetNode(id string) *model.HostNode {
	n.lock.Lock()
	defer n.lock.Unlock()
	if node, ok := n.nodes[id]; ok {
		if node.NodeStatus != nil {
			if node.Unschedulable {
				node.Status = "unschedulable"
			} else {
				node.Status="running"
			}
			if node.AvailableCPU == 0 {
				node.AvailableCPU = node.NodeStatus.Allocatable.Cpu().Value()
			}
			if node.AvailableMemory == 0 {
				node.AvailableMemory = node.NodeStatus.Allocatable.Memory().Value()
			}
		}
		return node
	}
	return nil
}

func (n *NodeCluster) loadAndWatchNodes() error {
	//加载节点信息
	noderes, err := n.client.Get(option.Config.NodePath, client.WithPrefix())
	if err != nil {
		return fmt.Errorf("load cluster nodes error:%s", err.Error())
	}
	for _, kv := range noderes.Kvs {
		if node := n.getNodeFromKV(kv); node != nil {
			n.CacheNode(node)
		}
	}
	//加载rainbond节点在线信息
	onlineres, err := n.client.Get(option.Config.OnlineNodePath, client.WithPrefix())
	if err != nil {
		return fmt.Errorf("load cluster nodes error:%s", err.Error())
	}
	for _, kv := range onlineres.Kvs {
		if node := n.getNodeFromKey(string(kv.Key)); node != nil {
			if !node.Alived {
				node.Alived = true
				node.UpTime = time.Now()
				n.UpdateNode(node)
			}
		}
	}
	go func() {
		ch := n.client.Watch(option.Config.NodePath, client.WithPrefix(), client.WithRev(noderes.Header.Revision))
		onlineCh := n.client.Watch(option.Config.OnlineNodePath, client.WithPrefix(), client.WithRev(onlineres.Header.Revision))
		for {
			select {
			case <-n.ctx.Done():
				return
			case event := <-ch:
				for _, ev := range event.Events {
					switch {
					case ev.IsCreate(), ev.IsModify():
						if node := n.getNodeFromKV(ev.Kv); node != nil {
							n.CacheNode(node)
						}
					case ev.Type == client.EventTypeDelete:
						if node := n.getNodeFromKey(string(ev.Kv.Key)); node != nil {
							n.RemoveNode(node)
						}
					}
				}
			case event := <-onlineCh:
				for _, ev := range event.Events {
					switch {
					case ev.IsCreate(), ev.IsModify():
						if node := n.getNodeFromKey(string(ev.Kv.Key)); node != nil {
							node.Alived = true
							node.UpTime = time.Now()
							//getInfoForMaster(node)
							n.UpdateNode(node)
							RegToHost(node, "add")
						}
					case ev.Type == client.EventTypeDelete:
						if node := n.getNodeFromKey(string(ev.Kv.Key)); node != nil {
							node.Alived = false
							node.DownTime = time.Now()
							n.UpdateNode(node)
							RegToHost(node, "del")
						}
					}
				}
			}
		}
	}()
	return nil
}

func (n *NodeCluster) loadAndWatchK8sNodes() {
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
				case event.Type == watch.Added, event.Type == watch.Modified:
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
				case event.Type == watch.Deleted:
					if node, ok := event.Object.(*v1.Node); ok {
						if rbnode := n.GetNode(node.Name); rbnode != nil {
							rbnode.NodeStatus = nil
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
func (n *NodeCluster) InstallNode() {

}

//CheckNodeInstall 简称节点是否安装 rainbond node
//如果未安装，尝试安装
func (n *NodeCluster) CheckNodeInstall(node *model.HostNode) {
	n.checkInstall <- node
}
func (n *NodeCluster) checkNodeInstall(node *model.HostNode) {
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
func (n *NodeCluster) GetAllNode() (nodes []*model.HostNode) {
	n.lock.Lock()
	defer n.lock.Unlock()
	for _, node := range n.nodes {
		if node.NodeStatus != nil {
			if node.Unschedulable {
				node.Status = "unschedulable"
			} else {
				node.Status = "schedulable"
			}
			if node.AvailableCPU == 0 {
				node.AvailableCPU = node.NodeStatus.Allocatable.Cpu().Value()
			}
			if node.AvailableMemory == 0 {
				node.AvailableMemory = node.NodeStatus.Allocatable.Memory().Value()
			}
		}
		nodes = append(nodes, node)
	}
	return
}

//CacheNode 添加节点到缓存
func (n *NodeCluster) CacheNode(node *model.HostNode) {
	n.lock.Lock()
	defer n.lock.Unlock()
	logrus.Debugf("add or update a rainbon node id:%s hostname:%s ip:%s", node.ID, node.HostName, node.InternalIP)
	n.nodes[node.ID] = node
}

//RemoveNode 从缓存移除节点
func (n *NodeCluster) RemoveNode(node *model.HostNode) {
	n.lock.Lock()
	defer n.lock.Unlock()
	if _, ok := n.nodes[node.ID]; ok {
		delete(n.nodes, node.ID)
	}
}

//UpdateNodeCondition 更新节点状态
func (n *NodeCluster) UpdateNodeCondition(nodeID, ctype, cvalue string) {
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
func (n *NodeCluster) GetLabelsNode(labels map[string]string) []string {
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
