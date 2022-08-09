package handler

import (
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//GetNamespaceSource Get all resources in the current namespace
func (c *clusterAction) GetNamespaceSource(ctx context.Context, content string, namespace string) (map[string]model.LabelResource, *util.APIHandleError) {
	logrus.Infof("GetNamespaceSource function begin")
	//存储workloads们的ConfigMap
	cmsMap := make(map[string][]string)
	//存储workloads们的secrets
	secretsMap := make(map[string][]string)
	deployments, cmMap, secretMap := c.getResourceName(context.Background(), namespace, content, model.Deployment)
	if len(cmsMap) != 0 {
		cmsMap = MergeMap(cmMap, cmsMap)
	}
	if len(secretMap) != 0 {
		secretsMap = MergeMap(secretMap, secretsMap)
	}
	jobs, cmMap, secretMap := c.getResourceName(ctx, namespace, content, model.Job)
	if len(cmsMap) != 0 {
		cmsMap = MergeMap(cmMap, cmsMap)
	}
	if len(secretMap) != 0 {
		secretsMap = MergeMap(secretMap, secretsMap)
	}
	cronJobs, cmMap, secretMap := c.getResourceName(ctx, namespace, content, model.CronJob)
	if len(cmsMap) != 0 {
		cmsMap = MergeMap(cmMap, cmsMap)
	}
	if len(secretMap) != 0 {
		secretsMap = MergeMap(secretMap, secretsMap)
	}
	stateFulSets, cmMap, secretMap := c.getResourceName(ctx, namespace, content, model.StateFulSet)
	if len(cmsMap) != 0 {
		cmsMap = MergeMap(cmMap, cmsMap)
	}
	if len(secretMap) != 0 {
		secretsMap = MergeMap(secretMap, secretsMap)
	}
	processWorkloads := model.LabelWorkloadsResourceProcess{
		Deployments:  deployments,
		Jobs:         jobs,
		CronJobs:     cronJobs,
		StateFulSets: stateFulSets,
	}
	services, _, _ := c.getResourceName(ctx, namespace, content, model.Service)
	pvc, _, _ := c.getResourceName(ctx, namespace, content, model.PVC)
	ingresses, _, _ := c.getResourceName(ctx, namespace, content, model.Ingress)
	networkPolicies, _, _ := c.getResourceName(ctx, namespace, content, model.NetworkPolicy)
	cms, _, _ := c.getResourceName(ctx, namespace, content, model.ConfigMap)
	secrets, _, _ := c.getResourceName(ctx, namespace, content, model.Secret)
	serviceAccounts, _, _ := c.getResourceName(ctx, namespace, content, model.ServiceAccount)
	roleBindings, _, _ := c.getResourceName(ctx, namespace, content, model.RoleBinding)
	horizontalPodAutoscalers, _, _ := c.getResourceName(ctx, namespace, content, model.HorizontalPodAutoscaler)
	roles, _, _ := c.getResourceName(ctx, namespace, content, model.Role)
	processOthers := model.LabelOthersResourceProcess{
		Services:                 services,
		PVC:                      pvc,
		Ingresses:                ingresses,
		NetworkPolicies:          networkPolicies,
		ConfigMaps:               MergeMap(cmsMap, cms),
		Secrets:                  MergeMap(secretsMap, secrets),
		ServiceAccounts:          serviceAccounts,
		RoleBindings:             roleBindings,
		HorizontalPodAutoscalers: horizontalPodAutoscalers,
		Roles:                    roles,
	}
	labelResource := resourceProcessing(processWorkloads, processOthers)
	logrus.Infof("GetNamespaceSource function end")
	return labelResource, nil
}

//resourceProcessing 将处理好的资源类型数据格式再加工成可作为返回值的数据。
func resourceProcessing(processWorkloads model.LabelWorkloadsResourceProcess, processOthers model.LabelOthersResourceProcess) map[string]model.LabelResource {
	labelResource := make(map[string]model.LabelResource)
	for label, deployments := range processWorkloads.Deployments {
		if val, ok := labelResource[label]; ok {
			val.Workloads.Deployments = deployments
			labelResource[label] = val
			continue
		}
		labelResource[label] = model.LabelResource{
			Workloads: model.WorkLoadsResource{
				Deployments: deployments,
			},
		}
	}
	for label, jobs := range processWorkloads.Jobs {
		if val, ok := labelResource[label]; ok {
			val.Workloads.Jobs = jobs
			labelResource[label] = val
			continue
		}
		labelResource[label] = model.LabelResource{
			Workloads: model.WorkLoadsResource{
				Jobs: jobs,
			},
		}

	}
	for label, cronJobs := range processWorkloads.CronJobs {
		if val, ok := labelResource[label]; ok {
			val.Workloads.CronJobs = cronJobs
			labelResource[label] = val
			continue
		}
		labelResource[label] = model.LabelResource{
			Workloads: model.WorkLoadsResource{
				CronJobs: cronJobs,
			},
		}
	}
	for label, stateFulSets := range processWorkloads.StateFulSets {
		if val, ok := labelResource[label]; ok {
			val.Workloads.StateFulSets = stateFulSets
			labelResource[label] = val
			continue
		}
		labelResource[label] = model.LabelResource{
			Workloads: model.WorkLoadsResource{
				StateFulSets: stateFulSets,
			},
		}
	}
	for label, service := range processOthers.Services {
		if val, ok := labelResource[label]; ok {
			val.Others.Services = service
			labelResource[label] = val
			continue
		}
		labelResource[label] = model.LabelResource{
			Others: model.OtherResource{
				Services: service,
			},
		}

	}
	for label, pvc := range processOthers.PVC {
		if val, ok := labelResource[label]; ok {
			val.Others.PVC = pvc
			labelResource[label] = val
			continue
		}
		labelResource[label] = model.LabelResource{
			Others: model.OtherResource{
				PVC: pvc,
			},
		}

	}
	for label, ingresses := range processOthers.Ingresses {
		if val, ok := labelResource[label]; ok {
			val.Others.Ingresses = ingresses
			labelResource[label] = val
			continue
		}
		labelResource[label] = model.LabelResource{
			Others: model.OtherResource{
				Ingresses: ingresses,
			},
		}
	}
	for label, networkPolicies := range processOthers.NetworkPolicies {
		if val, ok := labelResource[label]; ok {
			val.Others.NetworkPolicies = networkPolicies
			labelResource[label] = val
			continue
		}
		labelResource[label] = model.LabelResource{
			Others: model.OtherResource{
				NetworkPolicies: networkPolicies,
			},
		}
	}
	for label, configMaps := range processOthers.ConfigMaps {
		if val, ok := labelResource[label]; ok {
			val.Others.ConfigMaps = configMaps
			labelResource[label] = val
			continue
		}
		labelResource[label] = model.LabelResource{
			Others: model.OtherResource{
				ConfigMaps: configMaps,
			},
		}
	}
	for label, secrets := range processOthers.Secrets {
		if val, ok := labelResource[label]; ok {
			val.Others.Secrets = secrets
			labelResource[label] = val
			continue
		}
		labelResource[label] = model.LabelResource{
			Others: model.OtherResource{
				Secrets: secrets,
			},
		}
	}
	for label, serviceAccounts := range processOthers.ServiceAccounts {
		if val, ok := labelResource[label]; ok {
			val.Others.ServiceAccounts = serviceAccounts
			labelResource[label] = val
			continue
		}
		labelResource[label] = model.LabelResource{
			Others: model.OtherResource{
				ServiceAccounts: serviceAccounts,
			},
		}
	}
	for label, roleBindings := range processOthers.RoleBindings {
		if val, ok := labelResource[label]; ok {
			val.Others.RoleBindings = roleBindings
			labelResource[label] = val
			continue
		}
		labelResource[label] = model.LabelResource{
			Others: model.OtherResource{
				RoleBindings: roleBindings,
			},
		}
	}
	for label, horizontalPodAutoscalers := range processOthers.HorizontalPodAutoscalers {
		if val, ok := labelResource[label]; ok {
			val.Others.HorizontalPodAutoscalers = horizontalPodAutoscalers
			labelResource[label] = val
			continue
		}
		labelResource[label] = model.LabelResource{
			Others: model.OtherResource{
				HorizontalPodAutoscalers: horizontalPodAutoscalers,
			},
		}
	}
	for label, roles := range processOthers.Roles {
		if val, ok := labelResource[label]; ok {
			val.Others.Roles = roles
			labelResource[label] = val
			continue
		}
		labelResource[label] = model.LabelResource{
			Others: model.OtherResource{
				Roles: roles,
			},
		}
	}
	return labelResource
}

//Resource -
type Resource struct {
	ObjectMeta metav1.ObjectMeta
	Template   corev1.PodTemplateSpec
}

//getResourceName 将指定资源类型按照【label名】：[]{资源名...}处理后返回
func (c *clusterAction) getResourceName(ctx context.Context, namespace string, content string, resourcesType string) (map[string][]string, map[string][]string, map[string][]string) {
	resourceName := make(map[string][]string)
	var tempResources []*Resource
	isWorkloads := false
	cmMap := make(map[string][]string)
	secretMap := make(map[string][]string)
	switch resourcesType {
	case model.Deployment:
		resources, err := c.clientset.AppsV1().Deployments(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Deployment list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta, Template: dm.Spec.Template})
		}
		isWorkloads = true
	case model.Job:
		resources, err := c.clientset.BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Job list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			if dm.OwnerReferences != nil {
				if dm.OwnerReferences[0].Kind == model.CronJob {
					continue
				}
			}
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta, Template: dm.Spec.Template})
		}
		isWorkloads = true
	case model.CronJob:
		resources, err := c.clientset.BatchV1beta1().CronJobs(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get CronJob list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta, Template: dm.Spec.JobTemplate.Spec.Template})
		}
		isWorkloads = true
	case model.StateFulSet:
		resources, err := c.clientset.AppsV1().StatefulSets(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get StateFulSets list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta, Template: dm.Spec.Template})
		}
		isWorkloads = true
	case model.Service:
		resources, err := c.clientset.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Services list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta})
		}
	case model.PVC:
		resources, err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get PersistentVolumeClaims list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {

			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta})
		}
	case model.Ingress:
		resources, err := c.clientset.NetworkingV1().Ingresses(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Ingresses list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta})
		}
	case model.NetworkPolicy:
		resources, err := c.clientset.NetworkingV1().NetworkPolicies(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get NetworkPolicies list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta})
		}
	case model.ConfigMap:
		resources, err := c.clientset.CoreV1().ConfigMaps(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get ConfigMaps list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta})
		}
	case model.Secret:
		resources, err := c.clientset.CoreV1().Secrets(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Secrets list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta})
		}
	case model.ServiceAccount:
		resources, err := c.clientset.CoreV1().ServiceAccounts(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get ServiceAccounts list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta})
		}
	case model.RoleBinding:
		resources, err := c.clientset.RbacV1().RoleBindings(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get RoleBindings list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta})
		}
	case model.HorizontalPodAutoscaler:
		resources, err := c.clientset.AutoscalingV1().HorizontalPodAutoscalers(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get HorizontalPodAutoscalers list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, hpa := range resources.Items {
			rbdResource := false
			labels := make(map[string]string)
			switch hpa.Spec.ScaleTargetRef.Kind {
			case model.Deployment:
				deploy, err := c.clientset.AppsV1().Deployments(namespace).Get(context.Background(), hpa.Spec.ScaleTargetRef.Name, metav1.GetOptions{})
				if err != nil {
					logrus.Errorf("The bound deployment does not exist:%v", err)
				}
				if hpa.ObjectMeta.Labels["creator"] == "Rainbond" {
					rbdResource = true
				}
				labels = deploy.ObjectMeta.Labels
			case model.StateFulSet:
				ss, err := c.clientset.AppsV1().StatefulSets(namespace).Get(context.Background(), hpa.Spec.ScaleTargetRef.Name, metav1.GetOptions{})
				if err != nil {
					logrus.Errorf("The bound deployment does not exist:%v", err)
				}
				if hpa.ObjectMeta.Labels["creator"] == "Rainbond" {
					rbdResource = true
				}
				labels = ss.ObjectMeta.Labels
			}
			var app string
			if content == "unmanaged" && rbdResource {
				continue
			}
			app = labels["app"]
			if labels["app.kubernetes.io/name"] != "" {
				app = labels["app.kubernetes.io/name"]
			}
			if app == "" {
				app = "unclassified"
			}
			if _, ok := resourceName[app]; ok {
				resourceName[app] = append(resourceName[app], hpa.Name)
			} else {
				resourceName[app] = []string{hpa.Name}
			}
		}
		return resourceName, nil, nil
	case model.Role:
		resources, err := c.clientset.RbacV1().Roles(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Roles list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta})
		}
	}
	//这一块是统一处理资源，按label划分出来
	for _, rs := range tempResources {
		if content == "unmanaged" && rs.ObjectMeta.Labels["creator"] == "Rainbond" {
			continue
		}
		app := rs.ObjectMeta.Labels["app"]
		if rs.ObjectMeta.Labels["app.kubernetes.io/name"] != "" {
			app = rs.ObjectMeta.Labels["app.kubernetes.io/name"]
		}
		if app == "" {
			app = "unclassified"
		}
		//如果是Workloads类型的资源需要检查其内部configmap、secret、PVC（防止没有这三种资源没有label但是用到了）
		if isWorkloads {
			cmList, secretList := c.replenishLabel(ctx, rs, namespace, app)
			if _, ok := cmMap[app]; ok {
				cmMap[app] = append(cmMap[app], cmList...)
			} else {
				cmMap[app] = cmList
			}
			if _, ok := secretMap[app]; ok {
				secretMap[app] = append(secretMap[app], secretList...)
			} else {
				secretMap[app] = secretList
			}
		}
		if _, ok := resourceName[app]; ok {
			resourceName[app] = append(resourceName[app], rs.ObjectMeta.Name)
		} else {
			resourceName[app] = []string{rs.ObjectMeta.Name}
		}
	}
	return resourceName, cmMap, secretMap
}

//replenishLabel 获取workloads资源上携带的ConfigMap和secret，以及把pvc加上标签。
func (c *clusterAction) replenishLabel(ctx context.Context, resource *Resource, namespace string, app string) ([]string, []string) {
	var cmList []string
	var secretList []string
	resourceVolume := resource.Template.Spec.Volumes
	for _, volume := range resourceVolume {
		if pvc := volume.PersistentVolumeClaim; pvc != nil {
			PersistentVolumeClaims, err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(context.Background(), pvc.ClaimName, metav1.GetOptions{})
			if err != nil {
				logrus.Errorf("Failed to get PersistentVolumeClaims %s/%s:%v", namespace, pvc.ClaimName, err)
			}
			if PersistentVolumeClaims.Labels == nil {
				PersistentVolumeClaims.Labels = make(map[string]string)
			}
			if _, ok := PersistentVolumeClaims.Labels["app"]; !ok {
				if _, ok := PersistentVolumeClaims.Labels["app.kubernetes.io/name"]; !ok {
					PersistentVolumeClaims.Labels["app"] = app
				}
			}
			_, err = c.clientset.CoreV1().PersistentVolumeClaims(namespace).Update(context.Background(), PersistentVolumeClaims, metav1.UpdateOptions{})
			if err != nil {
				logrus.Errorf("PersistentVolumeClaims label update error:%v", err)
			}
			continue
		}
		if cm := volume.ConfigMap; cm != nil {
			cm, err := c.clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), cm.Name, metav1.GetOptions{})
			if err != nil {
				logrus.Errorf("Failed to get ConfigMap:%v", err)
			}
			if _, ok := cm.Labels["app"]; !ok {
				if _, ok := cm.Labels["app.kubernetes.io/name"]; !ok {
					cmList = append(cmList, cm.Name)
				}
			}
		}
		if secret := volume.Secret; secret != nil {
			secret, err := c.clientset.CoreV1().Secrets(namespace).Get(context.Background(), secret.SecretName, metav1.GetOptions{})
			if err != nil {
				logrus.Errorf("Failed to get Scret:%v", err)
			}
			if _, ok := secret.Labels["app"]; !ok {
				if _, ok := secret.Labels["app.kubernetes.io/name"]; !ok {
					cmList = append(cmList, secret.Name)
				}
			}
		}
	}
	return cmList, secretList
}
