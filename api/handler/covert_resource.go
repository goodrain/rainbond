package handler

import (
	"fmt"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	v1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

//ConvertResource 处理资源
func (c *clusterAction) ConvertResource(ctx context.Context, namespace string, lr map[string]model.LabelResource) (map[string]model.ApplicationResource, *util.APIHandleError) {
	logrus.Infof("ConvertResource function begin")
	appsServices := make(map[string]model.ApplicationResource)
	for label, resource := range lr {
		c.workloadHandle(ctx, appsServices, resource, namespace, label)
	}
	logrus.Infof("ConvertResource function end")
	return appsServices, nil
}

func (c *clusterAction) workloadHandle(ctx context.Context, cr map[string]model.ApplicationResource, lr model.LabelResource, namespace string, label string) {
	app := label
	deployResource := c.workloadDeployments(lr.Workloads.Deployments, namespace)
	stsResource := c.workloadStateFulSets(lr.Workloads.StateFulSets, namespace)
	jobResource := c.workloadJobs(lr.Workloads.Jobs, namespace)
	cjResource := c.workloadCronJobs(lr.Workloads.CronJobs, namespace)
	convertResource := append(deployResource, append(stsResource, append(jobResource, append(cjResource)...)...)...)
	k8sResources := c.getAppKubernetesResources(ctx, lr.Others, namespace)
	cr[app] = model.ApplicationResource{
		ConvertResource:     convertResource,
		KubernetesResources: k8sResources,
	}
}

func (c *clusterAction) workloadDeployments(dmNames []string, namespace string) []model.ConvertResource {
	var componentsCR []model.ConvertResource
	for _, dmName := range dmNames {
		resources, err := c.clientset.AppsV1().Deployments(namespace).Get(context.Background(), dmName, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Deployment %v:%v", dmName, err)
			return nil
		}
		memory, cpu := resources.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().Value(), resources.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().MilliValue()
		if memory == 0 {
			memory = resources.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().Value()
		}
		if cpu == 0 {
			cpu = resources.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().MilliValue()
		}
		//BasicManagement
		basic := model.BasicManagement{
			ResourceType: model.Deployment,
			Replicas:     resources.Spec.Replicas,
			Memory:       memory / 1024 / 1024,
			CPU:          cpu,
			Image:        resources.Spec.Template.Spec.Containers[0].Image,
			Cmd:          strings.Join(append(resources.Spec.Template.Spec.Containers[0].Command, resources.Spec.Template.Spec.Containers[0].Args...), " "),
		}
		parameter := model.YamlResourceParameter{
			ComponentsCR: &componentsCR,
			Basic:        basic,
			Template:     resources.Spec.Template,
			Namespace:    namespace,
			Name:         dmName,
			RsLabel:      resources.Labels,
		}
		c.PodTemplateSpecResource(parameter, nil)
	}
	return componentsCR
}

func (c *clusterAction) workloadStateFulSets(stsNames []string, namespace string) []model.ConvertResource {
	var componentsCR []model.ConvertResource
	for _, stsName := range stsNames {
		resources, err := c.clientset.AppsV1().StatefulSets(namespace).Get(context.Background(), stsName, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Deployment %v:%v", stsName, err)
			return nil
		}
		memory, cpu := resources.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().Value(), resources.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().MilliValue()
		if memory == 0 {
			memory = resources.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().Value()
		}
		if cpu == 0 {
			cpu = resources.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().MilliValue()
		}
		//BasicManagement
		basic := model.BasicManagement{
			ResourceType: model.StateFulSet,
			Replicas:     resources.Spec.Replicas,
			Memory:       memory / 1024 / 1024,
			CPU:          cpu,
			Image:        resources.Spec.Template.Spec.Containers[0].Image,
			Cmd:          strings.Join(append(resources.Spec.Template.Spec.Containers[0].Command, resources.Spec.Template.Spec.Containers[0].Args...), " "),
		}
		parameter := model.YamlResourceParameter{
			ComponentsCR: &componentsCR,
			Basic:        basic,
			Template:     resources.Spec.Template,
			Namespace:    namespace,
			Name:         stsName,
			RsLabel:      resources.Labels,
		}
		c.PodTemplateSpecResource(parameter, resources.Spec.VolumeClaimTemplates)
	}
	return componentsCR
}

func (c *clusterAction) workloadJobs(jobNames []string, namespace string) []model.ConvertResource {
	var componentsCR []model.ConvertResource
	for _, jobName := range jobNames {
		resources, err := c.clientset.BatchV1().Jobs(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Deployment %v:%v", jobName, err)
			return nil
		}
		var BackoffLimit, Parallelism, ActiveDeadlineSeconds, Completions string
		if resources.Spec.BackoffLimit != nil {
			BackoffLimit = fmt.Sprintf("%v", *resources.Spec.BackoffLimit)
		}
		if resources.Spec.Parallelism != nil {
			Parallelism = fmt.Sprintf("%v", *resources.Spec.Parallelism)
		}
		if resources.Spec.ActiveDeadlineSeconds != nil {
			ActiveDeadlineSeconds = fmt.Sprintf("%v", *resources.Spec.ActiveDeadlineSeconds)
		}
		if resources.Spec.Completions != nil {
			Completions = fmt.Sprintf("%v", *resources.Spec.Completions)
		}
		job := model.JobStrategy{
			Schedule:              resources.Spec.Template.Spec.SchedulerName,
			BackoffLimit:          BackoffLimit,
			Parallelism:           Parallelism,
			ActiveDeadlineSeconds: ActiveDeadlineSeconds,
			Completions:           Completions,
		}
		memory, cpu := resources.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().Value(), resources.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().MilliValue()
		if memory == 0 {
			memory = resources.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().Value()
		}
		if cpu == 0 {
			cpu = resources.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().MilliValue()
		}
		//BasicManagement
		basic := model.BasicManagement{
			ResourceType: model.Job,
			Replicas:     resources.Spec.Completions,
			Memory:       memory / 1024 / 1024,
			CPU:          cpu,
			Image:        resources.Spec.Template.Spec.Containers[0].Image,
			Cmd:          strings.Join(append(resources.Spec.Template.Spec.Containers[0].Command, resources.Spec.Template.Spec.Containers[0].Args...), " "),
			JobStrategy:  job,
		}
		parameter := model.YamlResourceParameter{
			ComponentsCR: &componentsCR,
			Basic:        basic,
			Template:     resources.Spec.Template,
			Namespace:    namespace,
			Name:         jobName,
			RsLabel:      resources.Labels,
		}
		c.PodTemplateSpecResource(parameter, nil)
	}
	return componentsCR
}

func (c *clusterAction) workloadCronJobs(cjNames []string, namespace string) []model.ConvertResource {
	var componentsCR []model.ConvertResource
	for _, cjName := range cjNames {
		resources, err := c.clientset.BatchV1beta1().CronJobs(namespace).Get(context.Background(), cjName, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Deployment %v:%v", cjName, err)
			return nil
		}
		BackoffLimit, Parallelism, ActiveDeadlineSeconds, Completions := "", "", "", ""
		if resources.Spec.JobTemplate.Spec.BackoffLimit != nil {
			BackoffLimit = fmt.Sprintf("%v", *resources.Spec.JobTemplate.Spec.BackoffLimit)
		}
		if resources.Spec.JobTemplate.Spec.Parallelism != nil {
			Parallelism = fmt.Sprintf("%v", *resources.Spec.JobTemplate.Spec.Parallelism)
		}
		if resources.Spec.JobTemplate.Spec.ActiveDeadlineSeconds != nil {
			ActiveDeadlineSeconds = fmt.Sprintf("%v", *resources.Spec.JobTemplate.Spec.ActiveDeadlineSeconds)
		}
		if resources.Spec.JobTemplate.Spec.Completions != nil {
			Completions = fmt.Sprintf("%v", *resources.Spec.JobTemplate.Spec.Completions)
		}
		job := model.JobStrategy{
			Schedule:              resources.Spec.Schedule,
			BackoffLimit:          BackoffLimit,
			Parallelism:           Parallelism,
			ActiveDeadlineSeconds: ActiveDeadlineSeconds,
			Completions:           Completions,
		}
		memory, cpu := resources.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Resources.Requests.Memory().Value(), resources.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Resources.Requests.Cpu().MilliValue()
		if memory == 0 {
			memory = resources.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().Value()
		}
		if cpu == 0 {
			cpu = resources.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().MilliValue()
		}
		//BasicManagement
		basic := model.BasicManagement{
			ResourceType: model.CronJob,
			Replicas:     resources.Spec.JobTemplate.Spec.Completions,
			Memory:       memory / 1024 / 1024,
			CPU:          cpu,
			Image:        resources.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image,
			Cmd:          strings.Join(append(resources.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Command, resources.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Args...), " "),
			JobStrategy:  job,
		}
		parameter := model.YamlResourceParameter{
			ComponentsCR: &componentsCR,
			Basic:        basic,
			Template:     resources.Spec.JobTemplate.Spec.Template,
			Namespace:    namespace,
			Name:         cjName,
			RsLabel:      resources.Labels,
		}
		c.PodTemplateSpecResource(parameter, nil)
	}
	return componentsCR
}

func (c *clusterAction) getAppKubernetesResources(ctx context.Context, others model.OtherResource, namespace string) []dbmodel.K8sResource {
	var k8sResources []dbmodel.K8sResource
	servicesMap := make(map[string]corev1.Service)
	servicesList, err := c.clientset.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("namespace:%v get services error:%v", namespace, err)
	}
	if len(others.Services) != 0 && err == nil {
		for _, services := range servicesList.Items {
			servicesMap[services.Name] = services
		}
		for _, servicesName := range others.Services {
			services, _ := servicesMap[servicesName]
			services.Kind = model.Service
			services.Status = corev1.ServiceStatus{}
			services.APIVersion = "v1"
			services.ManagedFields = []metav1.ManagedFieldsEntry{}
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", services)
			if err != nil {
				logrus.Errorf("namespace:%v service:%v error: %v", namespace, services.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:          services.Name,
				Kind:          model.Service,
				Content:       kubernetesResourcesYAML,
				State:         1,
				ErrorOverview: "创建成功",
			})
		}
	}

	pvcMap := make(map[string]corev1.PersistentVolumeClaim)
	pvcList, err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("namespace:%v get pvc error:%v", namespace, err)
	}
	if len(others.PVC) != 0 && err == nil {
		for _, pvc := range pvcList.Items {
			pvcMap[pvc.Name] = pvc
		}
		for _, pvcName := range others.PVC {
			pvc, _ := pvcMap[pvcName]
			pvc.Status = corev1.PersistentVolumeClaimStatus{}
			pvc.ManagedFields = []metav1.ManagedFieldsEntry{}
			pvc.Kind = model.PVC
			pvc.APIVersion = "v1"
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", pvc)
			if err != nil {
				logrus.Errorf("namespace:%v pvc:%v error: %v", namespace, pvc.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:          pvc.Name,
				Kind:          model.PVC,
				Content:       kubernetesResourcesYAML,
				State:         1,
				ErrorOverview: "创建成功",
			})
		}
	}

	ingressMap := make(map[string]networkingv1.Ingress)
	ingressList, err := c.clientset.NetworkingV1().Ingresses(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("namespace:%v get ingresses error:%v", namespace, err)
	}
	if len(others.Ingresses) != 0 && err == nil {
		for _, ingress := range ingressList.Items {
			ingressMap[ingress.Name] = ingress
		}
		for _, ingressName := range others.Ingresses {
			ingresses, _ := ingressMap[ingressName]
			ingresses.Status = networkingv1.IngressStatus{}
			ingresses.ManagedFields = []metav1.ManagedFieldsEntry{}
			ingresses.Kind = model.Ingress
			ingresses.APIVersion = "networking.k8s.io/v1"
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", ingresses)
			if err != nil {
				logrus.Errorf("namespace:%v ingresses:%v error: %v", namespace, ingresses.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:          ingresses.Name,
				Kind:          model.Ingress,
				Content:       kubernetesResourcesYAML,
				State:         1,
				ErrorOverview: "创建成功",
			})
		}
	}

	networkPoliciesMap := make(map[string]networkingv1.NetworkPolicy)
	networkPoliciesList, err := c.clientset.NetworkingV1().NetworkPolicies(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("namespace:%v get NetworkPolicies error:%v", namespace, err)
	}
	if len(others.NetworkPolicies) != 0 && err == nil {
		for _, networkPolicies := range networkPoliciesList.Items {
			networkPoliciesMap[networkPolicies.Name] = networkPolicies
		}
		for _, networkPoliciesName := range others.NetworkPolicies {
			networkPolicies, _ := networkPoliciesMap[networkPoliciesName]
			networkPolicies.ManagedFields = []metav1.ManagedFieldsEntry{}
			networkPolicies.Kind = model.NetworkPolicy
			networkPolicies.APIVersion = "networking.k8s.io/v1"
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", networkPolicies)
			if err != nil {
				logrus.Errorf("namespace:%v NetworkPolicies:%v error: %v", namespace, networkPolicies.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:          networkPolicies.Name,
				Kind:          model.NetworkPolicy,
				Content:       kubernetesResourcesYAML,
				State:         1,
				ErrorOverview: "创建成功",
			})
		}
	}

	cmMap := make(map[string]corev1.ConfigMap)
	cmList, err := c.clientset.CoreV1().ConfigMaps(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("namespace:%v get ConfigMaps error:%v", namespace, err)
	}
	if len(others.ConfigMaps) != 0 && err == nil {
		for _, cm := range cmList.Items {
			cmMap[cm.Name] = cm
		}
		for _, configMapsName := range others.ConfigMaps {
			configMaps, _ := cmMap[configMapsName]
			configMaps.ManagedFields = []metav1.ManagedFieldsEntry{}
			configMaps.Kind = model.ConfigMap
			configMaps.APIVersion = "v1"
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", configMaps)
			if err != nil {
				logrus.Errorf("namespace:%v ConfigMaps:%v error: %v", namespace, configMaps.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:          configMaps.Name,
				Kind:          model.ConfigMap,
				Content:       kubernetesResourcesYAML,
				State:         1,
				ErrorOverview: "创建成功",
			})
		}
	}

	secretsMap := make(map[string]corev1.Secret)
	secretsList, err := c.clientset.CoreV1().Secrets(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("namespace:%v get Secrets error:%v", namespace, err)
	}
	if len(others.Secrets) != 0 && err == nil {
		for _, secrets := range secretsList.Items {
			secretsMap[secrets.Name] = secrets
		}
		for _, secretsName := range others.Secrets {
			secrets, _ := secretsMap[secretsName]
			secrets.ManagedFields = []metav1.ManagedFieldsEntry{}
			secrets.Kind = model.Secret
			secrets.APIVersion = "v1"
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", secrets)
			if err != nil {
				logrus.Errorf("namespace:%v Secrets:%v error: %v", namespace, secrets.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:          secrets.Name,
				Kind:          model.Secret,
				Content:       kubernetesResourcesYAML,
				State:         1,
				ErrorOverview: "创建成功",
			})
		}
	}

	serviceAccountsMap := make(map[string]corev1.ServiceAccount)
	serviceAccountsList, err := c.clientset.CoreV1().ServiceAccounts(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("namespace:%v get ServiceAccounts error:%v", namespace, err)
	}
	if len(others.ServiceAccounts) != 0 && err == nil {
		for _, serviceAccounts := range serviceAccountsList.Items {
			serviceAccountsMap[serviceAccounts.Name] = serviceAccounts
		}
		for _, serviceAccountsName := range others.ServiceAccounts {
			serviceAccounts, _ := serviceAccountsMap[serviceAccountsName]
			serviceAccounts.ManagedFields = []metav1.ManagedFieldsEntry{}
			serviceAccounts.Kind = model.ServiceAccount
			serviceAccounts.APIVersion = "v1"
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", serviceAccounts)
			if err != nil {
				logrus.Errorf("namespace:%v ServiceAccounts:%v error: %v", namespace, serviceAccounts.Name, err)
				continue
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:          serviceAccounts.Name,
				Kind:          model.ServiceAccount,
				Content:       kubernetesResourcesYAML,
				State:         1,
				ErrorOverview: "创建成功",
			})
		}
	}

	roleBindingsMap := make(map[string]rbacv1.RoleBinding)
	roleBindingsList, _ := c.clientset.RbacV1().RoleBindings(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("namespace:%v get RoleBindings error:%v", namespace, err)
	}
	if len(others.RoleBindings) != 0 && err == nil {
		for _, roleBindings := range roleBindingsList.Items {
			roleBindingsMap[roleBindings.Name] = roleBindings
		}
		for _, roleBindingsName := range others.RoleBindings {
			roleBindings, _ := roleBindingsMap[roleBindingsName]
			roleBindings.ManagedFields = []metav1.ManagedFieldsEntry{}
			roleBindings.Kind = model.RoleBinding
			roleBindings.APIVersion = "rbac.authorization.k8s.io/v1"
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", roleBindings)
			if err != nil {
				logrus.Errorf("namespace:%v RoleBindings:%v error: %v", namespace, roleBindings.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:          roleBindings.Name,
				Kind:          model.RoleBinding,
				Content:       kubernetesResourcesYAML,
				State:         1,
				ErrorOverview: "创建成功",
			})
		}
	}

	hpaMap := make(map[string]v1.HorizontalPodAutoscaler)
	hpaList, _ := c.clientset.AutoscalingV1().HorizontalPodAutoscalers(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("namespace:%v get HorizontalPodAutoscalers error:%v", namespace, err)
	}
	if len(others.HorizontalPodAutoscalers) != 0 && err == nil {
		for _, hpa := range hpaList.Items {
			hpaMap[hpa.Name] = hpa
		}
		for _, hpaName := range others.HorizontalPodAutoscalers {
			hpa, _ := hpaMap[hpaName]
			hpa.Status = v1.HorizontalPodAutoscalerStatus{}
			hpa.ManagedFields = []metav1.ManagedFieldsEntry{}
			hpa.Kind = model.HorizontalPodAutoscaler
			hpa.APIVersion = "autoscaling/v1"
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", hpa)
			if err != nil {
				logrus.Errorf("namespace:%v HorizontalPodAutoscalers:%v error: %v", namespace, hpa.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:          hpa.Name,
				Kind:          model.HorizontalPodAutoscaler,
				Content:       kubernetesResourcesYAML,
				State:         1,
				ErrorOverview: "创建成功",
			})
		}
	}

	rolesMap := make(map[string]rbacv1.Role)
	rolesList, err := c.clientset.RbacV1().Roles(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("namespace:%v get roles error:%v", namespace, err)
	}
	if len(others.Roles) != 0 && err == nil {
		for _, roles := range rolesList.Items {
			rolesMap[roles.Name] = roles
		}
		for _, rolesName := range others.Roles {
			roles, _ := rolesMap[rolesName]
			roles.Kind = model.Role
			roles.APIVersion = "rbac.authorization.k8s.io/v1"
			kubernetesResourcesYAML, err := ObjectToJSONORYaml("yaml", roles)
			if err != nil {
				logrus.Errorf("namespace:%v roles:%v error: %v", namespace, roles.Name, err)
			}
			k8sResources = append(k8sResources, dbmodel.K8sResource{
				Name:          roles.Name,
				Kind:          model.Role,
				Content:       kubernetesResourcesYAML,
				ErrorOverview: "创建成功",
				State:         1,
			})
		}
	}
	return k8sResources
}
