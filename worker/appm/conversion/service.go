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

package conversion

import (
	"fmt"
	"strings"

	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/jinzhu/gorm"
	yaml "gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//ServiceSource conv ServiceSource
func ServiceSource(as *v1.AppService, dbmanager db.Manager) error {
	sscs, err := dbmanager.ServiceSourceDao().GetServiceSource(as.ServiceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return fmt.Errorf("conv service source failure %s", err.Error())
	}
	for _, ssc := range sscs {
		switch ssc.SourceType {
		case "deployment":
			var dm appsv1.Deployment
			if err := decoding(ssc.SourceBody, &dm); err != nil {
				return decodeError(err)
			}
			as.SetDeployment(&dm)
		case "statefulset":
			var ss appsv1.StatefulSet
			if err := decoding(ssc.SourceBody, &ss); err != nil {
				return decodeError(err)
			}
			as.SetStatefulSet(&ss)
		case "configmap":
			var cm corev1.ConfigMap
			if err := decoding(ssc.SourceBody, &cm); err != nil {
				return decodeError(err)
			}
			as.SetConfigMap(&cm)
		}
	}
	return nil
}
func decodeError(err error) error {
	return fmt.Errorf("decode service source failure %s", err.Error())
}
func decoding(source string, target interface{}) error {
	return yaml.Unmarshal([]byte(source), target)
}
func int32Ptr(i int) *int32 {
	j := int32(i)
	return &j
}

//TenantServiceBase conv tenant service base info
func TenantServiceBase(as *v1.AppService, dbmanager db.Manager) error {
	tenantService, err := dbmanager.TenantServiceDao().GetServiceByID(as.ServiceID)
	if err != nil {
		return fmt.Errorf("get service base info failure %s", err.Error())
	}
	tenant, err := dbmanager.TenantDao().GetTenantByUUID(tenantService.TenantID)
	if err != nil {
		return fmt.Errorf("get tenant info failure %s", err.Error())
	}
	serviceType, err := dbmanager.TenantServiceLabelDao().GetTenantServiceTypeLabel(as.ServiceID)
	if err != nil {
		return fmt.Errorf("get service type info failure %s", err.Error())
	}
	as.TenantID = tenantService.TenantID
	as.DeployVersion = tenantService.DeployVersion
	as.ContainerCPU = tenantService.ContainerCPU
	as.ContainerMemory = tenantService.ContainerMemory
	as.Replicas = tenantService.Replicas
	as.ServiceAlias = tenantService.ServiceAlias
	as.CreaterID = util.NewUUID()
	as.TenantName = tenant.Name
	if serviceType.LabelValue == util.StatefulServiceType {
		as.ServiceType = v1.TypeStatefulSet
		statefulset := as.GetStatefulSet()
		if statefulset == nil {
			statefulset = &appsv1.StatefulSet{}
		}
		initBaseStatefulSet(tenantService, statefulset)
		as.SetStatefulSet(statefulset)
	}
	if serviceType.LabelValue == util.StatelessServiceType {
		as.ServiceType = v1.TypeDeployment
		deployment := as.GetDeployment()
		if deployment == nil {
			deployment = &appsv1.Deployment{}
		}
		initBaseDeployment(tenantService, deployment)
		as.SetDeployment(deployment)
	}
	return nil
}

func initSelector(selector *metav1.LabelSelector, service *dbmodel.TenantServices) {
	selector.MatchLabels["name"] = service.ServiceAlias
	selector.MatchLabels["version"] = service.DeployVersion
}
func initBaseStatefulSet(service *dbmodel.TenantServices, stateful *appsv1.StatefulSet) {
	stateful.Spec.Replicas = int32Ptr(service.Replicas)
	if stateful.Spec.Selector == nil {
		stateful.Spec.Selector = &metav1.LabelSelector{}
	}
	initSelector(stateful.Spec.Selector, service)
	stateful.Spec.ServiceName = service.ServiceName
	stateful.Namespace = service.TenantID
	stateful.Name = service.ServiceAlias
	stateful.GenerateName = service.ServiceAlias
	stateful.Labels = getCommonLable(stateful.Labels, map[string]string{
		"name":       service.ServiceAlias,
		"version":    service.DeployVersion,
		"service_id": service.ServiceID,
	})
}

func initBaseDeployment(service *dbmodel.TenantServices, deploymemnt *appsv1.Deployment) {
	deploymemnt.Spec.Replicas = int32Ptr(service.Replicas)
	if deploymemnt.Spec.Selector == nil {
		deploymemnt.Spec.Selector = &metav1.LabelSelector{}
	}
	initSelector(deploymemnt.Spec.Selector, service)
	deploymemnt.Namespace = service.TenantID
	deploymemnt.Name = util.NewUUID()
	deploymemnt.GenerateName = strings.Replace(service.ServiceAlias, "_", "-", -1)
	deploymemnt.Labels = getCommonLable(deploymemnt.Labels, map[string]string{
		"name":       service.ServiceAlias,
		"version":    service.DeployVersion,
		"service_id": service.ServiceID,
	})
}
