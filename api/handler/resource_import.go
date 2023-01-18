package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	rainbondutil "github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/util/constants"
	"github.com/sirupsen/logrus"
	"github.com/twinj/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"time"
)

//ResourceImport Import the converted k8s resources into recognition
func (c *clusterAction) ResourceImport(namespace string, as map[string]model.ApplicationResource, eid string) (*model.ReturnResourceImport, *util.APIHandleError) {
	logrus.Infof("ResourceImport function begin")
	var returnResourceImport model.ReturnResourceImport
	tenant, err := c.createTenant(eid, namespace)
	returnResourceImport.Tenant = tenant
	if err != nil {
		logrus.Errorf("%v", err)
		return nil, &util.APIHandleError{Code: 400, Err: fmt.Errorf("create tenant error:%v", err)}
	}
	for appName, components := range as {
		app, err := c.createApp(eid, appName, tenant.UUID)
		if err != nil {
			logrus.Errorf("create app:%v err:%v", appName, err)
			return nil, &util.APIHandleError{Code: 400, Err: fmt.Errorf("create app:%v error:%v", appName, err)}
		}
		k8sResource, err := c.CreateK8sResource(components.KubernetesResources, app.AppID)
		if err != nil {
			logrus.Errorf("create K8sResources err:%v", err)
			return nil, &util.APIHandleError{Code: 400, Err: fmt.Errorf("create K8sResources err:%v", err)}
		}
		var componentAttributes []model.ComponentAttributes
		for _, componentResource := range components.ConvertResource {
			component, err := c.CreateComponent(app, tenant.UUID, componentResource, namespace, false)
			if err != nil {
				logrus.Errorf("%v", err)
				return nil, &util.APIHandleError{Code: 400, Err: fmt.Errorf("create app error:%v", err)}
			}
			c.createENV(componentResource.ENVManagement, component)
			c.createConfig(componentResource.ConfigManagement, component)
			c.createPort(componentResource.PortManagement, component)
			componentResource.TelescopicManagement.RuleID = c.createTelescopic(componentResource.TelescopicManagement, component)
			componentResource.HealthyCheckManagement.ProbeID = c.createHealthyCheck(componentResource.HealthyCheckManagement, component)
			c.createK8sAttributes(componentResource.ComponentK8sAttributesManagement, tenant.UUID, component)
			componentAttributes = append(componentAttributes, model.ComponentAttributes{
				TS:                     component,
				Image:                  componentResource.BasicManagement.Image,
				Cmd:                    componentResource.BasicManagement.Cmd,
				ENV:                    componentResource.ENVManagement,
				Config:                 componentResource.ConfigManagement,
				Port:                   componentResource.PortManagement,
				Telescopic:             componentResource.TelescopicManagement,
				HealthyCheck:           componentResource.HealthyCheckManagement,
				ComponentK8sAttributes: componentResource.ComponentK8sAttributesManagement,
			})
		}
		application := model.AppComponent{
			App:          app,
			Component:    componentAttributes,
			K8sResources: k8sResource,
		}
		returnResourceImport.App = append(returnResourceImport.App, application)
	}
	if err != nil {
		return nil, &util.APIHandleError{Code: 400, Err: fmt.Errorf("resource import error:%v", err)}
	}
	logrus.Infof("ResourceImport function end")
	return &returnResourceImport, nil
}

func (c *clusterAction) createTenant(eid string, namespace string) (*dbmodel.Tenants, error) {
	logrus.Infof("begin create tenant")
	var dbts dbmodel.Tenants
	id, name, errN := GetServiceManager().CreateTenandIDAndName(eid)
	if errN != nil {
		return nil, errN
	}
	dbts.EID = eid
	dbts.Namespace = namespace
	dbts.Name = name
	dbts.UUID = id
	dbts.LimitMemory = 0
	tenant, _ := db.GetManager().TenantDao().GetTenantIDByName(dbts.Name)
	if tenant != nil {
		logrus.Warningf("tenant %v already exists", dbts.Name)
		return tenant, nil
	}
	if err := db.GetManager().TenantDao().AddModel(&dbts); err != nil {
		if !strings.HasSuffix(err.Error(), "is exist") {
			return nil, err
		}
	}
	ns, err := c.clientset.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if err != nil {
		return nil, &util.APIHandleError{Code: 404, Err: fmt.Errorf("failed to get namespace %v:%v", namespace, err)}
	}
	if ns.Labels == nil {
		ns.Labels = make(map[string]string)
	}
	ns.Labels[constants.ResourceManagedByLabel] = constants.Rainbond
	_, err = c.clientset.CoreV1().Namespaces().Update(context.Background(), ns, metav1.UpdateOptions{})
	if err != nil {
		return nil, &util.APIHandleError{Code: 404, Err: fmt.Errorf("failed to add label to namespace %v:%v", namespace, err)}
	}
	logrus.Infof("end create tenant")
	return &dbts, nil
}

func (c *clusterAction) createApp(eid string, app string, tenantID string) (*dbmodel.Application, error) {
	appID := rainbondutil.NewUUID()
	application, _ := db.GetManager().ApplicationDao().GetAppByName(tenantID, app)
	if application != nil {
		logrus.Infof("app %v already exists", app)
		return application, nil
	}
	appReq := &dbmodel.Application{
		EID:            eid,
		TenantID:       tenantID,
		AppID:          appID,
		AppName:        app,
		AppType:        "rainbond",
		GovernanceMode: dbmodel.GovernanceModeKubernetesNativeService,
		K8sApp:         app,
	}
	if err := db.GetManager().ApplicationDao().AddModel(appReq); err != nil {
		return appReq, err
	}
	return appReq, nil
}

func (c *clusterAction) CreateK8sResource(k8sResources []dbmodel.K8sResource, AppID string) ([]dbmodel.K8sResource, error) {
	var k8sResourceList []*dbmodel.K8sResource
	for _, k8sResource := range k8sResources {
		k8sResource.AppID = AppID
		kr := k8sResource
		k8sResourceList = append(k8sResourceList, &kr)
	}
	err := db.GetManager().K8sResourceDao().CreateK8sResource(k8sResourceList)
	return k8sResources, err
}

func (c *clusterAction) CreateComponent(app *dbmodel.Application, tenantID string, component model.ConvertResource, namespace string, isYaml bool) (*dbmodel.TenantServices, error) {
	var extendMethod string
	switch component.BasicManagement.ResourceType {
	case model.Deployment:
		extendMethod = string(dbmodel.ServiceTypeStatelessMultiple)
	case model.Job:
		extendMethod = string(dbmodel.ServiceTypeJob)
	case model.CronJob:
		extendMethod = string(dbmodel.ServiceTypeCronJob)
	case model.StateFulSet:
		extendMethod = string(dbmodel.ServiceTypeStateMultiple)
	}
	serviceID := rainbondutil.NewUUID()
	serviceAlias := "gr" + serviceID[len(serviceID)-6:]
	replicas := 1
	if component.BasicManagement.Replicas != nil {
		replicas = int(*component.BasicManagement.Replicas)
	}
	JobStrategy, err := json.Marshal(component.BasicManagement.JobStrategy)
	if err != nil {
		logrus.Errorf("component %v BasicManagement.JobStrategy json error%v", component.ComponentsName, err)
	}
	ts := dbmodel.TenantServices{
		TenantID:         tenantID,
		ServiceID:        serviceID,
		ServiceAlias:     serviceAlias,
		ServiceName:      serviceAlias,
		ServiceType:      "application",
		Comment:          "docker run application",
		ContainerCPU:     int(component.BasicManagement.CPU),
		ContainerMemory:  int(component.BasicManagement.Memory),
		ContainerGPU:     0,
		UpgradeMethod:    "Rolling",
		ExtendMethod:     extendMethod,
		Replicas:         replicas,
		DeployVersion:    time.Now().Format("20060102150405"),
		Category:         "app_publish",
		CurStatus:        "undeploy",
		Status:           0,
		Namespace:        namespace,
		UpdateTime:       time.Now(),
		Kind:             "internal",
		AppID:            app.AppID,
		K8sComponentName: component.ComponentsName,
		JobStrategy:      string(JobStrategy),
	}
	if err := db.GetManager().TenantServiceDao().AddModel(&ts); err != nil {
		logrus.Errorf("add service error, %v", err)
		return nil, err
	}
	if !isYaml {
		changeLabel := func(label map[string]string) map[string]string {
			label[constants.ResourceManagedByLabel] = constants.Rainbond
			label["service_id"] = serviceID
			label["version"] = ts.DeployVersion
			label["creater_id"] = string(rainbondutil.NewTimeVersion())
			label["migrator"] = "rainbond"
			label["creator"] = "Rainbond"
			return label
		}
		switch component.BasicManagement.ResourceType {
		case model.Deployment:
			dm, err := c.clientset.AppsV1().Deployments(namespace).Get(context.Background(), component.ComponentsName, metav1.GetOptions{})
			if err != nil {
				logrus.Errorf("failed to get %v Deployments %v:%v", namespace, component.ComponentsName, err)
				return nil, &util.APIHandleError{Code: 404, Err: fmt.Errorf("failed to get Deployments %v:%v", namespace, err)}
			}
			if dm.Labels == nil {
				dm.Labels = make(map[string]string)
			}
			if dm.Spec.Template.Labels == nil {
				dm.Spec.Template.Labels = make(map[string]string)
			}
			dm.Labels = changeLabel(dm.Labels)
			dm.Spec.Template.Labels = changeLabel(dm.Spec.Template.Labels)
			_, err = c.clientset.AppsV1().Deployments(namespace).Update(context.Background(), dm, metav1.UpdateOptions{})
			if err != nil {
				logrus.Errorf("failed to update Deployments %v:%v", namespace, err)
				return nil, &util.APIHandleError{Code: 404, Err: fmt.Errorf("failed to update Deployments %v:%v", namespace, err)}
			}
		case model.Job:
			job, err := c.clientset.BatchV1().Jobs(namespace).Get(context.Background(), component.ComponentsName, metav1.GetOptions{})
			if err != nil {
				logrus.Errorf("failed to get %v Jobs %v:%v", namespace, component.ComponentsName, err)
				return nil, &util.APIHandleError{Code: 404, Err: fmt.Errorf("failed to get Jobs %v:%v", namespace, err)}
			}
			if job.Labels == nil {
				job.Labels = make(map[string]string)
			}
			job.Labels = changeLabel(job.Labels)
			_, err = c.clientset.BatchV1().Jobs(namespace).Update(context.Background(), job, metav1.UpdateOptions{})
			if err != nil {
				logrus.Errorf("failed to update StatefulSets %v:%v", namespace, err)
				return nil, &util.APIHandleError{Code: 404, Err: fmt.Errorf("failed to update StatefulSets %v:%v", namespace, err)}
			}
		case model.CronJob:
			cr, err := c.clientset.BatchV1beta1().CronJobs(namespace).Get(context.Background(), component.ComponentsName, metav1.GetOptions{})
			if err != nil {
				logrus.Errorf("failed to get %v CronJob %v:%v", namespace, component.ComponentsName, err)
				return nil, &util.APIHandleError{Code: 404, Err: fmt.Errorf("failed to get CronJob %v:%v", namespace, err)}
			}
			if cr.Labels == nil {
				cr.Labels = make(map[string]string)
			}
			cr.Labels = changeLabel(cr.Labels)
			if cr.Spec.JobTemplate.Labels == nil {
				cr.Spec.JobTemplate.Labels = make(map[string]string)
			}
			cr.Spec.JobTemplate.Labels = changeLabel(cr.Spec.JobTemplate.Labels)
			_, err = c.clientset.BatchV1beta1().CronJobs(namespace).Update(context.Background(), cr, metav1.UpdateOptions{})
			if err != nil {
				logrus.Errorf("failed to update CronJobs %v:%v", namespace, err)
				return nil, &util.APIHandleError{Code: 404, Err: fmt.Errorf("failed to update CronJobs %v:%v", namespace, err)}
			}
		case model.StateFulSet:
			sts, err := c.clientset.AppsV1().StatefulSets(namespace).Get(context.Background(), component.ComponentsName, metav1.GetOptions{})
			if err != nil {
				logrus.Errorf("failed to get %v StatefulSets %v:%v", namespace, component.ComponentsName, err)
				return nil, &util.APIHandleError{Code: 404, Err: fmt.Errorf("failed to get StatefulSets %v:%v", namespace, err)}
			}
			if sts.Labels == nil {
				sts.Labels = make(map[string]string)
			}
			sts.Labels = changeLabel(sts.Labels)
			if sts.Spec.Template.Labels == nil {
				sts.Spec.Template.Labels = make(map[string]string)
			}
			sts.Spec.Template.Labels = changeLabel(sts.Spec.Template.Labels)
			_, err = c.clientset.AppsV1().StatefulSets(namespace).Update(context.Background(), sts, metav1.UpdateOptions{})
			if err != nil {
				logrus.Errorf("failed to update StatefulSets %v:%v", namespace, err)
				return nil, &util.APIHandleError{Code: 404, Err: fmt.Errorf("failed to update StatefulSets %v:%v", namespace, err)}
			}
		}
	}
	return &ts, nil

}

func (c *clusterAction) createENV(envs []model.ENVManagement, service *dbmodel.TenantServices) {
	var envVar []*dbmodel.TenantServiceEnvVar
	for _, env := range envs {
		var envD dbmodel.TenantServiceEnvVar
		envD.AttrName = env.ENVKey
		envD.AttrValue = env.ENVValue
		envD.TenantID = service.TenantID
		envD.ServiceID = service.ServiceID
		envD.ContainerPort = 0
		envD.IsChange = true
		envD.Name = env.ENVExplain
		envD.Scope = "inner"
		envVar = append(envVar, &envD)
	}
	if err := db.GetManager().TenantServiceEnvVarDao().CreateOrUpdateEnvsInBatch(envVar); err != nil {
		logrus.Errorf("%v Environment variable creation failed:%v", service.ServiceAlias, err)
	}
}

func (c *clusterAction) createConfig(configs []model.ConfigManagement, service *dbmodel.TenantServices) {
	var configVar []*dbmodel.TenantServiceVolume
	var configFiles []*dbmodel.TenantServiceConfigFile
	for _, config := range configs {
		tsv := &dbmodel.TenantServiceVolume{
			ServiceID:          service.ServiceID,
			VolumeName:         config.ConfigName,
			VolumePath:         config.ConfigPath,
			VolumeType:         "config-file",
			Category:           "",
			VolumeProviderName: "",
			IsReadOnly:         false,
			VolumeCapacity:     0,
			AccessMode:         "RWX",
			SharePolicy:        "exclusive",
			BackupPolicy:       "exclusive",
			ReclaimPolicy:      "exclusive",
			AllowExpansion:     false,
			Mode:               &config.Mode,
		}
		configVar = append(configVar, tsv)
		configfile := &dbmodel.TenantServiceConfigFile{
			ServiceID:   service.ServiceID,
			VolumeName:  config.ConfigName,
			FileContent: config.ConfigValue,
		}
		configFiles = append(configFiles, configfile)
	}
	err := db.GetManager().TenantServiceVolumeDao().CreateOrUpdateVolumesInBatch(configVar)
	if err != nil {
		logrus.Errorf("TenantServiceVolume %v configuration file creation failed:%v", service.ServiceAlias, err)
	}

	err = db.GetManager().TenantServiceConfigFileDao().CreateOrUpdateConfigFilesInBatch(configFiles)
	if err != nil {
		logrus.Errorf("TenantServiceConfigFile %v configuration file creation failed:%v", service.ServiceAlias, err)
	}
}

func (c *clusterAction) createPort(ports []model.PortManagement, service *dbmodel.TenantServices) {
	var portVar []*dbmodel.TenantServicesPort
	for _, port := range ports {
		portAlias := strings.Replace(service.ServiceAlias, "-", "_", -1)
		var vpD dbmodel.TenantServicesPort
		vpD.ServiceID = service.ServiceID
		vpD.TenantID = service.TenantID
		vpD.IsInnerService = &port.Inner
		vpD.IsOuterService = &port.Outer
		vpD.ContainerPort = int(port.Port)
		vpD.MappingPort = int(port.Port)
		vpD.Name = port.Name
		vpD.Protocol = port.Protocol
		vpD.PortAlias = fmt.Sprintf("%v%v", strings.ToUpper(portAlias), port.Port)
		vpD.K8sServiceName = service.ServiceAlias
		portVar = append(portVar, &vpD)
	}
	if err := db.GetManager().TenantServicesPortDao().CreateOrUpdatePortsInBatch(portVar); err != nil {
		logrus.Errorf("%v port creation failed:%v", service.ServiceAlias, err)
	}
}

func (c *clusterAction) createTelescopic(telescopic model.TelescopicManagement, service *dbmodel.TenantServices) string {
	if !telescopic.Enable {
		return ""
	}
	r := &dbmodel.TenantServiceAutoscalerRules{
		RuleID:      rainbondutil.NewUUID(),
		ServiceID:   service.ServiceID,
		Enable:      true,
		XPAType:     "hpa",
		MinReplicas: int(telescopic.MinReplicas),
		MaxReplicas: int(telescopic.MaxReplicas),
	}
	telescopic.RuleID = r.RuleID
	if err := db.GetManager().TenantServceAutoscalerRulesDao().AddModel(r); err != nil {
		logrus.Errorf("%v TenantServiceAutoscalerRules creation failed:%v", service.ServiceAlias, err)
		return ""
	}
	for _, metric := range telescopic.CPUOrMemory {
		m := &dbmodel.TenantServiceAutoscalerRuleMetrics{
			RuleID:            r.RuleID,
			MetricsType:       metric.MetricsType,
			MetricsName:       metric.MetricsName,
			MetricTargetType:  metric.MetricTargetType,
			MetricTargetValue: metric.MetricTargetValue,
		}
		if err := db.GetManager().TenantServceAutoscalerRuleMetricsDao().AddModel(m); err != nil {
			logrus.Errorf("%v TenantServceAutoscalerRuleMetricsDao creation failed:%v", service.ServiceAlias, err)
		}
	}
	return r.RuleID
}

func (c *clusterAction) createHealthyCheck(telescopic model.HealthyCheckManagement, service *dbmodel.TenantServices) string {
	if telescopic.Status == 0 {
		return ""
	}
	var tspD dbmodel.TenantServiceProbe
	tspD.ServiceID = service.ServiceID
	tspD.Cmd = telescopic.Command
	tspD.FailureThreshold = telescopic.FailureThreshold
	tspD.HTTPHeader = telescopic.HTTPHeader
	tspD.InitialDelaySecond = telescopic.InitialDelaySecond
	tspD.IsUsed = &telescopic.Status
	tspD.Mode = telescopic.Mode
	tspD.Path = telescopic.Path
	tspD.PeriodSecond = telescopic.PeriodSecond
	tspD.Port = telescopic.Port
	tspD.ProbeID = strings.Replace(uuid.NewV4().String(), "-", "", -1)
	tspD.Scheme = telescopic.DetectionMethod
	tspD.SuccessThreshold = telescopic.SuccessThreshold
	tspD.TimeoutSecond = telescopic.TimeoutSecond
	tspD.FailureAction = ""
	if err := GetServiceManager().ServiceProbe(&tspD, "add"); err != nil {
		logrus.Errorf("%v createHealthyCheck creation failed:%v", service.ServiceAlias, err)
	}
	return tspD.ProbeID
}

func (c *clusterAction) createK8sAttributes(specials []*dbmodel.ComponentK8sAttributes, tenantID string, component *dbmodel.TenantServices) {
	for _, specials := range specials {
		specials.TenantID = tenantID
		specials.ComponentID = component.ServiceID
	}
	err := db.GetManager().ComponentK8sAttributeDao().CreateOrUpdateAttributesInBatch(specials)
	if err != nil {
		logrus.Errorf("%v createSpecial creation failed:%v", component.ServiceAlias, err)
	}
}
