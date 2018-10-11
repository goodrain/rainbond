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

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/apis/apps/v1beta1"

	"github.com/Sirupsen/logrus"
)

//DeploymentBuild DeploymentBuild
type DeploymentBuild struct {
	serviceID, eventID string
	podBuild           *PodTemplateSpecBuild
	dbmanager          db.Manager
	service            *model.TenantServices
	tenant             *model.Tenants
	logger             event.Logger
}

//DeploymentBuilder DeploymentBuilder
func DeploymentBuilder(serviceID string, logger event.Logger, nodeAPI string) (*DeploymentBuild, error) {
	podBuild, err := PodTemplateSpecBuilder(serviceID, logger, nodeAPI)
	if err != nil {
		logrus.Error("create pod template build error.", err.Error())
		return nil, err
	}
	return &DeploymentBuild{
		serviceID: serviceID,
		eventID:   logger.Event(),
		podBuild:  podBuild,
		tenant:    podBuild.GetTenant(),
		service:   podBuild.GetService(),
		dbmanager: db.GetManager(),
	}, nil
}

//Build 构建
func (s *DeploymentBuild) Build(creatorID string) (*v1beta1.Deployment, error) {
	pod, err := s.podBuild.Build(creatorID)
	if err != nil {
		logrus.Error("pod template build error:", err.Error())
		return nil, fmt.Errorf("pod template build error: %s", err.Error())
	}
	deploymentSpec := v1beta1.DeploymentSpec{
		Template: *pod,
	}
	deploymentSpec.Replicas = int32Ptr(s.service.Replicas)
	deploymentSpec.Selector = metav1.SetAsLabelSelector(map[string]string{
		"name": s.service.ServiceAlias,
		//todo
		"version": s.service.DeployVersion,
	})
	deployment := &v1beta1.Deployment{
		Spec: deploymentSpec,
	}
	deployment.Namespace = s.tenant.UUID
	deployment.Name = util.NewUUID()
	deployment.GenerateName = s.service.ServiceAlias
	deployment.Labels = map[string]string{
		"name": s.service.ServiceAlias,
		//todo
		"version":    s.service.DeployVersion,
		"creator":    "RainBond",
		"creator_id": creatorID,
		"service_id": s.service.ServiceID,
	}
	deployment.Kind = "Deployment"
	//TODO: 根据k8s版本进行更改
	deployment.APIVersion = "apps/v1beta1"
	return deployment, nil
}

//GetTenant 获取租户id
func (s *DeploymentBuild) GetTenant() string {
	return s.tenant.UUID
}
