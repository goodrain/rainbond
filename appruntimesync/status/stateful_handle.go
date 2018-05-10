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
	"fmt"

	"github.com/goodrain/rainbond/appruntimesync/source"
	"github.com/goodrain/rainbond/db"

	"github.com/Sirupsen/logrus"
	"github.com/jinzhu/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"
)

func (s *Manager) handleStatefulUpdate(update source.StatefulSetUpdate) {
	if update.StatefulSet == nil {
		return
	}
	var serviceID string
	deployIndo, err := db.GetManager().K8sDeployReplicationDao().GetK8sDeployReplication(update.StatefulSet.Name)
	if err != nil {
		if len(update.StatefulSet.Spec.Template.Spec.Containers) > 0 {
			for _, env := range update.StatefulSet.Spec.Template.Spec.Containers[0].Env {
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
		logrus.Error("handle application(StatefulSet) status error. service id is empty")
		return
	}
	readycount := s.getReadyCount(update.StatefulSet.Namespace,
		update.StatefulSet.Labels["name"],
		update.StatefulSet.Labels["version"])
	switch update.Op {
	case source.ADD:
		if update.StatefulSet.Status.Replicas == 0 {
			return
		}
		logrus.Infof("Stateful application ready count %d of service %s", readycount, update.StatefulSet.Labels["name"])
		if readycount >= update.StatefulSet.Status.Replicas {
			s.SetStatus(serviceID, RUNNING)
		}
		if readycount < update.StatefulSet.Status.Replicas {
			status := s.GetStatus(serviceID)
			if status == RUNNING {
				s.SetStatus(serviceID, ABNORMAL)
			}
			if status == CLOSED {
				s.SetStatus(serviceID, STARTING)
			}
		}
	case source.UPDATE:
		if update.StatefulSet.Status.Replicas == 0 {
			return
		}
		status := s.GetStatus(serviceID)
		//Ready数量==需要实例数量，应用在运行中
		if readycount >= update.StatefulSet.Status.Replicas {
			if status != STOPPING && status != UPGRADE {
				s.SetStatus(serviceID, RUNNING)
			}
		}
		if readycount < update.StatefulSet.Status.Replicas {
			if status == RUNNING && !s.isIgnoreDelete(update.StatefulSet.Name) {
				s.SetStatus(serviceID, ABNORMAL)
			}
		}
	case source.REMOVE:
		// if deploy, _ := db.GetManager().K8sDeployReplicationDao().GetK8sDeployReplicationByService(serviceID); len(deploy) == 1 {
		// 	s.SetStatus(serviceID, CLOSED)
		// 	db.GetManager().K8sDeployReplicationDao().DeleteK8sDeployReplication(update.StatefulSet.Name)
		// }
		if !s.isIgnoreDelete(update.StatefulSet.Name) {
			s.SetStatus(serviceID, CLOSED)
			db.GetManager().K8sDeployReplicationDao().DeleteK8sDeployReplication(update.StatefulSet.Name)
		} else {
			s.RmIgnoreDelete(update.StatefulSet.Name)
		}
	}
}

func (s *Manager) getReadyCount(namespace, name, version string) int32 {
	pods, err := s.ClientSet.Core().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("name=%s,version=%s", name, version),
	})
	if err != nil {
		logrus.Error("list application pods error.", err.Error())
		return 0
	}
	readyReplicasCount := 0
	for _, pod := range pods.Items {
		if IsPodReady(&pod) {
			readyReplicasCount++
		}
	}
	return int32(readyReplicasCount)
}

// GetPodCondition extracts the provided condition from the given status and returns that.
// Returns nil and -1 if the condition is not present, and the index of the located condition.
func GetPodCondition(status *v1.PodStatus, conditionType v1.PodConditionType) (int, *v1.PodCondition) {
	if status == nil {
		return -1, nil
	}
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return i, &status.Conditions[i]
		}
	}
	return -1, nil
}

// IsPodReady returns true if a pod is ready; false otherwise.
func IsPodReady(pod *v1.Pod) bool {
	return IsPodReadyConditionTrue(pod.Status)
}

// IsPodReadyConditionTrue retruns true if a pod is ready; false otherwise.
func IsPodReadyConditionTrue(status v1.PodStatus) bool {
	_, condition := GetPodCondition(&status, v1.PodReady)
	return condition != nil && condition.Status == v1.ConditionTrue
}
