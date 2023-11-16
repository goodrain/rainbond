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
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/goodrain/rainbond/util"

	"github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/cmd"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/core/config"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
)

// RainbondEndpointPrefix is the prefix of the key of the rainbond endpoints in etcd
const RainbondEndpointPrefix = "/rainbond/endpoint"

// ClusterClient ClusterClient
type ClusterClient interface {
	UpdateStatus(*HostNode, []NodeConditionType) error
	DownNode(*HostNode) error
	GetMasters() ([]*HostNode, error)
	GetNode(nodeID string) (*HostNode, error)
	RegistNode(node *HostNode) error
	GetDataCenterConfig() (*config.DataCenterConfig, error)
	GetOptions() *option.Conf
	GetEndpoints(key string) []string
	SetEndpoints(serviceName, hostIP string, value []string)
	DelEndpoints(key string)
}

// NewClusterClient new cluster client
func NewClusterClient(conf *option.Conf) ClusterClient {
	return &etcdClusterClient{
		conf: conf,
	}
}

type etcdClusterClient struct {
	conf      *option.Conf
	onlineLes clientv3.LeaseID
}

func (e *etcdClusterClient) UpdateStatus(n *HostNode, deleteConditions []NodeConditionType) error {
	existNode, err := e.GetNode(n.ID)
	if err != nil {
		return fmt.Errorf("get node %s failure where update node %s", n.ID, err.Error())
	}
	//update node mode
	existNode.Mode = n.Mode
	//The startup parameters shall prevail
	existNode.Role = n.Role
	existNode.HostName = n.HostName
	existNode.Status = n.Status
	existNode.NodeStatus.NodeHealth = n.NodeStatus.NodeHealth
	existNode.NodeStatus.NodeUpdateTime = time.Now()
	existNode.NodeStatus.Version = cmd.GetVersion()
	existNode.NodeStatus.AdviceAction = n.NodeStatus.AdviceAction
	existNode.NodeStatus.Status = n.NodeStatus.Status
	existNode.NodeStatus.NodeInfo = n.NodeStatus.NodeInfo
	existNode.AvailableMemory = n.AvailableMemory
	existNode.AvailableCPU = n.AvailableCPU
	existNode.InternalIP = n.InternalIP
	// only update system labels
	newLabels := n.Labels
	for k, v := range existNode.Labels {
		if !strings.HasPrefix(k, "rainbond_node_rule_") {
			newLabels[k] = v
		}
	}
	existNode.Labels = newLabels
	//update condition and delete old condition
	existNode.UpdataCondition(n.NodeStatus.Conditions...)
	for _, t := range deleteConditions {
		existNode.DeleteCondition(t)
		logrus.Infof("remove old condition %s", t)
	}
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
		keyInfo := strings.Split(string(kv.Key), "/")
		if !util.CheckIP(keyInfo[len(keyInfo)-1]) {
			e.conf.EtcdCli.Delete(ctx, string(kv.Key))
			continue
		}
		var res []string
		err = json.Unmarshal(kv.Value, &res)
		if err != nil {
			logrus.Errorf("Can unmarshal endpoints to array of the key %s", key)
			return
		}
		//Return data check
		for _, v := range res {
			if checkURL(v) {
				result = append(result, v)
			}
		}
	}
	logrus.Debugf("Get endpoints %s => %v", key, result)
	return
}
func checkURL(source string) bool {
	endpointURL, err := url.Parse(source)
	if err != nil && strings.Contains(err.Error(), "first path segment in URL cannot contain colon") {
		endpointURL, err = url.Parse(fmt.Sprintf("tcp://%s", source))
	}
	if err != nil || endpointURL.Host == "" || endpointURL.Path != "" {
		return false
	}
	return true
}

// SetEndpoints service name and hostip must set
func (e *etcdClusterClient) SetEndpoints(serviceName, hostIP string, value []string) {
	if serviceName == "" {
		return
	}
	if !util.CheckIP(hostIP) {
		return
	}
	for _, v := range value {
		if !checkURL(v) {
			logrus.Warningf("%s service host %s endpoint value %s invalid", serviceName, hostIP, v)
			continue
		}
	}
	key := fmt.Sprintf("%s/%s/%s", RainbondEndpointPrefix, serviceName, hostIP)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	jsonStr, err := json.Marshal(value)
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

// ErrorNotFound node not found.
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

// Update update node info
func (e *etcdClusterClient) Update(h *HostNode) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	saveNode := *h
	saveNode.NodeStatus.KubeNode = nil
	_, err := e.conf.EtcdCli.Put(ctx, e.conf.NodePath+"/"+saveNode.ID, h.String())
	return err
}

// Down node set node status is offline
func (e *etcdClusterClient) DownNode(h *HostNode) error {
	existNode, err := e.GetNode(h.ID)
	if err != nil {
		return fmt.Errorf("get node %s failure where update node", h.ID)
	}
	existNode.NodeStatus.Status = "offline"
	existNode.NodeStatus.LastDownTime = time.Now()
	return e.Update(existNode)
}
