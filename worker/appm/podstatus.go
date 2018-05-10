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

package appm

import (
	"github.com/goodrain/rainbond/event"
	"fmt"
	"sync"

	"k8s.io/client-go/pkg/api/v1"
)

//处理POD生命周期问题。并打日志处理

//PodStatus pod状态
type PodStatus struct {
	PodName           string
	PodScheduled      string
	PodScheduledError int
	Initialized       string
	Ready             string
	logger            event.Logger
}

//CacheManager  cache管理器
type CacheManager struct {
	caches map[string]*PodStatus
	lock   sync.Mutex
}

//NewCacheManager new
func NewCacheManager() *CacheManager {
	return &CacheManager{
		caches: make(map[string]*PodStatus),
	}
}

//AddPod 添加
func (m *CacheManager) AddPod(podName string, logger event.Logger) *PodStatus {
	m.lock.Lock()
	defer m.lock.Unlock()
	if s, ok := m.caches[podName]; ok {
		return s
	}
	m.caches[podName] = &PodStatus{PodName: podName, logger: logger}
	return m.caches[podName]
}

//RemovePod 移除POD状态
func (m *CacheManager) RemovePod(podName string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.caches, podName)
}

//AddStatus 更新POD状态
func (p *PodStatus) AddStatus(podStatus v1.PodStatus) (bool, error) {
	for _, cd := range podStatus.Conditions {
		if cd.Type == v1.PodScheduled {
			if p.PodScheduled == "" && cd.Status == v1.ConditionTrue {
				p.PodScheduled = string(cd.Status)
				p.logger.Info(fmt.Sprintf("实例(%s)调度已完成", p.PodName), map[string]string{"step": "worker-appm", "status": "success"})
			} else if cd.Status == v1.ConditionFalse {
				p.PodScheduledError++
				if p.PodScheduledError > 3 {
					p.logger.Error(fmt.Sprintf("实例(%s)调度失败", p.PodName), map[string]string{"step": "worker-appm", "status": "success"})
				}
			}
		}
		if cd.Type == v1.PodInitialized {
			if p.Initialized == "" && cd.Status == v1.ConditionTrue {
				p.Initialized = string(cd.Status)
				p.logger.Info(fmt.Sprintf("实例(%s)初始化已完成", p.PodName), map[string]string{"step": "worker-appm", "status": "success"})
			}
		}
		if cd.Type == v1.PodReady {
			if p.Ready == "" {
				var readyCount int
				for i := range podStatus.ContainerStatuses {
					c := podStatus.ContainerStatuses[i]
					if c.Ready {
						readyCount++
					} else {
						if c.State.Terminated != nil && c.State.Terminated.Reason == "Error" {
							p.logger.Debug(fmt.Sprintf("实例(%s)容器(%s)启动失败，即将关闭。请查看应用日志。", p.PodName, c.Name), map[string]string{"step": "worker-appm", "status": "failure"})
							return false, fmt.Errorf("应用容器重启")
						}
						p.logger.Debug(fmt.Sprintf("实例(%s)容器(%s)未启动完成.原因：(%s)", p.PodName, c.Name, buildReason(c.State)), map[string]string{"step": "worker-appm", "status": "warning"})
					}
				}
				if readyCount >= len(podStatus.ContainerStatuses) && cd.Status == v1.ConditionTrue {
					p.Ready = string(cd.Status)
					p.logger.Info(fmt.Sprintf("实例(%s)启动已完成", p.PodName), map[string]string{"step": "worker-appm", "status": "success"})
				}
			}
		}
	}
	if p.PodScheduled != "" && p.Initialized != "" && p.Ready != "" {
		return true, nil
	}
	return false, nil
}

func buildReason(state v1.ContainerState) string {
	var reason string
	if state.Waiting != nil {
		reason += "等待:" + state.Waiting.Reason
	}
	if state.Terminated != nil {
		reason += "终止:" + state.Terminated.Reason
	}
	return reason
}
