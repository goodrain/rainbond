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
		if err == gorm.ErrRecordNotFound {
			return ErrServiceNotFound
		}
		return fmt.Errorf("error getting service base info by serviceID(%s) %s", as.ServiceID, err.Error())
	}
	as.ServiceKind = dbmodel.ServiceKind(tenantService.Kind)
	tenant, err := dbmanager.TenantDao().GetTenantByUUID(tenantService.TenantID)
	if err != nil {
		return fmt.Errorf("get tenant info failure %s", err.Error())
	}
	as.TenantID = tenantService.TenantID
	if as.DeployVersion == "" {
		as.DeployVersion = tenantService.DeployVersion
	}
	as.AppID = tenantService.AppID
	as.ServiceAlias = tenantService.ServiceAlias
	as.UpgradeMethod = v1.TypeUpgradeMethod(tenantService.UpgradeMethod)
	if as.CreaterID == "" {
		as.CreaterID = string(util.NewTimeVersion())
	}
	as.TenantName = tenant.Name
	if err := initTenant(as, tenant); err != nil {
		return fmt.Errorf("conversion tenant info failure %s", err.Error())
	}
	if tenantService.Kind == dbmodel.ServiceKindThirdParty.String() {
		disCfg, _ := dbmanager.ThirdPartySvcDiscoveryCfgDao().GetByServiceID(as.ServiceID)
		as.SetDiscoveryCfg(disCfg)
		return nil
	}

	if tenantService.Kind == dbmodel.ServiceKindCustom.String() {
		return nil
	}
	label, _ := dbmanager.TenantServiceLabelDao().GetLabelByNodeSelectorKey(as.ServiceID, "windows")
	if label != nil {
		as.IsWindowsService = true
	}

	// component resource config
	as.ContainerCPU = tenantService.ContainerCPU
	as.ContainerGPU = tenantService.ContainerGPU
	as.ContainerMemory = tenantService.ContainerMemory
	as.Replicas = tenantService.Replicas
	if !tenantService.IsState() {
		initBaseDeployment(as, tenantService)
		return nil
	}
	if tenantService.IsState() {
		initBaseStatefulSet(as, tenantService)
		return nil
	}
	return fmt.Errorf("kind: %s; do not decision build type for service %s", tenantService.Kind, as.ServiceAlias)
}

func initTenant(as *v1.AppService, tenant *dbmodel.Tenants) error {
	if tenant == nil || tenant.UUID == "" {
		return fmt.Errorf("tenant is invalid")
	}
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   tenant.UUID,
			Labels: map[string]string{"creator": "Rainbond"},
		},
	}
	as.SetTenant(namespace)
	return nil
}
func initSelector(selector *metav1.LabelSelector, service *dbmodel.TenantServices) {
	if selector.MatchLabels == nil {
		selector.MatchLabels = make(map[string]string)
	}
	selector.MatchLabels["name"] = service.ServiceAlias
	selector.MatchLabels["tenant_id"] = service.TenantID
	selector.MatchLabels["service_id"] = service.ServiceID
	//selector.MatchLabels["version"] = service.DeployVersion
}
func initBaseStatefulSet(as *v1.AppService, service *dbmodel.TenantServices) {
	as.ServiceType = v1.TypeStatefulSet
	stateful := as.GetStatefulSet()
	if stateful == nil {
		stateful = &appsv1.StatefulSet{}
	}
	stateful.Namespace = as.TenantID
	stateful.Spec.Replicas = int32Ptr(service.Replicas)
	if stateful.Spec.Selector == nil {
		stateful.Spec.Selector = &metav1.LabelSelector{}
	}
	initSelector(stateful.Spec.Selector, service)
	stateful.Spec.ServiceName = service.ServiceName
	stateful.Name = service.ServiceName
	if stateful.Spec.ServiceName == "" {
		stateful.Spec.ServiceName = service.ServiceAlias
		stateful.Name = service.ServiceAlias
	}
	stateful.Namespace = service.TenantID
	stateful.GenerateName = service.ServiceAlias
	stateful.Labels = as.GetCommonLabels(stateful.Labels, map[string]string{
		"name":    service.ServiceAlias,
		"version": service.DeployVersion,
	})
	stateful.Spec.UpdateStrategy.Type = appsv1.RollingUpdateStatefulSetStrategyType
	if as.UpgradeMethod == v1.OnDelete {
		stateful.Spec.UpdateStrategy.Type = appsv1.OnDeleteStatefulSetStrategyType
	}
	as.SetStatefulSet(stateful)
}

func initBaseDeployment(as *v1.AppService, service *dbmodel.TenantServices) {
	as.ServiceType = v1.TypeDeployment
	deployment := as.GetDeployment()
	if deployment == nil {
		deployment = &appsv1.Deployment{}
	}
	deployment.Namespace = as.TenantID
	deployment.Spec.Replicas = int32Ptr(service.Replicas)
	if deployment.Spec.Selector == nil {
		deployment.Spec.Selector = &metav1.LabelSelector{}
	}
	initSelector(deployment.Spec.Selector, service)
	deployment.Namespace = service.TenantID
	deployment.Name = service.ServiceID + "-deployment"
	deployment.GenerateName = strings.Replace(service.ServiceAlias, "_", "-", -1)
	deployment.Labels = as.GetCommonLabels(deployment.Labels, map[string]string{
		"name":    service.ServiceAlias,
		"version": service.DeployVersion,
	})
	deployment.Spec.Strategy.Type = appsv1.RollingUpdateDeploymentStrategyType
	if as.UpgradeMethod == v1.OnDelete {
		deployment.Spec.Strategy.Type = appsv1.RecreateDeploymentStrategyType
	}
	as.SetDeployment(deployment)
}
