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

package controller

import (
	"net/http"

	"github.com/goodrain/rainbond/entrance/core"
	"github.com/goodrain/rainbond/entrance/store"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/client"
	restful "github.com/emicklei/go-restful"
	"github.com/twinj/uuid"

	"github.com/goodrain/rainbond/entrance/api/model"
	apistore "github.com/goodrain/rainbond/entrance/api/store"

	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//NodeSource 集群节点管理接口
type NodeSource struct {
	coreManager     core.Manager
	readStore       store.ReadStore
	apiStoreManager *apistore.Manager
	clientSet       *kubernetes.Clientset
}

func returns201(b *restful.RouteBuilder) {
	b.Returns(http.StatusCreated, "", model.HostNode{})
}

//Register 注册
func (u NodeSource) Register(container *restful.Container) {
	ws := new(restful.WebService)
	ws.Path("/nodes").
		Doc("Manage cluster computer nodes").
		Consumes(restful.MIME_XML, restful.MIME_JSON).
		Produces(restful.MIME_JSON, restful.MIME_XML) // you can specify this per route as well

	ws.Route(ws.POST("").To(u.addNode).
		// docs
		Doc("create a host node").
		Operation("createNode").
		Reads(model.HostNode{}).Do(returns201)) // from the request

	ws.Route(ws.GET("").To(u.getNodes).
		// docs
		Doc("get all host node").
		Operation("getNodes").
		Writes(ResponseType{})) // from the request

	ws.Route(ws.GET("{host_name}").To(u.getNodeInfo).
		// docs
		Doc("get all host node").
		Operation("getNodes").
		Param(ws.PathParameter("host_name", "the name of the host").DataType("string")).
		Writes(ResponseType{})) // from the request

	ws.Route(ws.DELETE("/{host_name}").To(u.removeNode).
		// docs
		Doc("delete a host node").
		Operation("removeDomain").
		Param(ws.PathParameter("host_name", "the name of the host").DataType("string")))

	ws.Route(ws.PUT("/{host_name}").To(u.updateNode).
		// docs
		Doc("update a host node").
		Operation("updateNode").
		Param(ws.PathParameter("host_name", "the name of the host").DataType("string")).
		Reads(model.HostNode{})) // from the request

	container.Add(ws)
}

//addNode 添加节点
func (u *NodeSource) addNode(request *restful.Request, response *restful.Response) {
	node := new(model.HostNode)
	err := request.ReadEntity(node)
	if err != nil {
		NewFaliResponse(http.StatusBadRequest, "request body error."+err.Error(), "读取请求数据错误，数据不合法", response)
		return
	}
	if node.UUID == "" {
		node.UUID = uuid.NewV4().String()
	}
	if node.HostName == "" {
		NewFaliResponse(http.StatusBadRequest, "node host name cannot be empty", "节点主机名不能为空", response)
		return
	}
	if node.InternalIP == "" {
		NewFaliResponse(http.StatusBadRequest, "node host ip cannot be empty", "节点内网ip不能为空", response)
		return
	}
	if node.ExternalIP == "" {
		NewFaliResponse(http.StatusBadRequest, "node host ip cannot be empty", "节点外网ip不能为空", response)
		return
	}
	if node.Role == "" {
		node.Role = "tree"
	}
	if node.Status == "" {
		node.Status = "create"
	}
	var create bool
	var status v1.NodeStatus
	// if node already init
	if node.Status == "init" && node.Role == "tree" {
		logrus.Info("create node to kubernetes")
		k8sNode, err := u.clientSet.Core().Nodes().Get(node.HostName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				newk8sNode, err := createK8sNode(node)
				if err != nil {
					NewFaliResponse(400, err.Error(), "创建集群节点失败", response)
					return
				}
				v1Node, err := u.clientSet.Core().Nodes().Create(newk8sNode)
				if err != nil {
					if !apierrors.IsAlreadyExists(err) {
						node.Status = "running"
					}
					NewFaliResponse(400, err.Error(), "创建集群节点失败", response)
					return
				}
				status = v1Node.Status
				create = true
			}
		} else {
			status = k8sNode.Status
		}
		node.Status = "running"

	}
	err = u.apiStoreManager.AddSource("/store/nodes/"+node.HostName, node)
	if err != nil {
		if create {
			if err := u.clientSet.Core().Nodes().Delete(node.HostName, nil); err != nil {
				logrus.Errorf("Unable to register node %q to etcd: error deleting old node: %v", node.HostName, err)
			} else {
				logrus.Errorf("Deleted old node object %q", node.HostName)
			}
		}
		if cerr, ok := err.(client.Error); ok {
			if cerr.Code == client.ErrorCodeNodeExist {
				NewFaliResponse(400, "node is exist.", "节点已存在", response)
				return
			}
		}
		NewFaliResponse(500, err.Error(), "存储节点信息失败", response)
		return
	}
	node.NodeStatus = &status
	NewPostSuccessResponse(node, nil, response)
}

//removeNode
func (u *NodeSource) removeNode(request *restful.Request, response *restful.Response) {
	hostName := request.PathParameter("host_name")
	if hostName == "" {
		NewFaliResponse(400, "host name can not be empty", "节点的主机名不能为空", response)
		return
	}
	node := model.HostNode{}
	err := u.apiStoreManager.GetSource("/store/nodes/"+hostName, &node)
	if err != nil {
		if client.IsKeyNotFound(err) {
			NewFaliResponse(404, "host name not found", "节点的主机名不存在", response)
			return
		}
		NewFaliResponse(500, err.Error(), "获取节点数据错误", response)
		return
	}
	if err := u.clientSet.Core().Nodes().Delete(hostName, nil); err != nil {
		if !apierrors.IsNotFound(err) {
			logrus.Errorf("Unable to register node %q to etcd: error deleting old node: %v", hostName, err)
			NewFaliResponse(500, err.Error(), "删除集群节点错误", response)
			return
		}
	}
	err = u.apiStoreManager.DeleteSource("/store/nodes/"+hostName, false)
	if err != nil {
		if client.IsKeyNotFound(err) {
			NewFaliResponse(404, "host name not found", "节点的主机名不存在", response)
			return
		}
		NewFaliResponse(500, err.Error(), "删除节点数据错误", response)
		return
	}
	NewSuccessResponse(node, nil, response)
}

//updateNode
func (u *NodeSource) updateNode(request *restful.Request, response *restful.Response) {
	node := new(model.HostNode)
	err := request.ReadEntity(node)
	if err != nil {
		NewFaliResponse(http.StatusBadRequest, "request body error."+err.Error(), "读取请求数据错误，数据不合法", response)
		return
	}
	if node.UUID == "" {
		NewFaliResponse(http.StatusBadRequest, "node uuid cannot be empty", "节点UUID不能为空", response)
		return
	}
	if node.HostName == "" {
		NewFaliResponse(http.StatusBadRequest, "node host name cannot be empty", "节点主机名不能为空", response)
		return
	}
	if node.InternalIP == "" {
		NewFaliResponse(http.StatusBadRequest, "node host ip cannot be empty", "节点内网ip不能为空", response)
		return
	}
	if node.ExternalIP == "" {
		NewFaliResponse(http.StatusBadRequest, "node host ip cannot be empty", "节点外网ip不能为空", response)
		return
	}
	var create bool
	var status v1.NodeStatus
	// if node already init
	if node.Status == "init" && node.Role == "tree" {
		k8sNode, err := createK8sNode(node)
		if err != nil {
			NewFaliResponse(400, err.Error(), "创建集群节点失败", response)
			return
		}
		v1Node, err := u.clientSet.Core().Nodes().Create(k8sNode)
		if err != nil {
			if !apierrors.IsAlreadyExists(err) {
				NewFaliResponse(400, err.Error(), "集群节点已存在", response)
				return
			}
			NewFaliResponse(400, err.Error(), "创建集群节点失败", response)
			return
		}
		status = v1Node.Status
		node.Status = "running"
		create = true
	}
	err = u.apiStoreManager.UpdateSource("/store/nodes/"+node.HostName, node)
	if err != nil {
		if create {
			if err := u.clientSet.Core().Nodes().Delete(node.HostName, nil); err != nil {
				logrus.Errorf("Unable to register node %q to etcd: error deleting old node: %v", node.HostName, err)
			} else {
				logrus.Errorf("Deleted old node object %q", node.HostName)
			}
		}
		if cerr, ok := err.(client.Error); ok {
			if cerr.Code == client.ErrorCodeNodeExist {
				NewFaliResponse(400, "node is exist.", "节点已存在", response)
				return
			}
		}
		NewFaliResponse(500, err.Error(), "存储节点信息失败", response)
		return
	}
	node.NodeStatus = &status
	NewSuccessResponse(node, nil, response)
}

func (u *NodeSource) getNodes(request *restful.Request, response *restful.Response) {
	list, err := u.apiStoreManager.GetSourceList("/store/nodes", "host_node")
	if err != nil {
		if client.IsKeyNotFound(err) {
			NewFaliResponse(404, "not found nodes", "节点不存在", response)
			return
		}
		NewFaliResponse(500, err.Error(), "查询节点错误", response)
		return
	}
	NewSuccessResponse(nil, list, response)
}

func (u *NodeSource) getNodeInfo(request *restful.Request, response *restful.Response) {
	hostName := request.PathParameter("host_name")
	if hostName == "" {
		NewFaliResponse(400, "host name can not be empty", "节点的主机名不能为空", response)
		return
	}
	node := model.HostNode{}
	err := u.apiStoreManager.GetSource("/store/nodes/"+hostName, &node)
	if err != nil {
		if client.IsKeyNotFound(err) {
			NewFaliResponse(404, "host name not found", "节点的主机名不存在", response)
			return
		}
		NewFaliResponse(500, err.Error(), "获取节点数据错误", response)
		return
	}
	k8sNode, err := u.clientSet.Core().Nodes().Get(hostName, metav1.GetOptions{})
	if err == nil {
		node.NodeStatus = &k8sNode.Status
		NewSuccessResponse(node, nil, response)
	} else {
		NewSuccessMessageResponse(node, nil, err.Error(), "获取节点运行状态错误", response)
	}
}

func createK8sNode(node *model.HostNode) (*v1.Node, error) {
	cpu, err := resource.ParseQuantity(fmt.Sprintf("%dm", node.AvailableCPU*1000))
	if err != nil {
		return nil, err
	}
	mem, err := resource.ParseQuantity(fmt.Sprintf("%dKi", node.AvailableMemory*1024*1024))
	if err != nil {
		return nil, err
	}
	nameAddress := v1.NodeAddress{
		Type:    v1.NodeHostName,
		Address: node.HostName,
	}
	internalIP := v1.NodeAddress{
		Type:    v1.NodeInternalIP,
		Address: node.InternalIP,
	}
	externalIP := v1.NodeAddress{
		Type:    v1.NodeExternalIP,
		Address: node.ExternalIP,
	}
	k8sNode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   node.HostName,
			Labels: node.Labels,
		},
		Spec: v1.NodeSpec{
			Unschedulable: node.Unschedulable,
		},
		Status: v1.NodeStatus{
			Allocatable: v1.ResourceList{v1.ResourceCPU: cpu, v1.ResourceMemory: mem},
			Addresses:   []v1.NodeAddress{nameAddress, internalIP, externalIP},
		},
	}
	return k8sNode, nil
}
