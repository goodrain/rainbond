package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	u "github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/util/constants"
	"github.com/jinzhu/gorm"
	"github.com/shirou/gopsutil/disk"
	"github.com/sirupsen/logrus"
	"github.com/twinj/uuid"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"os"
	"path"
	"runtime"
	"sigs.k8s.io/yaml"
	"strconv"
	"strings"
	"time"
)

// ClusterHandler -
type ClusterHandler interface {
	GetClusterInfo(ctx context.Context) (*model.ClusterResource, error)
	MavenSettingAdd(ctx context.Context, ms *MavenSetting) *util.APIHandleError
	MavenSettingList(ctx context.Context) (re []MavenSetting)
	MavenSettingUpdate(ctx context.Context, ms *MavenSetting) *util.APIHandleError
	MavenSettingDelete(ctx context.Context, name string) *util.APIHandleError
	MavenSettingDetail(ctx context.Context, name string) (*MavenSetting, *util.APIHandleError)
	GetNamespace(ctx context.Context, content string) ([]string, *util.APIHandleError)
	GetNamespaceSource(ctx context.Context, content string, namespace string) (map[string]model.LabelResource, *util.APIHandleError)
	ConvertResource(ctx context.Context, namespace string, lr map[string]model.LabelResource) (map[string]model.ApplicationResource, *util.APIHandleError)
	ResourceImport(ctx context.Context, namespace string, as map[string]model.ApplicationResource, eid string) (*model.ReturnResourceImport, *util.APIHandleError)
}

// NewClusterHandler -
func NewClusterHandler(clientset *kubernetes.Clientset, RbdNamespace string) ClusterHandler {
	return &clusterAction{
		namespace: RbdNamespace,
		clientset: clientset,
	}
}

type clusterAction struct {
	namespace        string
	clientset        *kubernetes.Clientset
	clusterInfoCache *model.ClusterResource
	cacheTime        time.Time
}

func (c *clusterAction) GetClusterInfo(ctx context.Context) (*model.ClusterResource, error) {
	timeout, _ := strconv.Atoi(os.Getenv("CLUSTER_INFO_CACHE_TIME"))
	if timeout == 0 {
		// default is 30 seconds
		timeout = 30
	}
	if c.clusterInfoCache != nil && c.cacheTime.Add(time.Second*time.Duration(timeout)).After(time.Now()) {
		return c.clusterInfoCache, nil
	}
	if c.clusterInfoCache != nil {
		logrus.Debugf("cluster info cache is timeout, will calculate a new value")
	}

	nodes, err := c.listNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("[GetClusterInfo] list nodes: %v", err)
	}

	var healthCapCPU, healthCapMem, unhealthCapCPU, unhealthCapMem int64
	usedNodeList := make([]*corev1.Node, len(nodes))
	for i := range nodes {
		node := nodes[i]
		if !isNodeReady(node) {
			logrus.Debugf("[GetClusterInfo] node(%s) not ready", node.GetName())
			unhealthCapCPU += node.Status.Allocatable.Cpu().Value()
			unhealthCapMem += node.Status.Allocatable.Memory().Value()
			continue
		}

		healthCapCPU += node.Status.Allocatable.Cpu().Value()
		healthCapMem += node.Status.Allocatable.Memory().Value()
		if node.Spec.Unschedulable == false {
			usedNodeList[i] = node
		}
	}

	var healthcpuR, healthmemR, unhealthCPUR, unhealthMemR, rbdMemR, rbdCPUR int64
	nodeAllocatableResourceList := make(map[string]*model.NodeResource, len(usedNodeList))
	var maxAllocatableMemory *model.NodeResource
	for i := range usedNodeList {
		node := usedNodeList[i]

		pods, err := c.listPods(ctx, node.Name)
		if err != nil {
			return nil, fmt.Errorf("list pods: %v", err)
		}

		nodeAllocatableResource := model.NewResource(node.Status.Allocatable)
		for _, pod := range pods {
			nodeAllocatableResource.AllowedPodNumber--
			for _, c := range pod.Spec.Containers {
				nodeAllocatableResource.Memory -= c.Resources.Requests.Memory().Value()
				nodeAllocatableResource.MilliCPU -= c.Resources.Requests.Cpu().MilliValue()
				nodeAllocatableResource.EphemeralStorage -= c.Resources.Requests.StorageEphemeral().Value()
				if isNodeReady(node) {
					healthcpuR += c.Resources.Requests.Cpu().MilliValue()
					healthmemR += c.Resources.Requests.Memory().Value()
				} else {
					unhealthCPUR += c.Resources.Requests.Cpu().MilliValue()
					unhealthMemR += c.Resources.Requests.Memory().Value()
				}
				if pod.Labels["creator"] == "Rainbond" {
					rbdMemR += c.Resources.Requests.Memory().Value()
					rbdCPUR += c.Resources.Requests.Cpu().MilliValue()
				}
			}
		}
		nodeAllocatableResourceList[node.Name] = nodeAllocatableResource

		// Gets the node resource with the maximum remaining scheduling memory
		if maxAllocatableMemory == nil {
			maxAllocatableMemory = nodeAllocatableResource
		} else {
			if nodeAllocatableResource.Memory > maxAllocatableMemory.Memory {
				maxAllocatableMemory = nodeAllocatableResource
			}
		}
	}

	var diskstauts *disk.UsageStat
	if runtime.GOOS != "windows" {
		diskstauts, _ = disk.Usage("/grdata")
	} else {
		diskstauts, _ = disk.Usage(`z:\\`)
	}
	var diskCap, reqDisk uint64
	if diskstauts != nil {
		diskCap = diskstauts.Total
		reqDisk = diskstauts.Used
	}

	result := &model.ClusterResource{
		CapCPU:                           int(healthCapCPU + unhealthCapCPU),
		CapMem:                           int(healthCapMem+unhealthCapMem) / 1024 / 1024,
		HealthCapCPU:                     int(healthCapCPU),
		HealthCapMem:                     int(healthCapMem) / 1024 / 1024,
		UnhealthCapCPU:                   int(unhealthCapCPU),
		UnhealthCapMem:                   int(unhealthCapMem) / 1024 / 1024,
		ReqCPU:                           float32(healthcpuR+unhealthCPUR) / 1000,
		ReqMem:                           int(healthmemR+unhealthMemR) / 1024 / 1024,
		RainbondReqCPU:                   float32(rbdCPUR) / 1000,
		RainbondReqMem:                   int(rbdMemR) / 1024 / 1024,
		HealthReqCPU:                     float32(healthcpuR) / 1000,
		HealthReqMem:                     int(healthmemR) / 1024 / 1024,
		UnhealthReqCPU:                   float32(unhealthCPUR) / 1000,
		UnhealthReqMem:                   int(unhealthMemR) / 1024 / 1024,
		ComputeNode:                      len(nodes),
		CapDisk:                          diskCap,
		ReqDisk:                          reqDisk,
		MaxAllocatableMemoryNodeResource: maxAllocatableMemory,
	}

	result.AllNode = len(nodes)
	for _, node := range nodes {
		if !isNodeReady(node) {
			result.NotReadyNode++
		}
	}
	c.clusterInfoCache = result
	c.cacheTime = time.Now()
	return result, nil
}

func (c *clusterAction) listNodes(ctx context.Context) ([]*corev1.Node, error) {
	opts := metav1.ListOptions{}
	nodeList, err := c.clientset.CoreV1().Nodes().List(ctx, opts)
	if err != nil {
		return nil, err
	}

	var nodes []*corev1.Node
	for idx := range nodeList.Items {
		node := &nodeList.Items[idx]
		// check if node contains taints
		if containsTaints(node) {
			logrus.Debugf("[GetClusterInfo] node(%s) contains NoSchedule taints", node.GetName())
			continue
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

func isNodeReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func containsTaints(node *corev1.Node) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Effect == corev1.TaintEffectNoSchedule {
			return true
		}
	}
	return false
}

func (c *clusterAction) listPods(ctx context.Context, nodeName string) (pods []corev1.Pod, err error) {
	podList, err := c.clientset.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": nodeName}).String()})
	if err != nil {
		return pods, err
	}

	return podList.Items, nil
}

//MavenSetting maven setting
type MavenSetting struct {
	Name       string `json:"name" validate:"required"`
	CreateTime string `json:"create_time"`
	UpdateTime string `json:"update_time"`
	Content    string `json:"content" validate:"required"`
	IsDefault  bool   `json:"is_default"`
}

//MavenSettingList maven setting list
func (c *clusterAction) MavenSettingList(ctx context.Context) (re []MavenSetting) {
	cms, err := c.clientset.CoreV1().ConfigMaps(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "configtype=mavensetting",
	})
	if err != nil {
		logrus.Errorf("list maven setting config list failure %s", err.Error())
	}
	for _, sm := range cms.Items {
		isDefault := false
		if sm.Labels["default"] == "true" {
			isDefault = true
		}
		re = append(re, MavenSetting{
			Name:       sm.Name,
			CreateTime: sm.CreationTimestamp.Format(time.RFC3339),
			UpdateTime: sm.Labels["updateTime"],
			Content:    sm.Data["mavensetting"],
			IsDefault:  isDefault,
		})
	}
	return
}

//MavenSettingAdd maven setting add
func (c *clusterAction) MavenSettingAdd(ctx context.Context, ms *MavenSetting) *util.APIHandleError {
	config := &corev1.ConfigMap{}
	config.Name = ms.Name
	config.Namespace = c.namespace
	config.Labels = map[string]string{
		"creator":    "Rainbond",
		"configtype": "mavensetting",
	}
	config.Annotations = map[string]string{
		"updateTime": time.Now().Format(time.RFC3339),
	}
	config.Data = map[string]string{
		"mavensetting": ms.Content,
	}
	_, err := c.clientset.CoreV1().ConfigMaps(c.namespace).Create(ctx, config, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return &util.APIHandleError{Code: 400, Err: fmt.Errorf("setting name is exist")}
		}
		logrus.Errorf("create maven setting configmap failure %s", err.Error())
		return &util.APIHandleError{Code: 500, Err: fmt.Errorf("create setting config failure")}
	}
	ms.CreateTime = time.Now().Format(time.RFC3339)
	ms.UpdateTime = time.Now().Format(time.RFC3339)
	return nil
}

//MavenSettingUpdate maven setting file update
func (c *clusterAction) MavenSettingUpdate(ctx context.Context, ms *MavenSetting) *util.APIHandleError {
	sm, err := c.clientset.CoreV1().ConfigMaps(c.namespace).Get(ctx, ms.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return &util.APIHandleError{Code: 404, Err: fmt.Errorf("setting name is not exist")}
		}
		logrus.Errorf("get maven setting config list failure %s", err.Error())
		return &util.APIHandleError{Code: 400, Err: fmt.Errorf("get setting failure")}
	}
	if sm.Data == nil {
		sm.Data = make(map[string]string)
	}
	if sm.Annotations == nil {
		sm.Annotations = make(map[string]string)
	}
	sm.Data["mavensetting"] = ms.Content
	sm.Annotations["updateTime"] = time.Now().Format(time.RFC3339)
	if _, err := c.clientset.CoreV1().ConfigMaps(c.namespace).Update(ctx, sm, metav1.UpdateOptions{}); err != nil {
		logrus.Errorf("update maven setting configmap failure %s", err.Error())
		return &util.APIHandleError{Code: 500, Err: fmt.Errorf("update setting config failure")}
	}
	ms.UpdateTime = sm.Annotations["updateTime"]
	ms.CreateTime = sm.CreationTimestamp.Format(time.RFC3339)
	return nil
}

//MavenSettingDelete maven setting file delete
func (c *clusterAction) MavenSettingDelete(ctx context.Context, name string) *util.APIHandleError {
	err := c.clientset.CoreV1().ConfigMaps(c.namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return &util.APIHandleError{Code: 404, Err: fmt.Errorf("setting not found")}
		}
		logrus.Errorf("delete maven setting config list failure %s", err.Error())
		return &util.APIHandleError{Code: 500, Err: fmt.Errorf("setting delete failure")}
	}
	return nil
}

//MavenSettingDetail maven setting file delete
func (c *clusterAction) MavenSettingDetail(ctx context.Context, name string) (*MavenSetting, *util.APIHandleError) {
	sm, err := c.clientset.CoreV1().ConfigMaps(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("get maven setting config failure %s", err.Error())
		return nil, &util.APIHandleError{Code: 404, Err: fmt.Errorf("setting not found")}
	}
	return &MavenSetting{
		Name:       sm.Name,
		CreateTime: sm.CreationTimestamp.Format(time.RFC3339),
		UpdateTime: sm.Annotations["updateTime"],
		Content:    sm.Data["mavensetting"],
	}, nil
}

//GetNamespace Get namespace of the current cluster
func (c *clusterAction) GetNamespace(ctx context.Context, content string) ([]string, *util.APIHandleError) {
	namespaceList, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, &util.APIHandleError{Code: 404, Err: fmt.Errorf("failed to get namespace:%v", err)}
	}
	namespaces := new([]string)
	for _, ns := range namespaceList.Items {
		if strings.HasPrefix(ns.Name, "kube-") || ns.Name == "rainbond" || ns.Name == "rbd-system" {
			continue
		}
		rbdNamespace := false
		for labelKey, labelValue := range ns.Labels {
			if labelKey == constants.ResourceManagedByLabel && labelValue == "rainbond" {
				rbdNamespace = true
			}
		}
		if content == "unmanaged" && rbdNamespace {
			continue
		}
		*namespaces = append(*namespaces, ns.Name)
	}
	return *namespaces, nil
}

//MapAddMap map去重合并
func MapAddMap(map1 map[string][]string, map2 map[string][]string) map[string][]string {
	for k, v := range map1 {
		if _, ok := map2[k]; ok {
			map2[k] = append(map2[k], v...)
			continue
		}
		map2[k] = v
	}
	return map2
}

//GetNamespaceSource Get all resources in the current namespace
func (c *clusterAction) GetNamespaceSource(ctx context.Context, content string, namespace string) (map[string]model.LabelResource, *util.APIHandleError) {
	logrus.Infof("GetNamespaceSource function begin")
	//存储workloads们的ConfigMap
	cmsMap := make(map[string][]string)
	//存储workloads们的secrets
	secretsMap := make(map[string][]string)
	deployments, cmMap, secretMap := c.getResourceName(ctx, namespace, content, model.Deployment)
	if len(cmsMap) != 0 {
		cmsMap = MapAddMap(cmMap, cmsMap)
	}
	if len(secretMap) != 0 {
		secretsMap = MapAddMap(secretMap, secretsMap)
	}
	jobs, cmMap, secretMap := c.getResourceName(ctx, namespace, content, model.Job)
	if len(cmsMap) != 0 {
		cmsMap = MapAddMap(cmMap, cmsMap)
	}
	if len(secretMap) != 0 {
		secretsMap = MapAddMap(secretMap, secretsMap)
	}
	cronJobs, cmMap, secretMap := c.getResourceName(ctx, namespace, content, model.CronJob)
	if len(cmsMap) != 0 {
		cmsMap = MapAddMap(cmMap, cmsMap)
	}
	if len(secretMap) != 0 {
		secretsMap = MapAddMap(secretMap, secretsMap)
	}
	stateFulSets, cmMap, secretMap := c.getResourceName(ctx, namespace, content, model.StateFulSet)
	if len(cmsMap) != 0 {
		cmsMap = MapAddMap(cmMap, cmsMap)
	}
	if len(secretMap) != 0 {
		secretsMap = MapAddMap(secretMap, secretsMap)
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
	networkPolicies, _, _ := c.getResourceName(ctx, namespace, content, model.NetworkPolicie)
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
		ConfigMaps:               MapAddMap(cmsMap, cms),
		Secrets:                  MapAddMap(secretsMap, secrets),
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
		resources, err := c.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Deployment list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta, Template: dm.Spec.Template})
		}
		logrus.Infof("Deployments:%v", tempResources)
		isWorkloads = true
	case model.Job:
		resources, err := c.clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Job list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta, Template: dm.Spec.Template})
		}
		logrus.Infof("Jobs:%v", tempResources)
		isWorkloads = true
	case model.CronJob:
		resources, err := c.clientset.BatchV1beta1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get CronJob list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta, Template: dm.Spec.JobTemplate.Spec.Template})
		}
		logrus.Infof("CronJobs:%v", tempResources)
		isWorkloads = true
	case model.StateFulSet:
		resources, err := c.clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get StateFulSets list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta, Template: dm.Spec.Template})
		}
		logrus.Infof("StateFulSets:%v", tempResources)
		isWorkloads = true
	case model.Service:
		resources, err := c.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Services list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta})
		}
		logrus.Infof("Service:%v", tempResources)
	case model.PVC:
		resources, err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get PersistentVolumeClaims list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {

			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta})
		}
		logrus.Infof("pvc:%v", tempResources)
	case model.Ingress:
		resources, err := c.clientset.ExtensionsV1beta1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Ingresses list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta})
		}
		logrus.Infof("ingress:%v", tempResources)
	case model.NetworkPolicie:
		resources, err := c.clientset.ExtensionsV1beta1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get NetworkPolicies list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta})
		}
		logrus.Infof("network:%v", tempResources)
	case model.ConfigMap:
		resources, err := c.clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get ConfigMaps list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta})
		}
		logrus.Infof("configmaps:%v", tempResources)
	case model.Secret:
		resources, err := c.clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Secrets list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta})
		}
		logrus.Infof("secrets:%v", tempResources)
	case model.ServiceAccount:
		resources, err := c.clientset.CoreV1().ServiceAccounts(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get ServiceAccounts list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta})
		}
	case model.RoleBinding:
		resources, err := c.clientset.RbacV1alpha1().RoleBindings(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get RoleBindings list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta})
		}
	case model.HorizontalPodAutoscaler:
		resources, err := c.clientset.AutoscalingV1().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get HorizontalPodAutoscalers list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, hpa := range resources.Items {
			rbdResource := false
			labels := make(map[string]string)
			switch hpa.Spec.ScaleTargetRef.Kind {
			case model.Deployment:
				deploy, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, hpa.Spec.ScaleTargetRef.Name, metav1.GetOptions{})
				if err != nil {
					logrus.Errorf("The bound deployment does not exist:%v", err)
				}
				if labelValue, ok := hpa.ObjectMeta.Labels["creator"]; ok && labelValue == "Rainbond" {
					rbdResource = true
				}
				labels = deploy.ObjectMeta.Labels
			case model.StateFulSet:
				ss, err := c.clientset.AppsV1().StatefulSets(namespace).Get(ctx, hpa.Spec.ScaleTargetRef.Name, metav1.GetOptions{})
				if err != nil {
					logrus.Errorf("The bound deployment does not exist:%v", err)
				}
				if labelValue, ok := hpa.ObjectMeta.Labels["creator"]; ok && labelValue == "Rainbond" {
					rbdResource = true
				}
				labels = ss.ObjectMeta.Labels
			}
			var app string
			if content == "unmanaged" && rbdResource {
				continue
			}
			if labelValue, ok := labels["app.kubernetes.io/name"]; ok {
				app = labelValue
			} else if lValue, ok := labels["app"]; ok {
				app = lValue
			}
			if app == "" {
				app = "UnLabel"
			}
			if _, ok := resourceName[app]; ok {
				resourceName[app] = append(resourceName[app], hpa.Name)
			} else {
				resourceName[app] = []string{hpa.Name}
			}
		}
		return resourceName, nil, nil
	case model.Role:
		resources, err := c.clientset.RbacV1alpha1().Roles(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Roles list:%v", err)
			return nil, cmMap, secretMap
		}
		for _, dm := range resources.Items {
			tempResources = append(tempResources, &Resource{ObjectMeta: dm.ObjectMeta})
		}
		logrus.Infof("roles:%v", tempResources)
	}
	//这一块是统一处理资源，按label划分出来
	for _, rs := range tempResources {
		rbdResource := false
		var app string
		if labelValue, ok := rs.ObjectMeta.Labels["creator"]; ok && labelValue == "Rainbond" {
			rbdResource = true
		}
		if content == "unmanaged" && rbdResource {
			continue
		}
		if labelValue, ok := rs.ObjectMeta.Labels["app.kubernetes.io/name"]; ok {
			app = labelValue
		} else if lValue, ok := rs.ObjectMeta.Labels["app"]; ok {
			app = lValue
		}
		if app == "" {
			app = "UnLabel"
		}
		//如果是Workloads类型的资源需要检查其内部configmap、secret、PVC（防止没有这三种资源没有label但是用到了）
		if isWorkloads {
			cmList, secretList := c.replenishLabel(ctx, rs, namespace, app)
			if _, ok := cmMap[app]; ok {
				cmMap[app] = append(cmMap[app], cmList...)
			}
			cmMap[app] = cmList
			if _, ok := secretMap[app]; ok {
				secretMap[app] = append(secretMap[app], secretList...)
			}
			secretMap[app] = secretList
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
			PersistentVolumeClaims, err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvc.ClaimName, metav1.GetOptions{})
			if err != nil {
				logrus.Errorf("Failed to get PersistentVolumeClaims:%v", err)
			}
			if _, ok := PersistentVolumeClaims.Labels["app"]; !ok {
				if _, ok := PersistentVolumeClaims.Labels["app.kubernetes.io/name"]; !ok {
					PersistentVolumeClaims.Labels["app"] = app
				}
			}
			_, err = c.clientset.CoreV1().PersistentVolumeClaims(namespace).Update(ctx, PersistentVolumeClaims, metav1.UpdateOptions{})
			if err != nil {
				logrus.Errorf("PersistentVolumeClaims label update error:%v", err)
			}
			continue
		}
		if cm := volume.ConfigMap; cm != nil {
			cm, err := c.clientset.CoreV1().ConfigMaps(namespace).Get(ctx, cm.Name, metav1.GetOptions{})
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
			secret, err := c.clientset.CoreV1().Secrets(namespace).Get(ctx, secret.SecretName, metav1.GetOptions{})
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
	dmCR := c.workloadDeployments(ctx, lr.Workloads.Deployments, namespace)
	sfsCR := c.workloadStateFulSets(ctx, lr.Workloads.StateFulSets, namespace)
	jCR := c.workloadJobs(ctx, lr.Workloads.Jobs, namespace)
	wCJ := c.workloadCronJobs(ctx, lr.Workloads.CronJobs, namespace)
	convertResource := append(dmCR, append(sfsCR, append(jCR, append(wCJ)...)...)...)
	cr[app] = model.ApplicationResource{
		ConvertResource: convertResource,
	}
}

func (c *clusterAction) workloadDeployments(ctx context.Context, dmNames []string, namespace string) []model.ConvertResource {
	var componentsCR []model.ConvertResource
	for _, dmName := range dmNames {
		resources, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, dmName, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Failed to get Deployment %v:%v", dmName, err)
			return nil
		}

		//BasicManagement
		b := model.BasicManagement{
			ResourceType: model.Deployment,
			Replicas:     *resources.Spec.Replicas,
			Memory:       resources.Spec.Template.Spec.Containers[0].Resources.Limits.Memory().Value() / 1024 / 1024,
			CPU:          resources.Spec.Template.Spec.Containers[0].Resources.Limits.Cpu().Value(),
			Image:        resources.Spec.Template.Spec.Containers[0].Image,
			Cmd:          strings.Join(append(resources.Spec.Template.Spec.Containers[0].Command, resources.Spec.Template.Spec.Containers[0].Args...), " "),
		}

		//Port
		var ps []model.PortManagement
		for _, port := range resources.Spec.Template.Spec.Containers[0].Ports {
			if string(port.Protocol) == "UDP" {
				ps = append(ps, model.PortManagement{
					Port:     port.ContainerPort,
					Protocol: "UDP",
					Inner:    false,
					Outer:    false,
				})
				continue
			}
			if string(port.Protocol) == "TCP" {
				ps = append(ps, model.PortManagement{
					Port:     port.ContainerPort,
					Protocol: "UDP",
					Inner:    false,
					Outer:    false,
				})
				continue
			}
			logrus.Warningf("Transport protocol type not recognized%v", port.Protocol)
		}

		//ENV
		var envs []model.ENVManagement
		for _, env := range resources.Spec.Template.Spec.Containers[0].Env {
			if cm := env.ValueFrom; cm == nil {
				envs = append(envs, model.ENVManagement{
					ENVKey:     env.Name,
					ENVValue:   env.Value,
					ENVExplain: "",
				})
			}
		}

		//Configs
		var configs []model.ConfigManagement
		//这一块是处理配置文件
		//配置文件的名字最终都是configmap里面的key值。
		//volume在被挂载后存在四种情况
		//第一种是volume存在items，volumeMount的SubPath不等于空。路径直接是volumeMount里面的mountPath。
		//第二种是volume存在items，volumeMount的SubPath等于空。路径则变成volumeMount里面的mountPath拼接上items里面每一个元素的key值。
		//第三种是volume不存在items，volumeMount的SubPath不等于空。路径直接是volumeMount里面的mountPath。
		//第四种是volume不存在items，volumeMount的SubPath等于空。路径则变成volumeMount里面的mountPath拼接上configmap资源里面每一个元素的key值
		for _, volume := range resources.Spec.Template.Spec.Volumes {
			if volume.ConfigMap != nil {
				cm, err := c.clientset.CoreV1().ConfigMaps(namespace).Get(ctx, volume.ConfigMap.Name, metav1.GetOptions{})
				if err != nil {
					logrus.Errorf("Failed to get ConfigMap %v:%v", volume.Name, err)
				}
				cmData := cm.Data
				isLog := true
				for _, volumeMount := range resources.Spec.Template.Spec.Containers[0].VolumeMounts {
					if volume.Name != volumeMount.Name {
						continue
					}
					isLog = false
					if volume.ConfigMap.Items != nil {
						if volumeMount.SubPath != "" {
							configName := ""
							var mode int32
							for _, item := range volume.ConfigMap.Items {
								if item.Path == volumeMount.SubPath {
									configName = item.Key
									mode = *item.Mode
								}
							}
							configs = append(configs, model.ConfigManagement{
								ConfigName:  configName,
								ConfigPath:  volumeMount.MountPath,
								ConfigValue: cmData[configName],
								Mode:        mode,
							})
							continue
						}
						p := volumeMount.MountPath
						for _, item := range volume.ConfigMap.Items {
							p := path.Join(p, item.Path)
							configs = append(configs, model.ConfigManagement{
								ConfigName:  item.Key,
								ConfigPath:  p,
								ConfigValue: cmData[item.Key],
								Mode:        *item.Mode,
							})
						}
					} else {
						if volumeMount.SubPath != "" {
							configs = append(configs, model.ConfigManagement{
								ConfigName:  volumeMount.SubPath,
								ConfigPath:  volumeMount.MountPath,
								ConfigValue: cmData[volumeMount.SubPath],
								Mode:        *volume.ConfigMap.DefaultMode,
							})
							continue
						}
						mountPath := volumeMount.MountPath
						for key, val := range cmData {
							mountPath = path.Join(mountPath, key)
							configs = append(configs, model.ConfigManagement{
								ConfigName:  key,
								ConfigPath:  mountPath,
								ConfigValue: val,
								Mode:        *volume.ConfigMap.DefaultMode,
							})
						}
					}
				}
				if isLog {
					logrus.Warningf("configmap type resource %v is not mounted in volumemount", volume.ConfigMap.Name)
				}
			}
		}

		//TelescopicManagement
		HPAResource, err := c.clientset.AutoscalingV1().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Failed to get HorizontalPodAutoscalers list:%v", err)
			return nil
		}
		var t model.TelescopicManagement
		//这一块就是自动伸缩的对应解析，
		//需要注意的一点是hpa的cpu和memory的阈值设置是通过Annotations["autoscaling.alpha.kubernetes.io/metrics"]字段设置
		//而且它的返回值是个json字符串所以设置了一个结构体进行解析。
		for _, hpa := range HPAResource.Items {
			if hpa.Spec.ScaleTargetRef.Kind != model.Deployment || hpa.Spec.ScaleTargetRef.Name != dmName {
				t.Enable = false
				continue
			}
			t.Enable = true
			t.MinReplicas = *hpa.Spec.MinReplicas
			t.MaxReplicas = hpa.Spec.MaxReplicas
			var cpuormemorys []*dbmodel.TenantServiceAutoscalerRuleMetrics
			cpuUsage := hpa.Spec.TargetCPUUtilizationPercentage
			if cpuUsage != nil {
				cpuormemorys = append(cpuormemorys, &dbmodel.TenantServiceAutoscalerRuleMetrics{
					MetricsType:       "resource_metrics",
					MetricsName:       "cpu",
					MetricTargetType:  "utilization",
					MetricTargetValue: int(*cpuUsage),
				})
			}
			CPUAndMemoryJson, ok := hpa.Annotations["autoscaling.alpha.kubernetes.io/metrics"]
			if ok {
				type com struct {
					T        string `json:"type"`
					Resource map[string]interface{}
				}
				var c []com
				err := json.Unmarshal([]byte(CPUAndMemoryJson), &c)
				if err != nil {
					logrus.Errorf("autoscaling.alpha.kubernetes.io/metrics parsing failed：%v", err)
					return nil
				}

				for _, cpuormemory := range c {
					switch cpuormemory.Resource["name"] {
					case "cpu":
						cpu := fmt.Sprint(cpuormemory.Resource["targetAverageValue"])
						if cpu[len(cpu)-1:] == "m" {
							cpuuse, _ := strconv.Atoi(cpu[:len(cpu)-1])
							cpuormemorys = append(cpuormemorys, &dbmodel.TenantServiceAutoscalerRuleMetrics{
								MetricsType:       "resource_metrics",
								MetricsName:       "cpu",
								MetricTargetType:  "average_value",
								MetricTargetValue: cpuuse,
							})
						}
						if cpu[len(cpu)-1:] == "g" || cpu[len(cpu)-1:] == "G" {
							cpuuse, _ := strconv.Atoi(cpu[:len(cpu)-1])
							cpuormemorys = append(cpuormemorys, &dbmodel.TenantServiceAutoscalerRuleMetrics{
								MetricsType:       "resource_metrics",
								MetricsName:       "cpu",
								MetricTargetType:  "average_value",
								MetricTargetValue: cpuuse * 1024,
							})
						}
					case "memory":
						memory := fmt.Sprint(cpuormemory.Resource["targetAverageValue"])
						if memory[len(memory)-1:] == "m" {
							memoryuse, _ := strconv.Atoi(memory[:len(memory)-1])
							cpuormemorys = append(cpuormemorys, &dbmodel.TenantServiceAutoscalerRuleMetrics{
								MetricsType:       "resource_metrics",
								MetricsName:       "memory",
								MetricTargetType:  "average_value",
								MetricTargetValue: memoryuse,
							})
						}
						if memory[len(memory)-1:] == "g" || memory[len(memory)-1:] == "G" {
							memoryuse, _ := strconv.Atoi(memory[:len(memory)-1])
							cpuormemorys = append(cpuormemorys, &dbmodel.TenantServiceAutoscalerRuleMetrics{
								MetricsType:       "resource_metrics",
								MetricsName:       "memory",
								MetricTargetType:  "average_value",
								MetricTargetValue: memoryuse * 1024,
							})
						}
						memoryusage := int(cpuormemory.Resource["targetAverageUtilization"].(float64))
						cpuormemorys = append(cpuormemorys, &dbmodel.TenantServiceAutoscalerRuleMetrics{
							MetricsType:       "resource_metrics",
							MetricsName:       "memory",
							MetricTargetType:  "utilization",
							MetricTargetValue: memoryusage,
						})
					}

				}
			}
			t.CpuOrMemory = cpuormemorys
		}

		//HealthyCheckManagement
		var hcm model.HealthyCheckManagement
		livenessProbe := resources.Spec.Template.Spec.Containers[0].LivenessProbe
		if livenessProbe != nil {
			var httpHeaders []string
			for _, httpHeader := range livenessProbe.HTTPGet.HTTPHeaders {
				nv := httpHeader.Name + "=" + httpHeader.Value
				httpHeaders = append(httpHeaders, nv)
			}
			hcm.Status = 1
			hcm.DetectionMethod = strings.ToLower(string(livenessProbe.HTTPGet.Scheme))
			hcm.Port = int(livenessProbe.HTTPGet.Port.IntVal)
			hcm.Path = livenessProbe.HTTPGet.Path
			if livenessProbe.Exec != nil {
				hcm.Command = strings.Join(livenessProbe.Exec.Command, " ")
			}
			hcm.HttpHeader = strings.Join(httpHeaders, ",")
			hcm.Mode = "liveness"
			hcm.InitialDelaySecond = int(livenessProbe.InitialDelaySeconds)
			hcm.PeriodSecond = int(livenessProbe.PeriodSeconds)
			hcm.TimeoutSecond = int(livenessProbe.TimeoutSeconds)
			hcm.FailureThreshold = int(livenessProbe.FailureThreshold)
			hcm.SuccessThreshold = int(livenessProbe.SuccessThreshold)
		} else {
			readinessProbe := resources.Spec.Template.Spec.Containers[0].ReadinessProbe
			var httpHeaders []string
			for _, httpHeader := range readinessProbe.HTTPGet.HTTPHeaders {
				nv := httpHeader.Name + "=" + httpHeader.Value
				httpHeaders = append(httpHeaders, nv)
			}
			if readinessProbe != nil {
				hcm.Status = 1
				hcm.DetectionMethod = strings.ToLower(string(readinessProbe.HTTPGet.Scheme))
				hcm.Mode = "readiness"
				hcm.Port = int(livenessProbe.HTTPGet.Port.IntVal)
				hcm.Path = readinessProbe.HTTPGet.Path
				if readinessProbe.Exec != nil {
					hcm.Command = strings.Join(readinessProbe.Exec.Command, " ")
				}
				hcm.HttpHeader = strings.Join(httpHeaders, ",")
				hcm.InitialDelaySecond = int(readinessProbe.InitialDelaySeconds)
				hcm.PeriodSecond = int(readinessProbe.PeriodSeconds)
				hcm.TimeoutSecond = int(readinessProbe.TimeoutSeconds)
				hcm.FailureThreshold = int(readinessProbe.FailureThreshold)
				hcm.SuccessThreshold = int(readinessProbe.SuccessThreshold)
			}
		}
		var attributes []*dbmodel.ComponentK8sAttributes

		if resources.Spec.Template.Spec.Volumes != nil {
			volumesYaml, err := ObjectToJsonORYam("yaml", resources.Spec.Template.Spec.Volumes)
			if err != nil {
				logrus.Errorf("deployment:%v volumes %v", dmName, err)
				return nil
			}
			volumesAttributes := &dbmodel.ComponentK8sAttributes{
				Name:           dbmodel.K8sAttributeNameVolumes,
				SaveType:       "yaml",
				AttributeValue: volumesYaml,
			}
			attributes = append(attributes, volumesAttributes)

		}
		if resources.Spec.Template.Spec.Containers[0].VolumeMounts != nil {
			volumeMountsYaml, err := ObjectToJsonORYam("yaml", resources.Spec.Template.Spec.Containers[0].VolumeMounts)
			if err != nil {
				logrus.Errorf("deployment:%v volumeMounts %v", dmName, err)
				return nil
			}
			volumeMountsAttributes := &dbmodel.ComponentK8sAttributes{
				Name:           dbmodel.K8sAttributeNameVolumeMounts,
				SaveType:       "yaml",
				AttributeValue: volumeMountsYaml,
			}
			attributes = append(attributes, volumeMountsAttributes)
		}
		if resources.Spec.Template.Spec.ServiceAccountName != "" {
			serviceAccountAttributes := &dbmodel.ComponentK8sAttributes{
				Name:           dbmodel.K8sAttributeNameServiceAccountName,
				SaveType:       "string",
				AttributeValue: resources.Spec.Template.Spec.ServiceAccountName,
			}
			attributes = append(attributes, serviceAccountAttributes)
		}
		if resources.Labels != nil {
			labelsJson, err := ObjectToJsonORYam("json", resources.Labels)
			if err != nil {
				logrus.Errorf("deployment:%v labels %v", dmName, err)
				return nil
			}
			labelsAttributes := &dbmodel.ComponentK8sAttributes{
				Name:           dbmodel.K8sAttributeNameLabels,
				SaveType:       "json",
				AttributeValue: labelsJson,
			}
			attributes = append(attributes, labelsAttributes)
		}
		if resources.Spec.Template.Spec.NodeSelector != nil {
			NodeSelectorJson, err := ObjectToJsonORYam("json", resources.Spec.Template.Spec.NodeSelector)
			if err != nil {
				logrus.Errorf("deployment:%v nodeSelector %v", dmName, err)
				return nil
			}
			nodeSelectorAttributes := &dbmodel.ComponentK8sAttributes{
				Name:           dbmodel.K8sAttributeNameNodeSelector,
				SaveType:       "json",
				AttributeValue: NodeSelectorJson,
			}
			attributes = append(attributes, nodeSelectorAttributes)
		}
		if resources.Spec.Template.Spec.Tolerations != nil {
			tolerationsYaml, err := ObjectToJsonORYam("yaml", resources.Spec.Template.Spec.Tolerations)
			if err != nil {
				logrus.Errorf("deployment:%v tolerations %v", dmName, err)
				return nil
			}
			tolerationsAttributes := &dbmodel.ComponentK8sAttributes{
				Name:           dbmodel.K8sAttributeNameTolerations,
				SaveType:       "yaml",
				AttributeValue: tolerationsYaml,
			}
			attributes = append(attributes, tolerationsAttributes)
		}
		if resources.Spec.Template.Spec.Affinity != nil {
			affinityYaml, err := ObjectToJsonORYam("yaml", resources.Spec.Template.Spec.Affinity)
			if err != nil {
				logrus.Errorf("deployment:%v affinity %v", dmName, err)
				return nil
			}
			affinityAttributes := &dbmodel.ComponentK8sAttributes{
				Name:           dbmodel.K8sAttributeNameAffinity,
				SaveType:       "yaml",
				AttributeValue: affinityYaml,
			}
			attributes = append(attributes, affinityAttributes)
		}
		if securityContext := resources.Spec.Template.Spec.Containers[0].SecurityContext; securityContext != nil && securityContext.Privileged != nil {
			privilegedAttributes := &dbmodel.ComponentK8sAttributes{
				Name:           dbmodel.K8sAttributeNamePrivileged,
				SaveType:       "string",
				AttributeValue: strconv.FormatBool(*securityContext.Privileged),
			}
			attributes = append(attributes, privilegedAttributes)
		}

		componentsCR = append(componentsCR, model.ConvertResource{
			ComponentsName:                   dmName,
			BasicManagement:                  b,
			PortManagement:                   ps,
			ENVManagement:                    envs,
			ConfigManagement:                 configs,
			TelescopicManagement:             t,
			HealthyCheckManagement:           hcm,
			ComponentK8sAttributesManagement: attributes,
		})

	}
	return componentsCR
}

func (c *clusterAction) workloadStateFulSets(ctx context.Context, sfsNames []string, namespace string) []model.ConvertResource {
	return nil
}

func (c *clusterAction) workloadJobs(ctx context.Context, jNames []string, namespace string) []model.ConvertResource {
	return nil
}

func (c *clusterAction) workloadCronJobs(ctx context.Context, cjNames []string, namespace string) []model.ConvertResource {
	return nil
}

//ResourceImport Import the converted k8s resources into recognition
func (c *clusterAction) ResourceImport(ctx context.Context, namespace string, as map[string]model.ApplicationResource, eid string) (*model.ReturnResourceImport, *util.APIHandleError) {
	logrus.Infof("ResourceImport function begin")
	var returnResourceImport model.ReturnResourceImport
	err := db.GetManager().DB().Transaction(func(tx *gorm.DB) error {
		tenant, err := c.createTenant(ctx, eid, namespace, tx)
		returnResourceImport.Tenant = tenant
		if err != nil {
			logrus.Errorf("%v", err)
			return &util.APIHandleError{Code: 400, Err: fmt.Errorf("create tenant error:%v", err)}
		}
		for appName, components := range as {
			app, err := c.createApp(eid, tx, appName, tenant.UUID)
			if err != nil {
				logrus.Errorf("%v", err)
				return &util.APIHandleError{Code: 400, Err: fmt.Errorf("create app error:%v", err)}
			}
			var ca []model.ComponentAttributes
			for _, componentResource := range components.ConvertResource {
				component, err := c.createComponent(ctx, app, tenant.UUID, componentResource, namespace)
				if err != nil {
					logrus.Errorf("%v", err)
					return &util.APIHandleError{Code: 400, Err: fmt.Errorf("create app error:%v", err)}
				}
				c.createENV(componentResource.ENVManagement, component)
				c.createConfig(componentResource.ConfigManagement, component)
				c.createPort(componentResource.PortManagement, component)
				componentResource.TelescopicManagement.RuleID = c.createTelescopic(componentResource.TelescopicManagement, component)
				componentResource.HealthyCheckManagement.ProbeID = c.createHealthyCheck(componentResource.HealthyCheckManagement, component)
				c.createSpecial(componentResource.ComponentK8sAttributesManagement, tenant.UUID, component)
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
				App:       app,
				Component: ca,
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
	ns, err := c.clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return nil, &util.APIHandleError{Code: 404, Err: fmt.Errorf("failed to get namespace %v:%v", namespace, err)}
	}
	ns.Labels[constants.ResourceManagedByLabel] = constants.Rainbond
	_, err = c.clientset.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{})
	if err != nil {
		return nil, &util.APIHandleError{Code: 404, Err: fmt.Errorf("failed to add label to namespace %v:%v", namespace, err)}
	}
	logrus.Infof("end create tenant")
	return &dbts, nil
}

func (c *clusterAction) createApp(eid string, tx *gorm.DB, app string, tenantID string) (*dbmodel.Application, error) {
	appID := u.NewUUID()
	application, _ := db.GetManager().ApplicationDaoTransactions(tx).GetAppByName(tenantID, app, app)
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

func (c *clusterAction) createComponent(ctx context.Context, app *dbmodel.Application, tenantID string, component model.ConvertResource, namespace string) (*dbmodel.TenantServices, error) {
	serviceID := strings.Replace(uuid.NewV4().String(), "-", "", -1)
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
		ExtendMethod:     "stateless_multiple",
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
	dm, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, component.ComponentsName, metav1.GetOptions{})
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
	dm.Labels["creater_id"] = string(u.NewTimeVersion())
	dm.Labels["migrator"] = "rainbond"
	dm.Spec.Template.Labels["service_id"] = serviceID
	dm.Spec.Template.Labels["version"] = ts.DeployVersion
	dm.Spec.Template.Labels["creater_id"] = string(u.NewTimeVersion())
	dm.Spec.Template.Labels["migrator"] = "rainbond"
	_, err = c.clientset.AppsV1().Deployments(namespace).Update(ctx, dm, metav1.UpdateOptions{})
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
	fmt.Println(ports)
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
		RuleID:      strings.Replace(uuid.NewV4().String(), "-", "", -1),
		ServiceID:   service.ServiceID,
		Enable:      true,
		XPAType:     "hpa",
		MinReplicas: int(telescopic.MinReplicas),
		MaxReplicas: int(telescopic.MaxReplicas),
	}
	telescopic.RuleID = r.RuleID
	if err := db.GetManager().TenantServceAutoscalerRulesDao().AddModel(r); err != nil {
		logrus.Errorf("%v TenantServiceAutoscalerRules creation failed:%v", service.ServiceAlias, err)
	}
	for _, metric := range telescopic.CpuOrMemory {
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
	tspD.HTTPHeader = telescopic.HttpHeader
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

func (c *clusterAction) createSpecial(specials []*dbmodel.ComponentK8sAttributes, tenantID string, component *dbmodel.TenantServices) {
	for _, specials := range specials {
		specials.TenantID = tenantID
		specials.ComponentID = component.ServiceID
	}
	err := db.GetManager().ComponentK8sAttributeDao().CreateOrUpdateAttributesInBatch(specials)
	if err != nil {
		logrus.Errorf("%v createSpecial creation failed:%v", component.ServiceAlias, err)
	}
}

//ObjectToJsonORYam changeType true is Json /false is yaml
func ObjectToJsonORYam(changeType string, data interface{}) (string, error) {
	dataJson, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("json serialization failed err:%v", err)
	}
	if changeType == "json" {
		return string(dataJson), nil
	}
	dataYaml, err := yaml.JSONToYAML(dataJson)
	if err != nil {
		return "", fmt.Errorf("yaml serialization failed err:%v", err)
	}
	return string(dataYaml), nil
}
