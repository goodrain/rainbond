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

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/cmd"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/core/config"
	"github.com/goodrain/rainbond/node/utils"
	"k8s.io/apimachinery/pkg/api/errors"
)

// RainbondEndpointPrefix is the prefix of the key of the rainbond endpoints in etcd
const RainbondEndpointPrefix = "/rainbond/endpoint"

//ClusterClient ClusterClient
type ClusterClient interface {
	UpdateStatus(*HostNode) error
	DownNode(*HostNode) error
	GetMasters() ([]*HostNode, error)
	GetNode(nodeID string) (*HostNode, error)
	RegistNode(node *HostNode) error
	GetDataCenterConfig() (*config.DataCenterConfig, error)
	GetOptions() *option.Conf
	GetEndpoints(key string) []string
	SetEndpoints(key string, value []string)
	DelEndpoints(key string)
}

//NewClusterClient new cluster client
func NewClusterClient(conf *option.Conf) ClusterClient {
	return &etcdClusterClient{
		conf: conf,
	}
}

type etcdClusterClient struct {
	conf      *option.Conf
	onlineLes clientv3.LeaseID
}

func (e *etcdClusterClient) UpdateStatus(n *HostNode) error {
	existNode, err := e.GetNode(n.ID)
	if err != nil {
		return fmt.Errorf("get node %s failure where update node %s", n.ID, err.Error())
	}
	//update node mode
	existNode.Mode = n.Mode
	//The startup parameters shall prevail
	existNode.Role = n.Role
	existNode.HostName = n.HostName
	existNode.NodeStatus.NodeHealth = n.NodeStatus.NodeHealth
	existNode.NodeStatus.NodeUpdateTime = time.Now()
	existNode.NodeStatus.Version = cmd.GetVersion()
	existNode.NodeStatus.AdviceAction = n.NodeStatus.AdviceAction
	existNode.NodeStatus.Status = n.NodeStatus.Status
	existNode.NodeStatus.NodeInfo = n.NodeStatus.NodeInfo
	existNode.AvailableMemory = n.AvailableMemory
	existNode.AvailableCPU = n.AvailableCPU
	// only update system labels
	newLabels := n.Labels
	for k, v := range existNode.Labels {
		if !strings.HasPrefix(k, "rainbond_node_rule_") {
			newLabels[k] = v
		}
	}
	existNode.Labels = newLabels
	existNode.UpdataCondition(n.NodeStatus.Conditions...)
	return e.Update(existNode)
}

func (e *etcdClusterClient) GetMasters() ([]*HostNode, error) {
	return nil, nil
}

func (e *etcdClusterClient) GetDataCenterConfig() (*config.DataCenterConfig, error) {
	return nil, nil
}

func (e *etcdClusterClient) GetOptions() *option.Conf {
	return e.conf
}

func (e *etcdClusterClient) GetEndpoints(key string) (result []string) {
	key = path.Join(RainbondEndpointPrefix, key)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	resp, err := e.conf.EtcdCli.Get(ctx, key, clientv3.WithPrefix())
	if err != nil || len(resp.Kvs) < 1 {
		logrus.Errorf("Can not get endpoints of the key %s", key)
		return
	}
	for _, kv := range resp.Kvs {
		var res []string
		err = json.Unmarshal(kv.Value, &res)
		if err != nil {
			logrus.Errorf("Can unmarshal endpoints to array of the key %s", key)
			return
		}
		result = append(result, res...)
	}
	logrus.Infof("Get endpoints %s => %v", key, result)
	return
}

func (e *etcdClusterClient) SetEndpoints(key string, value []string) {
	if key == "" || len(value) == 0 {
		return
	}

	// a key has wrong hostIP can't put into etcd
	if !utils.FilterEndpointKey(key) {
		logrus.Warnf("wrong key: %s, delete it", key)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		e.conf.EtcdCli.Delete(ctx, key)
		return
	}

	var endpoints []string
	for _, ep := range value {
		if !utils.FilterEndpoint(ep) {
			continue
		}
		endpoints = append(endpoints, ep)
	}

	key = path.Join(RainbondEndpointPrefix, key)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	jsonStr, err := json.Marshal(endpoints)
	if err != nil {
		logrus.Errorf("Can not marshal %s endpoints to json.", key)
		return
	}
	_, err = e.conf.EtcdCli.Put(ctx, key, string(jsonStr))
	if err != nil {
		logrus.Errorf("Failed to put endpoint for %s: %v", key, err)
	}
}

func (e *etcdClusterClient) DelEndpoints(key string) {
	key = path.Join(RainbondEndpointPrefix, key)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	_, err := e.conf.EtcdCli.Delete(ctx, key)
	if err != nil {
		logrus.Errorf("Failed to put endpoint for %s: %v", key, Error)
	}
	logrus.Infof("Delete endpoints: %s", key)
}

//ErrorNotFound node not found.
var ErrorNotFound = fmt.Errorf("node not found")

func (e *etcdClusterClient) GetNode(nodeID string) (*HostNode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	res, err := e.conf.EtcdCli.Get(ctx, fmt.Sprintf("%s/%s", e.conf.NodePath, nodeID))
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, ErrorNotFound
		}
		return nil, err
	}
	if res.Count < 1 {
		return nil, ErrorNotFound
	}
	return GetNodeFromKV(res.Kvs[0]), nil
}

func (e *etcdClusterClient) RegistNode(node *HostNode) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*8)
	defer cancel()
	_, err := e.conf.EtcdCli.Put(ctx, fmt.Sprintf("%s/%s", e.conf.NodePath, node.ID), node.String())
	if err != nil {
		return err
	}
	return nil
}

//Update update node info
func (e *etcdClusterClient) Update(h *HostNode) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	saveNode := *h
	saveNode.NodeStatus.KubeNode = nil
	_, err := e.conf.EtcdCli.Put(ctx, e.conf.NodePath+"/"+saveNode.ID, h.String())
	return err
}

//Down node set node status is offline
func (e *etcdClusterClient) DownNode(h *HostNode) error {
	existNode, err := e.GetNode(h.ID)
	if err != nil {
		return fmt.Errorf("get node %s failure where update node", h.ID)
	}
	existNode.NodeStatus.Status = "offline"
	existNode.NodeStatus.LastDownTime = time.Now()
	return e.Update(existNode)
}
