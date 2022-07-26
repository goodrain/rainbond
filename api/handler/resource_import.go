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
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
	"github.com/twinj/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
	"strings"
	"time"
)

//ResourceImport Import the converted k8s resources into recognition
func (c *clusterAction) ResourceImport(ctx context.Context, namespace string, as map[string]model.ApplicationResource, eid string) (*model.ReturnResourceImport, *util.APIHandleError) {
	logrus.Infof("ResourceImport function begin")
	var returnResourceImport model.ReturnResourceImport
	err := db.GetManager().DB().Transaction(func(tx *gorm.DB) error {
		tenant, err := c.createTenant(context.Background(), eid, namespace, tx)
		returnResourceImport.Tenant = tenant
		if err != nil {
			logrus.Errorf("%v", err)
			return &util.APIHandleError{Code: 400, Err: fmt.Errorf("create tenant error:%v", err)}
		}
		for appName, components := range as {
			app, err := c.createApp(eid, tx, appName, tenant.UUID)
			if err != nil {
				logrus.Errorf("create app:%v err:%v", appName, err)
				return &util.APIHandleError{Code: 400, Err: fmt.Errorf("create app:%v error:%v", appName, err)}
			}
			k8sResource, err := c.createK8sResource(tx, components.KubernetesResources, app.AppID)
			if err != nil {
				logrus.Errorf("create K8sResources err:%v", err)
				return &util.APIHandleError{Code: 400, Err: fmt.Errorf("create K8sResources err:%v", err)}
			}
			var ca []model.ComponentAttributes
			for _, componentResource := range components.ConvertResource {
				component, err := c.createComponent(context.Background(), app, tenant.UUID, componentResource, namespace)
				if err != nil {
					logrus.Errorf("%v", err)
					return &util.APIHandleError{Code: 400, Err: fmt.Errorf("create app error:%v", err)}
				}
				c.createENV(componentResource.ENVManagement, component)
				c.createConfig(componentResource.ConfigManagement, component)
				c.createPort(componentResource.PortManagement, component)
				componentResource.TelescopicManagement.RuleID = c.createTelescopic(componentResource.TelescopicManagement, component)
				componentResource.HealthyCheckManagement.ProbeID = c.createHealthyCheck(componentResource.HealthyCheckManagement, component)
				c.createK8sAttributes(componentResource.ComponentK8sAttributesManagement, tenant.UUID, component)
				ca = append(ca, model.ComponentAttributes{
					Ct:                     component,
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
				Component:    ca,
				K8sResources: k8sResource,
			}
			returnResourceImport.App = append(returnResourceImport.App, application)
		}
		return nil
	})
	if err != nil {
		return nil, &util.APIHandleError{Code: 400, Err: fmt.Errorf("resource import error:%v", err)}
	}
	logrus.Infof("ResourceImport function end")
	return &returnResourceImport, nil
}

func (c *clusterAction) createTenant(ctx context.Context, eid string, namespace string, tx *gorm.DB) (*dbmodel.Tenants, error) {
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
	if err := db.GetManager().TenantDaoTransactions(tx).AddModel(&dbts); err != nil {
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

func (c *clusterAction) createApp(eid string, tx *gorm.DB, app string, tenantID string) (*dbmodel.Application, error) {
	appID := rainbondutil.NewUUID()
	application, _ := db.GetManager().ApplicationDaoTransactions(tx).GetAppByName(tenantID, app)
	if application != nil {
		logrus.Infof("app %v already exists", app)
		return application, nil
	}
	appReq := &dbmodel.Application{
		EID:             eid,
		TenantID:        tenantID,
		AppID:           appID,
		AppName:         app,
		AppType:         "rainbond",
		AppStoreName:    "",
		AppStoreURL:     "",
		AppTemplateName: "",
		Version:         "",
		GovernanceMode:  dbmodel.GovernanceModeKubernetesNativeService,
		K8sApp:          app,
	}
	if err := db.GetManager().ApplicationDaoTransactions(tx).AddModel(appReq); err != nil {
		return appReq, err
	}
	return appReq, nil
}

func (c *clusterAction) createK8sResource(tx *gorm.DB, k8sResources []dbmodel.K8sResource, AppID string) ([]dbmodel.K8sResource, error) {
	var k8sResourceList []*dbmodel.K8sResource
	for _, k8sResource := range k8sResources {
		k8sResource.AppID = AppID
		kr := k8sResource
		k8sResourceList = append(k8sResourceList, &kr)
	}
	err := db.GetManager().K8sResourceDaoTransactions(tx).CreateK8sResourceInBatch(k8sResourceList)
	return k8sResources, err
}

func (c *clusterAction) createComponent(ctx context.Context, app *dbmodel.Application, tenantID string, component model.ConvertResource, namespace string) (*dbmodel.TenantServices, error) {
	serviceID := rainbondutil.NewUUID()
	serviceAlias := "gr" + serviceID[len(serviceID)-6:]
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
		ExtendMethod:     string(dbmodel.ServiceTypeStatelessMultiple),
		Replicas:         int(component.BasicManagement.Replicas),
		DeployVersion:    time.Now().Format("20060102150405"),
		Category:         "app_publish",
		CurStatus:        "undeploy",
		Status:           0,
		Namespace:        namespace,
		UpdateTime:       time.Now(),
		Kind:             "internal",
		AppID:            app.AppID,
		K8sComponentName: component.ComponentsName,
	}
	if err := db.GetManager().TenantServiceDao().AddModel(&ts); err != nil {
		logrus.Errorf("add service error, %v", err)
		return nil, err
	}
	dm, err := c.clientset.AppsV1().Deployments(namespace).Get(context.Background(), component.ComponentsName, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("failed to get %v deployment %v:%v", namespace, component.ComponentsName, err)
		return nil, &util.APIHandleError{Code: 404, Err: fmt.Errorf("failed to get deployment %v:%v", namespace, err)}
	}
	if dm.Labels == nil {
		dm.Labels = make(map[string]string)
	}
	dm.Labels[constants.ResourceManagedByLabel] = constants.Rainbond
	dm.Labels["service_id"] = serviceID
	dm.Labels["version"] = ts.DeployVersion
	dm.Labels["creater_id"] = string(rainbondutil.NewTimeVersion())
	dm.Labels["migrator"] = "rainbond"
	dm.Spec.Template.Labels["service_id"] = serviceID
	dm.Spec.Template.Labels["version"] = ts.DeployVersion
	dm.Spec.Template.Labels["creater_id"] = string(rainbondutil.NewTimeVersion())
	dm.Spec.Template.Labels["migrator"] = "rainbond"
	_, err = c.clientset.AppsV1().Deployments(namespace).Update(context.Background(), dm, metav1.UpdateOptions{})
	if err != nil {
		logrus.Errorf("failed to update deployment %v:%v", namespace, err)
		return nil, &util.APIHandleError{Code: 404, Err: fmt.Errorf("failed to update deployment %v:%v", namespace, err)}
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
	}
	err := db.GetManager().TenantServiceVolumeDao().CreateOrUpdateVolumesInBatch(configVar)
	if err != nil {
		logrus.Errorf("%v configuration file creation failed:%v", service.ServiceAlias, err)
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
		vpD.Protocol = port.Protocol
		vpD.PortAlias = fmt.Sprintf("%v%v", strings.ToUpper(portAlias), port.Port)
		vpD.K8sServiceName = fmt.Sprintf("%v-%v", service.ServiceAlias, port.Port)
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

//ObjectToJSONORYaml changeType true is json / yaml
func ObjectToJSONORYaml(changeType string, data interface{}) (string, error) {
	if data == nil {
		return "", nil
	}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("json serialization failed err:%v", err)
	}
	if changeType == "json" {
		return string(dataJSON), nil
	}
	dataYaml, err := yaml.JSONToYAML(dataJSON)
	if err != nil {
		return "", fmt.Errorf("yaml serialization failed err:%v", err)
	}
	return string(dataYaml), nil
}
