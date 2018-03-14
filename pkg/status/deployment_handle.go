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

package status

import (
	"github.com/goodrain/rainbond/pkg/db"

	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	"k8s.io/client-go/pkg/apis/apps/v1beta1"
)

// DeploymentUpdate describes an operation of deployment, sent on the channel.
// You can add, update or remove single endpoints by setting Op == ADD|UPDATE|REMOVE.
type DeploymentUpdate struct {
	Deployment *v1beta1.Deployment
	Op         Operation
}

func (s *statusManager) handleDeploymentUpdate(update DeploymentUpdate) {
	if update.Deployment == nil {
		return
	}
	var serviceID string
	deployIndo, err := db.GetManager().K8sDeployReplicationDao().GetK8sDeployReplication(update.Deployment.Name)
	if err != nil {
		if len(update.Deployment.Spec.Template.Spec.Containers) > 0 {
			for _, env := range update.Deployment.Spec.Template.Spec.Containers[0].Env {
				if env.Name == "SERVICE_ID" {
					serviceID = env.Value
				}
			}
		}
		if err != gorm.ErrRecordNotFound {
			logrus.Error("get deploy info from db error.", err.Error())
		}
	} else {
		serviceID = deployIndo.ServiceID
	}
	if serviceID == "" {
		logrus.Error("handle application(Deployment) status error. service id is empty")
		return
	}
	switch update.Op {
	case ADD:
		if update.Deployment.Status.Replicas == 0 {
			return
		}
		if update.Deployment.Status.ReadyReplicas >= update.Deployment.Status.Replicas {
			s.SetStatus(serviceID, RUNNING)
		}
		if update.Deployment.Status.ReadyReplicas < update.Deployment.Status.Replicas {
			status, _ := s.GetStatus(serviceID)
			if status == RUNNING {
				s.SetStatus(serviceID, ABNORMAL)
			}
			if status == CLOSED {
				s.SetStatus(serviceID, STARTING)
			}
		}
	case UPDATE:
		if update.Deployment.Status.Replicas == 0 {
			return
		}
		status, _ := s.GetStatus(serviceID)
		//Ready数量==需要实例数量，应用在运行中
		if update.Deployment.Status.ReadyReplicas >= update.Deployment.Status.Replicas {
			if status != STOPPING && status != UPGRADE {
				s.SetStatus(serviceID, RUNNING)
			}
		}
		if update.Deployment.Status.ReadyReplicas < update.Deployment.Status.Replicas {
			if status == RUNNING && !s.isIgnoreDelete(update.Deployment.Name) {
				s.SetStatus(serviceID, ABNORMAL)
			}
		}
	case REMOVE:
		// if deploy, _ := db.GetManager().K8sDeployReplicationDao().GetK8sDeployReplicationByService(serviceID); len(deploy) == 1 {
		// 	s.SetStatus(serviceID, CLOSED)
		// 	db.GetManager().K8sDeployReplicationDao().DeleteK8sDeployReplication(update.Deployment.Name)
		// }
		if !s.isIgnoreDelete(update.Deployment.Name) {
			s.SetStatus(serviceID, CLOSED)
			db.GetManager().K8sDeployReplicationDao().DeleteK8sDeployReplication(update.Deployment.Name)
		} else {
			s.RmIgnoreDelete(update.Deployment.Name)
		}
	}
}
