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

	"github.com/Sirupsen/logrus"

	"github.com/goodrain/rainbond/pkg/node/masterserver/task"

	"k8s.io/client-go/kubernetes"

	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/core/config"
	"github.com/goodrain/rainbond/pkg/node/core/store"
	"github.com/goodrain/rainbond/pkg/node/masterserver/node"
)

//MasterServer 主节点服务
type MasterServer struct {
	*store.Client
	*model.HostNode
	Cluster          *node.NodeCluster
	TaskEngine       *task.TaskEngine
	ctx              context.Context
	cancel           context.CancelFunc
	datacenterConfig *config.DataCenterConfig
}

//NewMasterServer 创建master节点
func NewMasterServer(modelnode *model.HostNode, k8sClient *kubernetes.Clientset) (*MasterServer, error) {
	datacenterConfig := config.GetDataCenterConfig()
	ctx, cancel := context.WithCancel(context.Background())
	nodecluster := node.CreateNodeCluster(k8sClient, modelnode, datacenterConfig)
	taskengin := task.CreateTaskEngine(nodecluster, modelnode)
	ms := &MasterServer{
		Client:           store.DefalutClient,
		TaskEngine:       taskengin,
		HostNode:         modelnode,
		Cluster:          nodecluster,
		ctx:              ctx,
		cancel:           cancel,
		datacenterConfig: datacenterConfig,
	}
	return ms, nil
}

//Start 启动
func (m *MasterServer) Start() error {
	//监控配置变化启动
	m.datacenterConfig.Start()
	if err := m.Cluster.Start(); err != nil {
		logrus.Error("node cluster start error,", err.Error())
		return err
	}
	if err := m.TaskEngine.Start(); err != nil {
		logrus.Error("task engin start error,", err.Error())
		return err
	}
	return nil
}

//Stop 停止
func (m *MasterServer) Stop(i interface{}) {
	if m.Cluster != nil {
		m.Cluster.Stop(i)
	}
	if m.TaskEngine != nil {
		m.TaskEngine.Stop()
	}
	m.cancel()
}
