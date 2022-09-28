package handler

import (
	"context"
	"fmt"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/util/constants"
	"github.com/shirou/gopsutil/disk"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net"
	"os"
	"runtime"
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
	ResourceImport(namespace string, as map[string]model.ApplicationResource, eid string) (*model.ReturnResourceImport, *util.APIHandleError)
	AddAppK8SResource(ctx context.Context, namespace string, appID string, resourceYaml string) ([]*dbmodel.K8sResource, *util.APIHandleError)
	DeleteAppK8SResource(ctx context.Context, namespace, appID, name, yaml, kind string) *util.APIHandleError
	UpdateAppK8SResource(ctx context.Context, namespace, appID, name, resourceYaml, kind string) (dbmodel.K8sResource, *util.APIHandleError)
	SyncAppK8SResources(ctx context.Context, resources *model.SyncResources) ([]*dbmodel.K8sResource, *util.APIHandleError)
	AppYamlResourceName(yamlResource model.YamlResource) (map[string]model.LabelResource, *util.APIHandleError)
	AppYamlResourceDetailed(yamlResource model.YamlResource, yamlImport bool) (model.ApplicationResource, *util.APIHandleError)
	AppYamlResourceImport(yamlResource model.YamlResource, components model.ApplicationResource) (model.AppComponent, *util.APIHandleError)
}

// NewClusterHandler -
func NewClusterHandler(clientset *kubernetes.Clientset, RbdNamespace string, config *rest.Config, mapper meta.RESTMapper) ClusterHandler {
	return &clusterAction{
		namespace: RbdNamespace,
		clientset: clientset,
		config:    config,
		mapper:    mapper,
	}
}

type clusterAction struct {
	namespace        string
	clientset        *kubernetes.Clientset
	clusterInfoCache *model.ClusterResource
	cacheTime        time.Time
	config           *rest.Config
	mapper           meta.RESTMapper
}

//GetClusterInfo -
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
	var resourceProxyStatus bool
	conn, err := net.DialTimeout("tcp", "rbd-resource-proxy:80", 3*time.Second)
	if err != nil || conn == nil {
		logrus.Errorf("connection rbd-resource-proxy failed error, %v", err)
		resourceProxyStatus = false
	} else {
		resourceProxyStatus = true
	}
	conn.Close()
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
		ResourceProxyStatus:              resourceProxyStatus,
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
		if labelValue, isRBDNamespace := ns.Labels[constants.ResourceManagedByLabel]; isRBDNamespace && labelValue == "rainbond" && content == "unmanaged" {
			continue
		}
		*namespaces = append(*namespaces, ns.Name)
	}
	return *namespaces, nil
}

//MergeMap map去重合并
func MergeMap(map1 map[string][]string, map2 map[string][]string) map[string][]string {
	for k, v := range map1 {
		if _, ok := map2[k]; ok {
			map2[k] = append(map2[k], v...)
			continue
		}
		map2[k] = v
	}
	return map2
}
