
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

package appm

import (
	"github.com/goodrain/rainbond/pkg/db"
	"github.com/goodrain/rainbond/pkg/db/model"
	"github.com/goodrain/rainbond/pkg/event"
	"github.com/goodrain/rainbond/pkg/util"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/apis/apps/v1beta1"

	"github.com/Sirupsen/logrus"
)

type DeploymentBuild struct {
	serviceID, eventID string
	podBuild           *PodTemplateSpecBuild
	dbmanager          db.Manager
	service            *model.TenantServices
	tenant             *model.Tenants
	logger             event.Logger
}

//DeploymentBuilder DeploymentBuilder
func DeploymentBuilder(serviceID string, logger event.Logger) (*DeploymentBuild, error) {
	podBuild, err := PodTemplateSpecBuilder(serviceID, logger)
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
func (s *DeploymentBuild) Build() (*v1beta1.Deployment, error) {
	pod, err := s.podBuild.Build()
	if err != nil {
		logrus.Error("pod template build error:", err.Error())
		return nil, fmt.Errorf("pod template build error: %s", err.Error())
	}
	deploymentSpec := v1beta1.DeploymentSpec{
		Template: *pod,
	}
	deploymentSpec.Replicas = int32Ptr(s.service.Replicas)
	deploymentSpec.Selector = metav1.SetAsLabelSelector(map[string]string{
		"name":    s.service.ServiceAlias,
		"version": s.service.DeployVersion,
	})
	deployment := &v1beta1.Deployment{
		Spec: deploymentSpec,
	}
	deployment.Namespace = s.tenant.UUID
	deployment.Name = util.NewUUID()
	deployment.GenerateName = s.service.ServiceAlias
	deployment.Labels = map[string]string{
		"name":    s.service.ServiceAlias,
		"version": s.service.DeployVersion,
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
