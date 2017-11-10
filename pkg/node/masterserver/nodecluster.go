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

package masterserver

import (
	"context"
	"fmt"
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
	"github.com/goodrain/rainbond/pkg/node/core/store"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

//NodeCluster 节点管理器
type NodeCluster struct {
	ctx       context.Context
	cancel    context.CancelFunc
	nodes     map[string]*model.HostNode
	lock      sync.Mutex
	client    *store.Client
	k8sClient *kubernetes.Clientset
}

//CreateNodeCluster 创建节点管理器
func CreateNodeCluster(k8sClient *kubernetes.Clientset) (*NodeCluster, error) {
	ctx, cancel := context.WithCancel(context.Background())
	nc := NodeCluster{
		ctx:       ctx,
		cancel:    cancel,
		nodes:     make(map[string]*model.HostNode, 5),
		client:    store.DefalutClient,
		k8sClient: k8sClient,
	}
	if err := nc.loadNodes(); err != nil {
		return nil, err
	}
	return &nc, nil
}

//Start 启动
func (n *NodeCluster) Start() {
	go n.watchNodes()
	go n.watchK8sNodes()
}

//Stop 停止
func (n *NodeCluster) Stop(i interface{}) {
	n.cancel()
}
func (n *NodeCluster) loadNodes() error {
	//加载节点信息
	res, err := n.client.Get(option.Config.NodePath, client.WithPrefix())
	if err != nil {
		return fmt.Errorf("load cluster nodes error:%s", err.Error())
	}
	for _, kv := range res.Kvs {
		if node := n.getNodeFromKV(kv); node != nil {
			n.AddNode(node)
		}
	}
	//加载rainbond节点在线信息
	res, err = n.client.Get(option.Config.OnlineNodePath, client.WithPrefix())
	if err != nil {
		return fmt.Errorf("load cluster nodes error:%s", err.Error())
	}
	for _, kv := range res.Kvs {
		if node := n.getNodeFromKey(string(kv.Key)); node != nil {
			if !node.Alived {
				node.Alived = true
				node.UpTime = time.Now()
			}
		}
	}
	//加载k8s节点信息
	list, err := n.k8sClient.Core().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("load k8s nodes from k8s api error:%s", err.Error())
	}
	for _, node := range list.Items {
		if cn, ok := n.nodes[node.Name]; ok {
			cn.NodeStatus = &node.Status
			cn.UpdataK8sCondition(node.Status.Conditions)
			n.UpdateNode(cn)
		} else {
			logrus.Warning("k8s node %s can not exist in rainbond cluster.", node.Name)
		}
	}
	return nil
}

//UpdateNode 更新节点信息
func (n *NodeCluster) UpdateNode(node *model.HostNode) {
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
	if node, ok := n.nodes[id]; ok {
		return node
	}
	return nil
}
func (n *NodeCluster) watchNodes() {
	ch := n.client.Watch(option.Config.NodePath, client.WithPrefix())
	onlineCh := n.client.Watch(option.Config.OnlineNodePath, client.WithPrefix())
	for {
		select {
		case <-n.ctx.Done():
			return
		case event := <-ch:
			for _, ev := range event.Events {
				switch {
				case ev.IsCreate(), ev.IsModify():
					if node := n.getNodeFromKV(ev.Kv); node != nil {
						n.AddNode(node)
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
						n.UpdateNode(node)
					}
				case ev.Type == client.EventTypeDelete:
					if node := n.getNodeFromKey(string(ev.Kv.Key)); node != nil {
						node.Alived = false
						node.DownTime = time.Now()
						n.UpdateNode(node)
					}
				}
			}
		}
	}
}

func (n *NodeCluster) watchK8sNodes() {
	for {
		wc, err := n.k8sClient.Core().Nodes().Watch(metav1.ListOptions{})
		if err != nil {
			logrus.Error("watch k8s node error.", err.Error())
			time.Sleep(time.Second * 5)
		}
		defer wc.Stop()
		for {
			select {
			case event := <-wc.ResultChan():
				switch {
				case event.Type == watch.Added, event.Type == watch.Modified:
					if node, ok := event.Object.(*v1.Node); ok {
						if rbnode := n.GetNode(node.Name); rbnode != nil {
							rbnode.NodeStatus = &node.Status
							rbnode.UpdataK8sCondition(node.Status.Conditions)
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

//GetAllNode 获取全部节点
func (n *NodeCluster) GetAllNode() (nodes []*model.HostNode) {
	n.lock.Lock()
	defer n.lock.Unlock()
	for _, node := range n.nodes {
		nodes = append(nodes, node)
	}
	return
}

//AddNode 添加节点到缓存
func (n *NodeCluster) AddNode(node *model.HostNode) {
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
