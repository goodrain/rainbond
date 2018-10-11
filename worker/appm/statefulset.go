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
	"fmt"
	"strings"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/apis/apps/v1beta1"

	"github.com/Sirupsen/logrus"
)

//StatefulSetBuild StatefulSetBuild
type StatefulSetBuild struct {
	serviceID, eventID string
	podBuild           *PodTemplateSpecBuild
	dbmanager          db.Manager
	service            *model.TenantServices
	tenant             *model.Tenants
	logger             event.Logger
}

//StatefulSetBuilder StatefulSetBuilder
func StatefulSetBuilder(serviceID string, logger event.Logger, nodeAPI string) (*StatefulSetBuild, error) {
	podBuild, err := PodTemplateSpecBuilder(serviceID, logger, nodeAPI)
	if err != nil {
		logrus.Error("create pod template build error.", err.Error())
		return nil, err
	}
	return &StatefulSetBuild{
		serviceID: serviceID,
		eventID:   logger.Event(),
		podBuild:  podBuild,
		tenant:    podBuild.GetTenant(),
		service:   podBuild.GetService(),
		dbmanager: db.GetManager(),
	}, nil
}
func int32Ptr(i int) *int32 {
	j := int32(i)
	return &j
}

//Build 构建
func (s *StatefulSetBuild) Build(creatorID string) (*v1beta1.StatefulSet, error) {
	pod, err := s.podBuild.Build(creatorID)
	if err != nil {
		logrus.Error("pod template build error:", err.Error())
		return nil, fmt.Errorf("pod template build error: %s", err.Error())
	}
	//有状态服务挂载目录地址到POD级。即每个POD挂载以不变PODNAME结尾的目录
	//原路径: /grdata/tenant/***/services/***
	//有状态服务路径: /grdata/tenant/***/services/***/PODNAME
	if len(pod.Spec.Containers) > 0 && len(pod.Spec.Containers[0].VolumeMounts) > 0 {
		for j := range pod.Spec.Containers {
			for i := range pod.Spec.Containers[j].VolumeMounts {
				vm := pod.Spec.Containers[j].VolumeMounts[i]
				var podName = "PODNAME"
				if strings.HasPrefix(vm.Name, "manual") || strings.HasPrefix(vm.Name, "mnt") || strings.HasPrefix(vm.Name, "vol") {
					pod.Spec.Containers[j].VolumeMounts[i].SubPath = podName
				}
			}
		}
	}
	statefulSpec := v1beta1.StatefulSetSpec{
		Template: *pod,
	}
	statefulSpec.Replicas = int32Ptr(s.service.Replicas)
	statefulSpec.Selector = metav1.SetAsLabelSelector(map[string]string{
		"name":    s.service.ServiceAlias,
		"version": s.service.DeployVersion,
	})
	//赋值 对应的ServiceName
	statefulSpec.ServiceName = s.service.ServiceAlias
	stateful := &v1beta1.StatefulSet{
		Spec: statefulSpec,
	}
	stateful.Namespace = s.tenant.UUID
	stateful.Name = s.service.ServiceAlias
	stateful.GenerateName = s.service.ServiceAlias
	stateful.Labels = map[string]string{
		"name":       s.service.ServiceAlias,
		"version":    s.service.DeployVersion,
		"creator":    "RainBond",
		"creator_id": creatorID,
		"service_id": s.service.ServiceID,
	}
	stateful.Kind = "StatefulSet"
	//TODO: 根据k8s版本进行更改
	stateful.APIVersion = "apps/v1beta1"
	return stateful, nil
}

//GetTenant 获取租户id
func (s *StatefulSetBuild) GetTenant() string {
	return s.tenant.UUID
}
