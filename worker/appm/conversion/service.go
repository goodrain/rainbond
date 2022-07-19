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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	apimodel "github.com/goodrain/rainbond/api/model"

	"github.com/goodrain/rainbond/api/handler/app_governance_mode/adaptor"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/batch/v1beta1"

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
	if tenantService.K8sComponentName == "" {
		tenantService.K8sComponentName = tenantService.ServiceAlias
	}
	as.K8sComponentName = tenantService.K8sComponentName
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
	if tenantService.IsJob() {
		initBaseJob(as, tenantService)
		return nil
	}
	if tenantService.IsCronJob() {
		initBaseCronJob(as, tenantService)
		return nil
	}
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
	if tenant == nil || tenant.Namespace == "" {
		return fmt.Errorf("tenant is invalid")
	}
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   tenant.Namespace,
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
	stateful.Namespace = as.GetNamespace()
	stateful.Spec.Replicas = int32Ptr(service.Replicas)
	if stateful.Spec.Selector == nil {
		stateful.Spec.Selector = &metav1.LabelSelector{}
	}
	initSelector(stateful.Spec.Selector, service)
	stateful.Name = as.GetK8sWorkloadName()
	stateful.Spec.ServiceName = as.GetK8sWorkloadName()
	stateful.GenerateName = service.ServiceAlias
	injectLabels := getInjectLabels(as)
	stateful.Labels = as.GetCommonLabels(stateful.Labels, map[string]string{
		"name":    service.ServiceAlias,
		"version": service.DeployVersion,
	}, injectLabels)
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
	deployment.Namespace = as.GetNamespace()
	deployment.Spec.Replicas = int32Ptr(service.Replicas)
	if deployment.Spec.Selector == nil {
		deployment.Spec.Selector = &metav1.LabelSelector{}
	}
	initSelector(deployment.Spec.Selector, service)
	deployment.Name = as.GetK8sWorkloadName()
	deployment.GenerateName = strings.Replace(service.ServiceAlias, "_", "-", -1)
	injectLabels := getInjectLabels(as)
	deployment.Labels = as.GetCommonLabels(deployment.Labels, map[string]string{
		"name":    service.ServiceAlias,
		"version": service.DeployVersion,
	}, injectLabels)
	deployment.Spec.Strategy.Type = appsv1.RollingUpdateDeploymentStrategyType
	if as.UpgradeMethod == v1.OnDelete {
		deployment.Spec.Strategy.Type = appsv1.RecreateDeploymentStrategyType
	}
	as.SetDeployment(deployment)
}

func initBaseJob(as *v1.AppService, service *dbmodel.TenantServices) {
	as.ServiceType = v1.TypeJob
	job := as.GetJob()
	if job == nil {
		job = &batchv1.Job{}
	}
	job.Namespace = as.GetNamespace()
	if job.Spec.Selector == nil {
		job.Spec.Selector = &metav1.LabelSelector{}
	}
	job.Name = as.GetK8sWorkloadName()
	job.GenerateName = strings.Replace(service.ServiceAlias, "_", "-", -1)
	injectLabels := getInjectLabels(as)
	job.Labels = as.GetCommonLabels(job.Labels, map[string]string{
		"name":    service.ServiceAlias,
		"version": service.DeployVersion,
	}, injectLabels)

	var js *apimodel.JobStrategy
	if service.JobStrategy != ""{
		err := json.Unmarshal([]byte(service.JobStrategy), &js)
		if err != nil {
			logrus.Error("job strategy json unmarshal error", err)
		}
		if js.ActiveDeadlineSeconds != "" {
			ads, err := strconv.ParseInt(js.ActiveDeadlineSeconds, 10, 64)
			if err != nil {
				logrus.Error("activeDeadlineSeconds ParseInt error", err)
			}
			job.Spec.ActiveDeadlineSeconds = &ads
		}
		if js.BackoffLimit != "" {
			res, err := strconv.ParseInt(js.BackoffLimit, 10, 32)
			if err != nil {
				logrus.Error("BackoffLimit ParseInt error", err)
			}
			bkl := int32(res)
			job.Spec.BackoffLimit = &bkl
		}
		if js.Parallelism != "" {
			res, err := strconv.ParseInt(js.Parallelism, 10, 32)
			if err != nil {
				logrus.Error("Parallelism ParseInt error", err)
			}
			pll := int32(res)
			job.Spec.Parallelism = &pll
		}
		if js.Completions != "" {
			res, err := strconv.ParseInt(js.Completions, 10, 32)
			if err != nil {
				logrus.Error("Completions ParseInt error", err)
			}
			cpt := int32(res)
			job.Spec.Completions = &cpt
		}
	}
	as.SetJob(job)
}

func initBaseCronJob(as *v1.AppService, service *dbmodel.TenantServices) {
	as.ServiceType = v1.TypeCronJob
	cronJob := as.GetCronJob()
	if cronJob == nil {
		cronJob = &v1beta1.CronJob{}
	}
	injectLabels := getInjectLabels(as)
	jobTemp := v1beta1.JobTemplateSpec{}
	jobTemp.Name = as.GetK8sWorkloadName()
	jobTemp.Namespace = as.GetNamespace()
	jobTemp.Labels = as.GetCommonLabels(jobTemp.Labels, map[string]string{
		"name":    service.ServiceAlias,
		"version": service.DeployVersion,
	}, injectLabels)
	if service.JobStrategy != ""{
		var js *apimodel.JobStrategy
		err := json.Unmarshal([]byte(service.JobStrategy), &js)
		if err != nil {
			logrus.Error("job strategy json unmarshal error", err)
		}
		if js.ActiveDeadlineSeconds != "" {
			ads, err := strconv.ParseInt(js.ActiveDeadlineSeconds, 10, 64)
			if err != nil {
				logrus.Error("activeDeadlineSeconds ParseInt error", err)
			}
			jobTemp.Spec.ActiveDeadlineSeconds = &ads
		}
		if js.BackoffLimit != "" {
			res, err := strconv.ParseInt(js.BackoffLimit, 10, 32)
			if err != nil {
				logrus.Error("BackoffLimit ParseInt error", err)
			}
			bkl := int32(res)
			jobTemp.Spec.BackoffLimit = &bkl
		}
		if js.Parallelism != "" {
			res, err := strconv.ParseInt(js.Parallelism, 10, 32)
			if err != nil {
				logrus.Error("Parallelism ParseInt error", err)
			}
			pll := int32(res)
			jobTemp.Spec.Parallelism = &pll
		}
		if js.Completions != "" {
			res, err := strconv.ParseInt(js.Completions, 10, 32)
			if err != nil {
				logrus.Error("Completions ParseInt error", err)
			}
			cpt := int32(res)
			jobTemp.Spec.Completions = &cpt
		}
		cronJob.Spec.Schedule = js.Schedule
	}
	cronJob.Spec.JobTemplate = jobTemp
	cronJob.Namespace = as.GetNamespace()
	cronJob.Name = as.GetK8sWorkloadName()
	as.SetCronJob(cronJob)
}

func getInjectLabels(as *v1.AppService) map[string]string {
	mode, err := adaptor.NewAppGoveranceModeHandler(as.GovernanceMode, nil)
	if err != nil {
		logrus.Warningf("getInjectLabels failed: %v", err)
		return nil
	}
	injectLabels := mode.GetInjectLabels()
	return injectLabels
}
