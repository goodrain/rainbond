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
	"github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/node/core/config"
	"github.com/goodrain/rainbond/node/core/job"
)

//ClusterClient ClusterClient
type ClusterClient interface {
	UpdateStatus(*HostNode) error
	GetMasters() ([]*HostNode, error)
	GetNode(nodeID string) (*HostNode, error)
	GetDataCenterConfig() (*config.DataCenterConfig, error)
	WatchJobs() <-chan *job.Event

	//WatchTasks()
	//UpdateTask()
}

//NewClusterClient new cluster client
func NewClusterClient(etcdClient *clientv3.Client) ClusterClient {
	return &etcdClusterClient{
		etcdClient: etcdClient,
	}
}

type etcdClusterClient struct {
	etcdClient *clientv3.Client
}

func (e *etcdClusterClient) UpdateStatus(*HostNode) error {
	return nil
}

func (e *etcdClusterClient) GetMasters() ([]*HostNode, error) {
	return nil, nil
}

func (e *etcdClusterClient) GetDataCenterConfig() (*config.DataCenterConfig, error) {
	return nil, nil
}

func (e *etcdClusterClient) WatchJobs() <-chan *job.Event {
	return nil
}

func (e *etcdClusterClient) GetNode(nodeID string) (*HostNode, error) {
	return nil, nil
}
