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

	"k8s.io/client-go/kubernetes"

	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/core/config"
	"github.com/goodrain/rainbond/pkg/node/core/store"
)

//MasterServer 主节点服务
type MasterServer struct {
	*store.Client
	*model.HostNode
	Cluster          *NodeCluster
	TaskEngine       *TaskEngine
	ctx              context.Context
	cancel           context.CancelFunc
	datacenterConfig *config.DataCenterConfig
}

//NewMasterServer 创建master节点
func NewMasterServer(node *model.HostNode, k8sClient *kubernetes.Clientset) (*MasterServer, error) {
	datacenterConfig := config.CreateDataCenterConfig()
	ctx, cancel := context.WithCancel(context.Background())
	cluster, err := CreateNodeCluster(k8sClient)
	if err != nil {
		cancel()
		return nil, err
	}
	ms := &MasterServer{
		Client:           store.DefalutClient,
		TaskEngine:       CreateTaskEngine(cluster, node),
		HostNode:         node,
		Cluster:          cluster,
		ctx:              ctx,
		cancel:           cancel,
		datacenterConfig: datacenterConfig,
	}
	return ms, nil
}

//Start 启动
func (m *MasterServer) Start() error {
	m.Cluster.Start()
	m.TaskEngine.Start()
	//监控配置变化启动
	m.datacenterConfig.Start()
	return nil
}

//Stop 停止
func (m *MasterServer) Stop(i interface{}) {
	m.Cluster.Stop(i)
	m.TaskEngine.Stop()
	m.cancel()
}
