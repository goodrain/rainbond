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
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"strconv"
	"strings"

	apimodel "github.com/goodrain/rainbond/api/model"

	"github.com/goodrain/rainbond/api/handler/app_governance_mode/adaptor"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceSource conv ServiceSource
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

// TenantServiceBase conv tenant service base info
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
	if tenantService.IsVM() {
		initBaseVirtualMachine(as, tenantService)
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

func initBaseVirtualMachine(as *v1.AppService, service *dbmodel.TenantServices) {
	as.ServiceType = v1.TypeVirtualMachine
	vm := as.GetVirtualMachine()
	if vm == nil {
		vm = &kubevirtv1.VirtualMachine{}
	}
	vm.Namespace = as.GetNamespace()
	vm.Name = as.GetK8sWorkloadName()
	vmTem := kubevirtv1.VirtualMachineInstanceTemplateSpec{ObjectMeta: metav1.ObjectMeta{}}
	vm.Spec.Template = &vmTem
	vm.GenerateName = strings.Replace(service.ServiceAlias, "_", "-", -1)
	injectLabels := getInjectLabels(as)
	vm.Labels = as.GetCommonLabels(vm.Labels, map[string]string{
		"name":               service.ServiceAlias,
		"version":            service.DeployVersion,
		"kubevirt.io/domain": as.GetK8sWorkloadName(),
	}, injectLabels)
	r := kubevirtv1.RunStrategyAlways
	vm.Spec.RunStrategy = &r

	as.SetVirtualMachine(vm)
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
	if service.JobStrategy != "" {
		err := json.Unmarshal([]byte(service.JobStrategy), &js)
		if err != nil {
			logrus.Error("job strategy json unmarshal error", err)
		}
		if js.ActiveDeadlineSeconds != "" {
			ads, err := strconv.ParseInt(js.ActiveDeadlineSeconds, 10, 64)
			if err == nil {
				job.Spec.ActiveDeadlineSeconds = &ads
			}
		}
		if js.BackoffLimit != "" {
			res, err := strconv.ParseInt(js.BackoffLimit, 10, 32)
			if err == nil {
				bkl := int32(res)
				job.Spec.BackoffLimit = &bkl
			}
		}
		if js.Parallelism != "" {
			res, err := strconv.ParseInt(js.Parallelism, 10, 32)
			if err == nil {
				pll := int32(res)
				job.Spec.Parallelism = &pll
			}
		}
		if js.Completions != "" {
			res, err := strconv.ParseInt(js.Completions, 10, 32)
			if err == nil {
				cpt := int32(res)
				job.Spec.Completions = &cpt
			}
		}
	}
	as.SetJob(job)
}

func initBaseCronJob(as *v1.AppService, service *dbmodel.TenantServices) {
	as.ServiceType = v1.TypeCronJob
	injectLabels := getInjectLabels(as)
	jobTemp := batchv1.JobTemplateSpec{}
	jobTemp.Name = as.GetK8sWorkloadName()
	jobTemp.Namespace = as.GetNamespace()
	jobTemp.Labels = as.GetCommonLabels(jobTemp.Labels, map[string]string{
		"name":    service.ServiceAlias,
		"version": service.DeployVersion,
	}, injectLabels)
	var schedule string
	if service.JobStrategy != "" {
		var js *apimodel.JobStrategy
		err := json.Unmarshal([]byte(service.JobStrategy), &js)
		if err != nil {
			logrus.Error("job strategy json unmarshal error", err)
		}
		if js.ActiveDeadlineSeconds != "" {
			ads, err := strconv.ParseInt(js.ActiveDeadlineSeconds, 10, 64)
			if err == nil {
				jobTemp.Spec.ActiveDeadlineSeconds = &ads
			}
		}
		if js.BackoffLimit != "" {
			res, err := strconv.ParseInt(js.BackoffLimit, 10, 32)
			if err == nil {
				bkl := int32(res)
				jobTemp.Spec.BackoffLimit = &bkl
			}
		}
		if js.Parallelism != "" {
			res, err := strconv.ParseInt(js.Parallelism, 10, 32)
			if err == nil {
				pll := int32(res)
				jobTemp.Spec.Parallelism = &pll
			}
		}
		if js.Completions != "" {
			res, err := strconv.ParseInt(js.Completions, 10, 32)
			if err == nil {
				cpt := int32(res)
				jobTemp.Spec.Completions = &cpt
			}
		}
		schedule = js.Schedule
	}

	if k8sutil.GetKubeVersion().AtLeast(utilversion.MustParseSemantic("v1.21.0")) {
		cronJob := as.GetCronJob()
		if cronJob == nil {
			cronJob = &batchv1.CronJob{}
		}
		cronJob.Spec.Schedule = schedule
		cronJob.Spec.JobTemplate = jobTemp
		cronJob.Namespace = as.GetNamespace()
		cronJob.Name = as.GetK8sWorkloadName()
		as.SetCronJob(cronJob)
		return
	}
	cronJob := as.GetBetaCronJob()
	if cronJob == nil {
		cronJob = &batchv1beta1.CronJob{}
	}
	cronJob.Spec.JobTemplate = batchv1beta1.JobTemplateSpec{
		ObjectMeta: jobTemp.ObjectMeta,
		Spec:       jobTemp.Spec,
	}
	cronJob.Namespace = as.GetNamespace()
	cronJob.Name = as.GetK8sWorkloadName()
	as.SetBetaCronJob(cronJob)
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

func CreateHttproute(k8sApp, namespace, appID string, service []*dbmodel.TenantServicesPort, component *dbmodel.TenantServices, gatewayClient *v1beta1.GatewayV1beta1Client) (string, error) {
	name := k8sApp + "-" + component.K8sComponentName
	labels := make(map[string]string)
	labels["app_id"] = appID
	labels["component_id"] = component.ServiceID
	labels["gray_route"] = "true"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	kind := gatewayv1beta1.Kind(apimodel.Service)
	var serviceName string
	var port gatewayv1beta1.PortNumber
	if service != nil && len(service) > 0 {
		serviceName = service[0].K8sServiceName
		port = gatewayv1beta1.PortNumber(int32(service[0].ContainerPort))
	}
	ns := gatewayv1beta1.Namespace(namespace)
	parentReference := gatewayv1beta1.ParentReference{
		Kind: &kind,
		Name: gatewayv1beta1.ObjectName(serviceName),
	}
	weight := int32(1)
	rule := gatewayv1beta1.HTTPRouteRule{
		BackendRefs: []gatewayv1beta1.HTTPBackendRef{
			{
				BackendRef: gatewayv1beta1.BackendRef{
					BackendObjectReference: gatewayv1beta1.BackendObjectReference{
						Kind:      &kind,
						Port:      &port,
						Namespace: &ns,
						Name:      gatewayv1beta1.ObjectName(serviceName),
					},
					Weight: &weight,
				},
			},
		},
	}
	httpRoute := gatewayv1beta1.HTTPRoute{
		TypeMeta: metav1.TypeMeta{
			Kind:       apimodel.HTTPRoute,
			APIVersion: apimodel.APIVersionHTTPRoute,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: gatewayv1beta1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1beta1.CommonRouteSpec{
				ParentRefs: []gatewayv1beta1.ParentReference{
					parentReference,
				},
			},
			Rules: []gatewayv1beta1.HTTPRouteRule{
				rule,
			},
		},
	}
	_, err := gatewayClient.HTTPRoutes(namespace).Create(ctx, &httpRoute, metav1.CreateOptions{})
	if err != nil && !k8serror.IsAlreadyExists(err) {
		return "", err
	}
	return name, nil
}

func CreateRollout(k8sApp, namespace string, service []*dbmodel.TenantServicesPort, component *dbmodel.TenantServices, gray *dbmodel.AppGrayRelease, kruiseClient *versioned.Clientset, gatewayClient *v1beta1.GatewayV1beta1Client) error {
	annotations := make(map[string]string)
	annotations["rollouts.kruise.io/rolling-style"] = "partition"
	name := k8sApp + "-" + component.K8sComponentName
	labels := make(map[string]string)
	labels["app_id"] = gray.AppID
	labels["component_id"] = component.ServiceID
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var hostname, httpRouteName string
	if gray.EntryComponentID == component.ServiceID {
		httpRouteName = gray.EntryHTTPRoute
		httproute, err := gatewayClient.HTTPRoutes(namespace).Get(ctx, gray.EntryHTTPRoute, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if httproute.Spec.Hostnames != nil && len(httproute.Spec.Hostnames) > 0 {
			hostname = string(httproute.Spec.Hostnames[0])
		}
	} else {
		err := gatewayClient.HTTPRoutes(namespace).Delete(ctx, k8sApp+"-"+component.K8sComponentName, metav1.DeleteOptions{})
		if err != nil && !k8serror.IsNotFound(err) {
			return err
		}

		flowEntryRule := [][]apimodel.FlowEntryRule{
			{
				{
					HeaderKey:   "Gray",
					HeaderType:  "Exact",
					HeaderValue: "true",
				},
			},
		}
		flowEntryRuleByte, err := json.Marshal(flowEntryRule)
		if err != nil {
			return err
		}
		gray.FlowEntryRule = string(flowEntryRuleByte)
		name, err := CreateHttproute(k8sApp, namespace, gray.AppID, service, component, gatewayClient)
		if err != nil {
			return err
		}
		httpRouteName = name
	}
	labels["hostname"] = hostname
	var serviceName string
	if service != nil && len(service) > 0 {
		serviceName = service[0].K8sServiceName
	}
	spec, err := HandleRolloutSpec(gray, serviceName, httpRouteName, name)
	if err != nil {
		return err
	}

	rollout := &v1alpha1.Rollout{
		TypeMeta: metav1.TypeMeta{
			Kind:       apimodel.Rollout,
			APIVersion: apimodel.APIVersionRollout,
		},
		ObjectMeta: metav1.ObjectMeta{
			Annotations: annotations,
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
		},
		Spec: spec,
	}
	rollout, err = kruiseClient.RolloutsV1alpha1().Rollouts(namespace).Create(ctx, rollout, metav1.CreateOptions{})
	if err != nil && !k8serror.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func HandleRolloutSpec(gray *dbmodel.AppGrayRelease, serviceName, httpRouteName, deployName string) (v1alpha1.RolloutSpec, error) {
	var matches []v1alpha2.HTTPRouteMatch
	var flowEntryRule [][]apimodel.FlowEntryRule
	err := json.Unmarshal([]byte(gray.FlowEntryRule), &flowEntryRule)
	if err != nil {
		return v1alpha1.RolloutSpec{}, err
	}
	for _, rules := range flowEntryRule {
		var headers []v1alpha2.HTTPHeaderMatch
		for _, header := range rules {
			headerType := v1alpha2.HeaderMatchType(header.HeaderType)
			headers = append(headers, v1alpha2.HTTPHeaderMatch{
				//Name:  v1beta1.HTTPRouteInterface(header.HeaderKey),
				Type:  &headerType,
				Value: header.HeaderValue,
			})
		}
		httpRouteMatch := v1alpha2.HTTPRouteMatch{Headers: headers}
		matches = append(matches, httpRouteMatch)
	}
	var grayStrategy []int32
	err = json.Unmarshal([]byte(gray.GrayStrategy), &grayStrategy)
	if err != nil {
		return v1alpha1.RolloutSpec{}, err
	}
	var steps []v1alpha1.CanaryStep
	for _, step := range grayStrategy {
		weight := step
		// TODO: matches
		steps = append(steps, v1alpha1.CanaryStep{
			Weight: &weight,
			//Matches: matches,
		})
	}
	var trafficRoutings []*v1alpha1.TrafficRouting
	if serviceName != "" && httpRouteName != "" {
		routeName := httpRouteName
		trafficRoutings = append(trafficRoutings, &v1alpha1.TrafficRouting{
			Service: serviceName,
			Gateway: &v1alpha1.GatewayTrafficRouting{HTTPRouteName: &routeName},
		})
	}
	return v1alpha1.RolloutSpec{
		ObjectRef: v1alpha1.ObjectRef{
			WorkloadRef: &v1alpha1.WorkloadRef{
				APIVersion: apimodel.APIVersionDeployment,
				Kind:       apimodel.Deployment,
				Name:       deployName,
			},
		},
		Strategy: v1alpha1.RolloutStrategy{
			Canary: &v1alpha1.CanaryStrategy{
				Steps:           steps,
				TrafficRoutings: trafficRoutings,
			},
		},
	}, nil
}
