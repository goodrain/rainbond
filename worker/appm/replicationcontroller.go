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
	"github.com/goodrain/rainbond/util"

	"k8s.io/client-go/pkg/api/v1"

	"github.com/Sirupsen/logrus"
)

//ReplicationControllerBuild ReplicationControllerBuild
type ReplicationControllerBuild struct {
	serviceID, eventID string
	podBuild           *PodTemplateSpecBuild
	dbmanager          db.Manager
	service            *model.TenantServices
	tenant             *model.Tenants
	logger             event.Logger
}

//ReplicationControllerBuilder ReplicationControllerBuilder
func ReplicationControllerBuilder(serviceID string, logger event.Logger, nodeAPI string) (*ReplicationControllerBuild, error) {
	podBuild, err := PodTemplateSpecBuilder(serviceID, logger, nodeAPI)
	if err != nil {
		logrus.Error("create pod template build error.", err.Error())
		return nil, err
	}
	return &ReplicationControllerBuild{
		serviceID: serviceID,
		eventID:   logger.Event(),
		podBuild:  podBuild,
		tenant:    podBuild.GetTenant(),
		service:   podBuild.GetService(),
		dbmanager: db.GetManager(),
	}, nil
}

//Build 构建
func (s *ReplicationControllerBuild) Build(creatorID string) (*v1.ReplicationController, error) {
	pod, err := s.podBuild.Build(creatorID)
	if err != nil {
		logrus.Error("pod template build error:", err.Error())
		return nil, fmt.Errorf("pod template build error: %s", err.Error())
	}
	rcSpec := v1.ReplicationControllerSpec{
		Template: pod,
	}
	rcSpec.Replicas = int32Ptr(s.service.Replicas)
	rcSpec.Selector = map[string]string{
		"name":    s.service.ServiceAlias,
		"version": s.service.DeployVersion,
	}
	rc := &v1.ReplicationController{
		Spec: rcSpec,
	}
	rc.Namespace = s.tenant.UUID
	rc.Name = util.NewUUID()
	rc.GenerateName = strings.Replace(s.service.ServiceAlias, "_", "-", -1)
	rc.Labels = map[string]string{
		"name":       s.service.ServiceAlias,
		"version":    s.service.DeployVersion,
		"creator":    "RainBond",
		"creator_id": creatorID,
		"service_id": s.service.ServiceID,
	}
	rc.Kind = "ReplicationController"
	//TODO: 根据k8s版本进行更改
	rc.APIVersion = "apps/v1beta1"
	return rc, nil
}

//GetTenant 获取租户id
func (s *ReplicationControllerBuild) GetTenant() string {
	return s.tenant.UUID
}
