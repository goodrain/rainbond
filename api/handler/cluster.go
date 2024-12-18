package handler

import (
	"context"
	"fmt"
	rainbondv1alpha1 "github.com/goodrain/rainbond-operator/api/v1alpha1"
	"github.com/goodrain/rainbond-operator/util/rbdutil"
	"github.com/goodrain/rainbond/api/client/prometheus"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/grctl/clients"
	mqclient "github.com/goodrain/rainbond/mq/client"
	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/pkg/component/grpc"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/pkg/component/mq"
	"github.com/goodrain/rainbond/pkg/component/prom"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	utils "github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/util/constants"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	workerclient "github.com/goodrain/rainbond/worker/client"
	"github.com/goodrain/rainbond/worker/server/pb"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/sirupsen/logrus"
	"io"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/util/flushwriter"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gateway "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/typed/apis/v1beta1"
	"strconv"
	"strings"
	"time"
)

const (
	// OfficialPluginLabel Those with this label are official plug-ins
	OfficialPluginLabel = "plugin.rainbond.io/name"
)

// ClusterHandler -
type ClusterHandler interface {
	GetClusterInfo(ctx context.Context) (*model.ClusterResource, error)
	MavenSettingAdd(ctx context.Context, ms *MavenSetting) *util.APIHandleError
	MavenSettingList(ctx context.Context) (re []MavenSetting)
	MavenSettingUpdate(ctx context.Context, ms *MavenSetting) *util.APIHandleError
	MavenSettingDelete(ctx context.Context, name string) *util.APIHandleError
	MavenSettingDetail(ctx context.Context, name string) (*MavenSetting, *util.APIHandleError)
	BatchGetGateway(ctx context.Context) ([]*model.GatewayResource, *util.APIHandleError)
	GetNamespace(ctx context.Context, content string) ([]string, *util.APIHandleError)
	GetNamespaceSource(ctx context.Context, content string, namespace string) (map[string]model.LabelResource, *util.APIHandleError)
	ConvertResource(ctx context.Context, namespace string, lr map[string]model.LabelResource) (map[string]model.ApplicationResource, *util.APIHandleError)
	ResourceImport(namespace string, as map[string]model.ApplicationResource, eid string) (*model.ReturnResourceImport, *util.APIHandleError)
	AddAppK8SResource(ctx context.Context, namespace string, appID string, resourceYaml string) ([]*dbmodel.K8sResource, *util.APIHandleError)
	DeleteAppK8SResource(ctx context.Context, namespace, appID, name, yaml, kind string)
	GetAppK8SResource(ctx context.Context, namespace, appID, name, resourceYaml, kind string) (dbmodel.K8sResource, *util.APIHandleError)
	UpdateAppK8SResource(ctx context.Context, namespace, appID, name, resourceYaml, kind string) (dbmodel.K8sResource, *util.APIHandleError)
	SyncAppK8SResources(ctx context.Context, resources *model.SyncResources) ([]*dbmodel.K8sResource, *util.APIHandleError)
	AppYamlResourceName(yamlResource model.YamlResource) (map[string]model.LabelResource, *util.APIHandleError)
	AppYamlResourceDetailed(yamlResource model.YamlResource, yamlImport bool) (model.ApplicationResource, *util.APIHandleError)
	AppYamlResourceImport(namespace, tenantID, appID string, components model.ApplicationResource) (model.AppComponent, *util.APIHandleError)
	RbdLog(w http.ResponseWriter, r *http.Request, podName string, follow bool) error
	GetRbdPods() (rbds []model.RbdResp, err error)
	CreateShellPod(regionName string) (pod *corev1.Pod, err error)
	DeleteShellPod(podName string) error
	ListPlugins(official bool) (rbds []*model.RainbondPlugins, err error)
	ListAbilities() (rbds []unstructured.Unstructured, err error)
	GetAbility(abilityID string) (rbd *unstructured.Unstructured, err error)
	UpdateAbility(abilityID string, ability *unstructured.Unstructured) error
	GenerateAbilityID(ability *unstructured.Unstructured) string
	ListRainbondComponents(ctx context.Context) (res []*model.RainbondComponent, err error)
	ListUpgradeStatus() ([]model.ComponentStatus, error)
	GetClusterRegionStatus() (map[string]interface{}, error)
}

// NewClusterHandler -
func NewClusterHandler() ClusterHandler {
	return &clusterAction{
		namespace:      configs.Default().PublicConfig.RbdNamespace,
		clientset:      k8s.Default().Clientset,
		config:         k8s.Default().RestConfig,
		mapper:         k8s.Default().Mapper,
		client:         k8s.Default().K8sClient,
		grctlImage:     configs.Default().APIConfig.GrctlImage,
		prometheusCli:  prom.Default().PrometheusCli,
		rainbondClient: k8s.Default().RainbondClient,
		statusCli:      grpc.Default().StatusClient,
		dynamicClient:  k8s.Default().DynamicClient,
		gatewayClient:  k8s.Default().GatewayClient,
		mqclient:       mq.Default().MqClient,
	}
}

type clusterAction struct {
	namespace        string
	clientset        *kubernetes.Clientset
	clusterInfoCache *model.ClusterResource
	cacheTime        time.Time
	config           *rest.Config
	mapper           meta.RESTMapper
	grctlImage       string
	client           client.Client
	prometheusCli    prometheus.Interface
	rainbondClient   versioned.Interface
	statusCli        *workerclient.AppRuntimeSyncClient
	dynamicClient    dynamic.Interface
	gatewayClient    *gateway.GatewayV1beta1Client
	mqclient         mqclient.MQClient
}

type nodePod struct {
	Memory           prometheus.MetricValue
	CPU              prometheus.MetricValue
	EphemeralStorage prometheus.MetricValue
}

// GetClusterInfo -
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
	var nodeReady int32

	var healthcpuR, healthmemR, unhealthCPUR, unhealthMemR, rbdMemR, rbdCPUR int64
	nodeAllocatableResourceList := make(map[string]*model.NodeResource, len(usedNodeList))
	var maxAllocatableMemory *model.NodeResource
	query := fmt.Sprint(`rbd_api_exporter_cluster_pod_number`)
	podNumber := c.prometheusCli.GetMetric(query, time.Now())
	var instance string
	var runPodNumber int
	for _, podNum := range podNumber.MetricData.MetricValues {
		instance = podNum.Metadata["instance"]
		runPodNumber = int(podNum.Sample.Value())
	}

	query = fmt.Sprintf(`rbd_api_exporter_cluster_pod_memory{instance="%v"}`, instance)
	podMemoryMetric := c.prometheusCli.GetMetric(query, time.Now())

	query = fmt.Sprintf(`rbd_api_exporter_cluster_pod_cpu{instance="%v"}`, instance)
	podCPUMetric := c.prometheusCli.GetMetric(query, time.Now())

	query = fmt.Sprintf(`rbd_api_exporter_cluster_pod_ephemeral_storage{instance="%v"}`, instance)
	podEphemeralStorageMetric := c.prometheusCli.GetMetric(query, time.Now())

	nodeMap := make(map[string][]nodePod)
	for i, memory := range podMemoryMetric.MetricData.MetricValues {
		if nodePodList, ok := nodeMap[memory.Metadata["node_name"]]; ok {
			nodePodList = append(nodePodList, nodePod{
				Memory:           memory,
				CPU:              podCPUMetric.MetricData.MetricValues[i],
				EphemeralStorage: podEphemeralStorageMetric.MetricData.MetricValues[i],
			})
			nodeMap[memory.Metadata["node_name"]] = nodePodList
			continue
		}
		nodeMap[memory.Metadata["node_name"]] = []nodePod{
			{
				Memory:           memory,
				CPU:              podCPUMetric.MetricData.MetricValues[i],
				EphemeralStorage: podEphemeralStorageMetric.MetricData.MetricValues[i],
			},
		}
	}

	for i := range nodes {
		node := nodes[i]
		if !isNodeReady(node) {
			logrus.Debugf("[GetClusterInfo] node(%s) not ready", node.GetName())
			unhealthCapCPU += node.Status.Allocatable.Cpu().Value()
			unhealthCapMem += node.Status.Allocatable.Memory().Value()
			continue
		}
		nodeReady++
		healthCapCPU += node.Status.Allocatable.Cpu().Value()
		healthCapMem += node.Status.Allocatable.Memory().Value()
		if node.Spec.Unschedulable == false {
			usedNodeList[i] = node
		}
		nodeAllocatableResource := model.NewResource(node.Status.Allocatable)
		if nodePods, ok := nodeMap[node.Name]; ok {
			for _, pod := range nodePods {
				memory := int64(pod.Memory.Sample.Value())
				cpu := int64(pod.CPU.Sample.Value())
				ephemeralStorage := int64(pod.EphemeralStorage.Sample.Value())
				nodeAllocatableResource.AllowedPodNumber--
				nodeAllocatableResource.Memory -= memory
				nodeAllocatableResource.MilliCPU -= cpu
				nodeAllocatableResource.EphemeralStorage -= ephemeralStorage
				if isNodeReady(node) {
					healthcpuR += cpu
					healthmemR += memory
				} else {
					unhealthCPUR += cpu
					unhealthMemR += memory
				}
				if _, ok := pod.Memory.Metadata["service_id"]; ok {
					rbdMemR += memory
					rbdCPUR += cpu
				}
				nodeAllocatableResourceList[node.Name] = nodeAllocatableResource
			}
			// Gets the node resource with the maximum remaining scheduling memory
			if maxAllocatableMemory == nil {
				maxAllocatableMemory = nodeAllocatableResource
			} else {
				if nodeAllocatableResource.Memory > maxAllocatableMemory.Memory {
					maxAllocatableMemory = nodeAllocatableResource
				}
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
		ResourceProxyStatus:              true,
		K8sVersion:                       k8sutil.GetKubeVersion().String(),
		NodeReady:                        nodeReady,
		RunPodNumber:                     runPodNumber,
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
		if taint.Effect == corev1.TaintEffectNoSchedule && taint.Key != "node.kubernetes.io/unschedulable" {
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

// MavenSetting maven setting
type MavenSetting struct {
	Name       string `json:"name" validate:"required"`
	CreateTime string `json:"create_time"`
	UpdateTime string `json:"update_time"`
	Content    string `json:"content" validate:"required"`
	IsDefault  bool   `json:"is_default"`
}

// MavenSettingList maven setting list
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

// MavenSettingAdd maven setting add
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

// MavenSettingUpdate maven setting file update
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

// MavenSettingDelete maven setting file delete
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

// MavenSettingDetail maven setting file delete
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

func (c *clusterAction) BatchGetGateway(ctx context.Context) ([]*model.GatewayResource, *util.APIHandleError) {
	gateways, err := c.gatewayClient.Gateways(corev1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, &util.APIHandleError{Code: 404, Err: fmt.Errorf("failed to batch get gateway:%v", err)}
	}
	var gatewayList []*model.GatewayResource
	nodes, err := c.clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{LabelSelector: "rainbond-gateway-node=true"})
	if err != nil {
		return nil, &util.APIHandleError{Code: 404, Err: fmt.Errorf("failed to batch get nodes:%v", err)}
	}
	if len(nodes.Items) == 0 {
		nodes, err = c.clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return nil, &util.APIHandleError{Code: 404, Err: fmt.Errorf("failed to batch get nodes:%v", err)}
		}
	}
	nodeIP := "0.0.0.0"
	if len(nodes.Items) > 0 {
		addresses := nodes.Items[0].Status.Addresses
		if addresses != nil {
			for _, address := range addresses {
				if address.Type == corev1.NodeExternalIP {
					nodeIP = address.Address
					break
				}
				if address.Type == corev1.NodeInternalIP {
					nodeIP = address.Address
				}
			}
		}
	}
	for _, gc := range gateways.Items {
		serviceName, ok := gc.Labels["service-name"]
		var loadBalancerIP []string
		var nodePort []string
		if ok {
			var gatewaySvc corev1.Service
			gatewaySvcList, err := c.clientset.CoreV1().Services(gc.Namespace).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				logrus.Errorf("get gateway(%v) service(%v) failure: %v", gc.Name, serviceName, err)
			}
			for _, gatewayService := range gatewaySvcList.Items {
				if strings.HasPrefix(gatewayService.Name, serviceName) {
					externalIPs := gatewaySvc.Spec.ExternalIPs
					ports := gatewayService.Spec.Ports
					if ports != nil {
						for _, port := range ports {
							if port.NodePort != 0 {
								nodePort = append(nodePort, fmt.Sprintf("%v:%v(%v)", nodeIP, port.NodePort, port.Name))
							}
							if externalIPs != nil {
								for _, ip := range externalIPs {
									loadBalancerIP = append(loadBalancerIP, fmt.Sprintf("%v:%v(%v)", ip, port.Port, port.Name))
								}
							}
						}
					}
					break
				}

			}
		}
		var listenerNames []string
		for _, listener := range gc.Spec.Listeners {
			name := string(listener.Name)
			listenerNames = append(listenerNames, name)
		}
		gatewayList = append(gatewayList, &model.GatewayResource{
			Name:           gc.GetName(),
			Namespace:      gc.GetNamespace(),
			LoadBalancerIP: loadBalancerIP,
			NodePortIP:     nodePort,
			ListenerNames:  listenerNames,
		})
	}
	return gatewayList, nil
}

// GetNamespace Get namespace of the current cluster
func (c *clusterAction) GetNamespace(ctx context.Context, content string) ([]string, *util.APIHandleError) {
	namespaceList, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, &util.APIHandleError{Code: 404, Err: fmt.Errorf("failed to get namespace:%v", err)}
	}
	namespaces := new([]string)
	for _, ns := range namespaceList.Items {
		if strings.HasPrefix(ns.Name, "kube-") || ns.Name == "rainbond" || ns.Name == utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace) {
			continue
		}
		if labelValue, isRBDNamespace := ns.Labels[constants.ResourceManagedByLabel]; isRBDNamespace && labelValue == "rainbond" && content == "unmanaged" {
			continue
		}
		*namespaces = append(*namespaces, ns.Name)
	}
	return *namespaces, nil
}

// MergeMap map去重合并
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

// CreateShellPod -
func (c *clusterAction) CreateShellPod(regionName string) (pod *corev1.Pod, err error) {
	ctx := context.Background()
	volumes := []corev1.Volume{
		{
			Name: "grctl-config",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "grctl-config",
			MountPath: "/root/.rbd",
		},
	}
	labels := make(map[string]string)
	labels["app.kubernetes.io/part-of"] = "shell-tool"
	shellPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("shell-%v-", regionName),
			Namespace:    c.namespace,
			Labels:       labels,
		},
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: new(int64),
			RestartPolicy:                 corev1.RestartPolicyNever,
			NodeSelector: map[string]string{
				"kubernetes.io/os": "linux",
			},
			ServiceAccountName: "rainbond-operator",
			Containers: []corev1.Container{
				{
					Name:            "shell",
					TTY:             true,
					Stdin:           true,
					StdinOnce:       true,
					Image:           c.grctlImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					VolumeMounts:    volumeMounts,
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:            "init-shell",
					TTY:             true,
					Stdin:           true,
					StdinOnce:       true,
					Image:           c.grctlImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"grctl", "install"},
					VolumeMounts:    volumeMounts,
				},
			},
			Volumes: volumes,
		},
	}
	pod, err = c.clientset.CoreV1().Pods(utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace)).Create(ctx, shellPod, metav1.CreateOptions{})
	if err != nil {
		logrus.Error("create shell pod error:", err)
		return nil, err
	}
	return pod, nil
}

// DeleteShellPod -
func (c *clusterAction) DeleteShellPod(podName string) (err error) {
	err = c.clientset.CoreV1().Pods(utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace)).Delete(context.Background(), podName, metav1.DeleteOptions{})
	if err != nil {
		logrus.Error("delete shell pod error:", err)
		return err
	}
	return nil
}

// RbdLog returns the logs reader for a container in a pod, a pod or a component.
func (c *clusterAction) RbdLog(w http.ResponseWriter, r *http.Request, podName string, follow bool) error {
	if podName == "" {
		// Only support return the logs reader for a container now.
		return errors.WithStack(bcode.NewBadRequest("the field 'podName' and 'containerName' is required"))
	}
	request := c.clientset.CoreV1().Pods(utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace)).GetLogs(podName, &corev1.PodLogOptions{
		Follow: follow,
	})
	out, err := request.Stream(context.TODO())
	if err != nil {
		if apierrors.IsNotFound(err) {
			return errors.Wrap(bcode.ErrPodNotFound, "get pod log")
		}
		return errors.Wrap(err, "get stream from request")
	}
	defer out.Close()

	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)

	// Flush headers, if possible
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	writer := flushwriter.Wrap(w)

	_, err = io.Copy(writer, out)
	if err != nil {
		if strings.HasSuffix(err.Error(), "write: broken pipe") {
			return nil
		}
		logrus.Warningf("write stream to response: %v", err)
	}
	return nil
}

// GetRbdPods -
func (c *clusterAction) GetRbdPods() (rbds []model.RbdResp, err error) {
	pods, err := c.clientset.CoreV1().Pods(utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace)).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		logrus.Error("get rbd pod list error:", err)
		return nil, err
	}
	var rbd model.RbdResp
	for _, pod := range pods.Items {
		if strings.Contains(pod.Name, "rbd-chaos") || strings.Contains(pod.Name, "rbd-api") || strings.Contains(pod.Name, "rbd-worker") || strings.Contains(pod.Name, "rbd-gateway") {
			rbdSplit := strings.Split(pod.Name, "-")
			rbdName := fmt.Sprintf("%s-%s", rbdSplit[0], rbdSplit[1])
			rbd.RbdName = rbdName
			rbd.PodName = pod.Name
			rbd.NodeName = pod.Spec.NodeName
			rbds = append(rbds, rbd)
		}
	}
	return rbds, nil
}

func (c *clusterAction) ListRainbondComponents(ctx context.Context) (res []*model.RainbondComponent, err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// rainbond components
	podList, err := c.clientset.CoreV1().Pods(utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace)).List(ctx, metav1.ListOptions{
		LabelSelector: fields.SelectorFromSet(rbdutil.LabelsForRainbond(nil)).String(),
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	pods := make(map[string][]corev1.Pod)
	ComponentAllPods := make(map[string]int)
	ComponentRunPods := make(map[string]int)
	appNameMap := make(map[string]int)
	for _, pod := range podList.Items {
		labels := pod.Labels
		appName := labels["name"]
		if len(appName) == 0 {
			logrus.Warningf("list rainbond components. label 'name' not found for pod(%s/%s)", pod.Namespace, pod.Name)
			continue
		}
		appNameMap[appName]++
		pods[appName] = append(pods[appName], pod)
		if pod.Status.Phase == "Running" {
			// 检查容器是否处于等待或崩溃重启状态
			isContainerRunning := true
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == "CrashLoopBackOff" {
					isContainerRunning = false
					break
				}
			}
			if isContainerRunning {
				ComponentRunPods[appName]++
			}
		}
		ComponentAllPods[appName]++
	}
	var appNames []string
	for name := range appNameMap {
		appNames = append(appNames, name)
	}
	// rainbond operator
	roPods, err := c.clientset.CoreV1().Pods(utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace)).List(ctx, metav1.ListOptions{
		LabelSelector: fields.SelectorFromSet(map[string]string{
			"name": "rainbond-operator",
		}).String(),
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	pods["rainbond-operator"] = roPods.Items
	for _, ropod := range roPods.Items {
		if ropod.Status.Phase == "Running" {
			ComponentRunPods["rainbond-operator"]++
		}
		ComponentAllPods["rainbond-operator"]++
	}
	appNames = append(appNames, "rainbond-operator")
	for _, name := range appNames {
		res = append(res, &model.RainbondComponent{
			Name:    name,
			Pods:    pods[name],
			AllPods: ComponentAllPods[name],
			RunPods: ComponentRunPods[name],
		})
	}
	return res, nil
}

// ListPlugins -
func (c *clusterAction) ListPlugins(official bool) (plugins []*model.RainbondPlugins, err error) {
	res, err := c.HandlePlugins()
	if err != nil {
		return nil, errors.Wrap(err, "get rbd plugins")
	}
	if official {
		for _, plugin := range res {
			if name, ok := plugin.Labels[OfficialPluginLabel]; ok && name != "" {
				plugin.Name = name
				plugins = append(plugins, plugin)
			}
		}
		return plugins, nil
	}
	return res, nil
}

// ListAbilities -
func (c *clusterAction) ListAbilities() (rbds []unstructured.Unstructured, err error) {
	list, err := c.rainbondClient.RainbondV1alpha1().RBDAbilities(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "get rbd abilities")
	}
	// Remove duplicate watchGroups
	existWatchGroups := make(map[string]struct{})
	var watchGroups []v1alpha1.WatchGroup
	for _, item := range list.Items {
		for _, wg := range item.Spec.WatchGroups {
			if wg.APIVersion == "" || wg.Kind == "" {
				continue
			}
			if _, ok := existWatchGroups[wg.APIVersion+wg.Kind]; ok {
				continue
			}
			existWatchGroups[wg.APIVersion+wg.Kind] = struct{}{}
			watchGroups = append(watchGroups, wg)
		}
	}

	for _, wg := range watchGroups {
		gvk := schema.FromAPIVersionAndKind(wg.APIVersion, wg.Kind)
		mapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			logrus.Warningf("get rest mapping for %s/%s: %v", wg.APIVersion, wg.Kind, err)
			continue
		}
		list, err := c.dynamicClient.Resource(mapping.Resource).Namespace(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			logrus.Warningf("list %s/%s: %v", wg.APIVersion, wg.Kind, err)
			continue
		}
		rbds = append(rbds, list.Items...)
	}
	return rbds, nil
}

func (c *clusterAction) GenerateAbilityID(ability *unstructured.Unstructured) string {
	// example abilityID: namespace_apiVersion_kind_name
	return fmt.Sprintf("%s_%s_%s_%s", ability.GetNamespace(), strings.Replace(ability.GetAPIVersion(), "/", "-", -1), ability.GetKind(), ability.GetName())
}

func (c *clusterAction) ParseAbilityID(abilityID string) (namespace, apiVersion, kind, name string, err error) {
	// example abilityID: namespace_apiVersion_kind_name
	split := strings.SplitN(abilityID, "_", 4)
	if len(split) < 4 {
		return "", "", "", "", fmt.Errorf("invalid abilityID: %s", abilityID)
	}
	namespace, apiVersion, kind, name = split[0], split[1], split[2], split[3]
	if strings.ContainsAny(apiVersion, "-") {
		apiVersion = strings.Replace(apiVersion, "-", "/", -1)
	}
	return namespace, apiVersion, kind, name, nil
}

// GetAbility -
func (c *clusterAction) GetAbility(abilityID string) (*unstructured.Unstructured, error) {
	namespace, apiVersion, kind, name, err := c.ParseAbilityID(abilityID)
	if err != nil {
		return nil, err
	}
	logrus.Debugf("get ability: [%v, %v, %v, %v]", namespace, apiVersion, kind, name)

	gvk := schema.FromAPIVersionAndKind(apiVersion, kind)
	mapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, errors.Wrap(err, "ability_id invalid")
	}
	resource, err := c.dynamicClient.Resource(mapping.Resource).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "get ability")
	}
	resource.SetManagedFields(nil)
	return resource, nil
}

// UpdateAbility -
func (c *clusterAction) UpdateAbility(abilityID string, ability *unstructured.Unstructured) error {
	namespace, apiVersion, kind, name, err := c.ParseAbilityID(abilityID)
	if err != nil {
		return err
	}
	logrus.Debugf("update ability: [%v, %v, %v, %v]", namespace, apiVersion, kind, name)

	gvk := schema.FromAPIVersionAndKind(apiVersion, kind)
	mapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return errors.Wrap(err, "ability_id invalid")
	}
	_, err = c.dynamicClient.Resource(mapping.Resource).Namespace(namespace).Update(context.Background(), ability, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "update ability")
	}
	return nil
}

// HandlePlugins -
func (c *clusterAction) HandlePlugins() (plugins []*model.RainbondPlugins, err error) {
	list, err := c.rainbondClient.RainbondV1alpha1().RBDPlugins(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "get rbd plugins")
	}
	// list plugin status
	var appIDs []string
	for _, item := range list.Items {
		if item.Labels["app_id"] != "" {
			appIDs = append(appIDs, item.Labels["app_id"])
		}
	}
	appStatuses := make(map[string]string)
	statuses, err := c.statusCli.ListAppStatuses(context.Background(), &pb.AppStatusesReq{
		AppIds: appIDs,
	})
	if err != nil {
		return nil, errors.Wrap(err, "get app statuses")
	}
	for _, status := range statuses.AppStatuses {
		appStatuses[status.AppId] = status.Status
	}

	// Establish the mapping relationship between appID and teamName
	apps, err := db.GetManager().ApplicationDao().ListByAppIDs(appIDs)
	if err != nil {
		return nil, errors.Wrap(err, "get apps by app ids")
	}
	tenants, err := db.GetManager().TenantDao().GetALLTenants("")
	if err != nil {
		return nil, errors.Wrap(err, "get tenants")
	}

	var (
		appTeamIDs = make(map[string]string)
		teamNames  = make(map[string]string)
	)
	for _, app := range apps {
		appTeamIDs[app.AppID] = app.TenantID
	}
	for _, tenant := range tenants {
		teamNames[tenant.UUID] = tenant.Name
	}

	// Construct the returned data
	for _, plugin := range list.Items {
		appID := plugin.Labels["app_id"]
		status := "NIL"
		if appStatuses[appID] != "" {
			status = appStatuses[appID]
		}
		logrus.Debugf("plugin Name: %v, namespace %v", plugin.Name, plugin.Namespace)
		frontedRelativePath, _ := extractFilePath(plugin.Spec.FrontedPath)
		var pluginViews []string
		for _, view := range plugin.Spec.PluginView {
			pluginViews = append(pluginViews, view.String())
		}

		enableStatus := plugin.GetLabels()[v1alpha1.PluginEnableLabel.String()]
		if enableStatus == "" {
			enableStatus = v1alpha1.True.String()
		}
		plugins = append(plugins, &model.RainbondPlugins{
			RegionAppID:         appID,
			Name:                plugin.GetName(),
			TeamName:            teamNames[appTeamIDs[appID]],
			Icon:                plugin.Spec.Icon,
			Description:         plugin.Spec.Description,
			Version:             plugin.Spec.Version,
			Author:              "plugin.Spec.Author",
			Status:              status,
			Alias:               plugin.Spec.DisplayName,
			AccessURLs:          plugin.Spec.AccessURLs,
			Labels:              plugin.Labels,
			FrontedPath:         plugin.Spec.FrontedPath,
			FrontedRelativePath: frontedRelativePath,
			PluginType:          plugin.Spec.PluginType.String(),
			PluginViews:         pluginViews,
			EnableStatus:        enableStatus,
		})
	}
	return plugins, nil
}

func extractFilePath(frontedPath string) (string, error) {
	// 解析 frontedPath
	parsedURL, err := url.Parse(frontedPath)
	if err != nil {
		return "", err
	}

	// 返回路径部分，路径部分可能以 / 开头
	filePath := parsedURL.Path

	// 去掉路径前面的斜杠
	filePath = strings.TrimPrefix(filePath, "/")

	return filePath, nil
}

const (
	ComponentRainbondOperator = "rainbond-operator" // deployment
	ComponentRBDAPI           = "rbd-api"           // deployment
	ComponentRBDWorker        = "rbd-worker"        // deployment
	ComponentRBDAPPUI         = "rbd-app-ui"        // deployment, optional
	ComponentRBDChaos         = "rbd-chaos"         // daemonset
	ComponentRBDMQ            = "rbd-mq"            // deployment
)

func (c *clusterAction) ListUpgradeStatus() ([]model.ComponentStatus, error) {
	components := []string{ComponentRainbondOperator, ComponentRBDAPI, ComponentRBDWorker, ComponentRBDAPPUI, ComponentRBDChaos, ComponentRBDMQ}
	var statuses []model.ComponentStatus

	// 遍历所有组件并查询其状态
	for _, component := range components {
		var status model.ComponentStatus
		status.Name = component

		// 根据组件类型查询状态
		if component == ComponentRBDChaos {
			// DaemonSet 状态检查，包含重试机制
			dsStatus, err := retryStatusCheck(func() (model.ComponentStatus, error) {
				return c.checkDaemonSetStatus(component)
			})
			if err != nil {
				status.Status = "Failed"
				status.Message = err.Error()
			} else {
				status = dsStatus
			}
		} else {
			// 检查组件是否存在，ComponentRBDAPPUI 是可选的
			exists, err := c.checkComponentExists(component)
			if err != nil {
				// 处理组件存在性检查中的错误
				status.Status = "Failed"
				status.Message = fmt.Sprintf("error checking component existence: %v", err)
				statuses = append(statuses, status)
				continue
			}
			if !exists {
				// 如果组件不存在则跳过
				continue
			}
			// Deployment 状态检查，包含重试机制
			deployStatus, err := retryStatusCheck(func() (model.ComponentStatus, error) {
				return c.checkDeploymentStatus(component)
			})
			if err != nil {
				status.Status = "Failed"
				status.Message = err.Error()
			} else {
				status = deployStatus
			}
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

// 重试机制封装，防止短暂的错误导致失败
func retryStatusCheck(checkFunc func() (model.ComponentStatus, error)) (model.ComponentStatus, error) {
	var status model.ComponentStatus
	var err error
	retryErr := retry.OnError(retry.DefaultRetry, func(err error) bool {
		// Retry on any error
		return true
	}, func() error {
		status, err = checkFunc()
		return err
	})
	if retryErr != nil {
		return model.ComponentStatus{}, retryErr
	}
	return status, nil
}

// 检查组件是否存在（用于处理可选的组件，如 ComponentRBDAPPUI）
func (c *clusterAction) checkComponentExists(componentName string) (bool, error) {
	// 检查 Deployment 是否存在
	_, err := c.clientset.AppsV1().Deployments(c.namespace).Get(context.TODO(), componentName, metav1.GetOptions{})
	if err == nil {
		return true, nil
	} else if apierrors.IsNotFound(err) {
		// 如果返回 404 错误，说明该组件不存在
		return false, nil
	} else {
		// 其他类型的错误
		return false, fmt.Errorf("error checking component %s existence: %v", componentName, err)
	}
}

func (c *clusterAction) checkDeploymentStatus(deploymentName string) (model.ComponentStatus, error) {
	deployment, err := c.clientset.AppsV1().Deployments(c.namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return model.ComponentStatus{}, fmt.Errorf("error getting deployment %s: %v", deploymentName, err)
	}

	replicas := *deployment.Spec.Replicas                        // 总期望副本数
	updatedReplicas := deployment.Status.UpdatedReplicas         // 更新到新镜像的副本数
	availableReplicas := deployment.Status.AvailableReplicas     // 可用副本数
	unavailableReplicas := deployment.Status.UnavailableReplicas // 不可用副本数

	status := model.ComponentStatus{
		Name: deploymentName,
	}

	// 检查升级是否还在进行
	isProgressing := false
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentProgressing && condition.Status == corev1.ConditionTrue {
			isProgressing = true
			break
		}
	}

	// 如果 replicas 与 updatedReplicas 相等，且 unavailableReplicas 为 0，说明升级完成
	if updatedReplicas == replicas && unavailableReplicas == 0 {
		status.Status = "Completed"
		status.Progress = 100
	} else {
		status.Status = "Upgrading"
		// 更新进度：结合 updatedReplicas 和 availableReplicas 的比例
		logrus.Infof("updatedReplicas/replicas*50 [%f], int64(updatedReplicas/replicas*50) [%d]", float64(updatedReplicas)/float64(replicas)*50, int64(updatedReplicas*50/replicas))
		logrus.Infof("availableReplicas/replicas*50 [%f], int64(availableReplicas/replicas*50) [%d]", float64(availableReplicas)/float64(replicas)*50, int64(availableReplicas/replicas*50))
		progress := int64(updatedReplicas*50/replicas + availableReplicas*50/replicas)
		floatval := (float64(updatedReplicas)/float64(replicas))*50 + (float64(availableReplicas)/float64(replicas))*50
		logrus.Infof("deployment %s progress: %d, %f", deploymentName, progress, floatval)
		// 如果进度为 100，但是 unavailableReplicas 不等于 0，则说明升级卡住了
		if progress == 100 {
			progress = 99
		}
		status.Progress = progress

		// 如果有不可用副本，说明可能有问题
		if unavailableReplicas > 0 {
			// 获取不可用 Pod 的详细状态信息
			podMessage := c.getPodMessage(deploymentName)
			if podMessage != "" {
				status.Message = podMessage
			} else {
				status.Message = fmt.Sprintf("%d replicas are unavailable", unavailableReplicas)
			}
		}

		// 如果进展缓慢，或者长时间没有变化，可以增加错误提示
		if isProgressing {
			if status.Message == "" {
				status.Message = "Deployment is progressing, but not all replicas are ready"
			} else {
				status.Message += "; Deployment is progressing, but not all replicas are ready"
			}
		}
	}
	return status, nil
}

// 获取Pod的详细信息，展示包括创建中的信息和终止状态
func (c *clusterAction) getPodMessage(deploymentName string) string {
	// 根据Deployment的label来获取相关的Pod
	podList, err := c.clientset.CoreV1().Pods(c.namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("name=%s", deploymentName), // 假设使用name=deploymentName作为label
	})
	if err != nil {
		return fmt.Sprintf("Error listing pods for %s: %v", deploymentName, err)
	}

	var messages []string

	// 遍历所有Pod，检查每个Pod的状态
	for _, pod := range podList.Items {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			// 检查容器是否在等待状态，展示创建中的详细信息
			if containerStatus.State.Waiting != nil {
				messages = append(messages, fmt.Sprintf("Pod %s is in waiting state: %s", pod.Name, containerStatus.State.Waiting.Message))
			}
			// 检查容器是否处于终止状态且有错误
			if containerStatus.State.Terminated != nil && containerStatus.State.Terminated.ExitCode != 0 {
				messages = append(messages, fmt.Sprintf("Pod %s terminated with exit code %d: %s", pod.Name, containerStatus.State.Terminated.ExitCode, containerStatus.State.Terminated.Message))
			}
			// 检查容器是否处于Running但Not Ready状态
			if containerStatus.State.Running != nil && !containerStatus.Ready {
				messages = append(messages, fmt.Sprintf("Pod %s: Container %s is running but not ready", pod.Name, containerStatus.Name))
			}
		}
	}

	// 如果有详细状态信息，返回所有状态
	if len(messages) > 0 {
		return fmt.Sprintf("Pod details: %s", strings.Join(messages, "; "))
	}
	// 如果没有发现详细信息
	return ""
}

func (c *clusterAction) checkDaemonSetStatus(daemonsetName string) (model.ComponentStatus, error) {
	daemonset, err := c.clientset.AppsV1().DaemonSets(c.namespace).Get(context.TODO(), daemonsetName, metav1.GetOptions{})
	if err != nil {
		return model.ComponentStatus{}, fmt.Errorf("error getting daemonset %s: %v", daemonsetName, err)
	}

	desired := daemonset.Status.DesiredNumberScheduled
	updated := daemonset.Status.UpdatedNumberScheduled
	available := daemonset.Status.NumberAvailable

	status := model.ComponentStatus{
		Name: daemonsetName,
	}

	// 检查所有 pods 是否都更新并可用
	if updated == desired && available == desired {
		status.Status = "Completed"
		status.Progress = 100
	} else {
		status.Status = "Upgrading"
		progress := int64(available * 100 / desired)
		// 如果进度为 100，但是 available 与 desired 不相等，说明升级卡住了
		if progress == 100 {
			progress = 99
		}
		status.Progress = progress
		// 如果升级卡住，检查 pod 的错误信息
		podMessage := c.checkPodStatus(daemonsetName)
		if podMessage != "" {
			status.Message = podMessage
		}
	}
	return status, nil
}

func (c *clusterAction) checkPodStatus(componentName string) string {
	// 根据组件名称列出相关的 Pod
	podList, err := c.clientset.CoreV1().Pods(c.namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("name=%s", componentName),
	})
	if err != nil {
		return fmt.Sprintf("Error listing pods for %s: %v", componentName, err)
	}
	for _, pod := range podList.Items {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			// 检查容器是否处于等待或终止状态
			if containerStatus.State.Terminated != nil && containerStatus.State.Terminated.ExitCode != 0 {
				return fmt.Sprintf("Pod %s terminated with exit code: %d", pod.Name, containerStatus.State.Terminated.ExitCode)
			} else if containerStatus.State.Waiting != nil {
				return containerStatus.State.Waiting.Message
			}
		}
		// 检查 Pod 的健康状态
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status != corev1.ConditionTrue {
				return fmt.Sprintf("Pod %s is running but not ready: %s", pod.Name, condition.Message)
			}
		}
	}
	return ""
}

func (c *clusterAction) GetClusterRegionStatus() (map[string]interface{}, error) {
	secret := &corev1.Secret{}
	if err := c.client.Get(context.Background(), types.NamespacedName{Namespace: c.namespace, Name: "rbd-api-server-cert"}, secret); err != nil {
		return nil, err
	}
	var cluster rainbondv1alpha1.RainbondCluster
	if err := clients.RainbondKubeClient.Get(context.Background(), types.NamespacedName{Namespace: c.namespace, Name: "rainbondcluster"}, &cluster); err != nil {
		return nil, err
	}
	var gatewayIngressIP string
	if len(cluster.Spec.GatewayIngressIPs) > 0 && cluster.Spec.GatewayIngressIPs[0] != "" {
		gatewayIngressIP = cluster.Spec.GatewayIngressIPs[0]
	} else if len(cluster.Spec.NodesForGateway) > 0 {
		gatewayIngressIP = cluster.Spec.NodesForGateway[0].InternalIP
	}

	if secret != nil {
		var ips = strings.ReplaceAll(strings.Join(cluster.GatewayIngressIPs(), "-"), ".", "_")
		if availableips, ok := secret.Labels["availableips"]; ok && availableips == ips {
			caPem := secret.Data["ca.pem"]
			clientPem := secret.Data["server.pem"]
			clientKey := secret.Data["server.key.pem"]
			regionInfo := make(map[string]interface{})
			regionInfo["regionName"] = time.Now().Unix()
			regionInfo["regionType"] = []string{"custom"}
			regionInfo["sslCaCert"] = string(caPem)
			regionInfo["keyFile"] = string(clientKey)
			regionInfo["certFile"] = string(clientPem)
			regionInfo["url"] = fmt.Sprintf("https://%s:%s", gatewayIngressIP, "8443")
			regionInfo["wsUrl"] = fmt.Sprintf("ws://%s:%s", gatewayIngressIP, "6060")
			regionInfo["httpDomain"] = cluster.Spec.SuffixHTTPHost
			regionInfo["tcpDomain"] = cluster.GatewayIngressIP()
			regionInfo["desc"] = "Helm"
			regionInfo["regionAlias"] = "对接集群"
			regionInfo["provider"] = "helm"
			regionInfo["providerClusterId"] = ""
			regionInfo["token"] = os.Getenv("HELM_TOKEN")
			if os.Getenv("ENTERPRISE_ID") != "" {
				regionInfo["enterpriseId"] = os.Getenv("ENTERPRISE_ID")
			}
			if os.Getenv("CLOUD_SERVER") != "" {
				cloud := os.Getenv("CLOUD_SERVER")
				switch cloud {
				case "aliyun":
					regionInfo["regionType"] = []string{"aliyun"}
				case "huawei":
					regionInfo["regionType"] = []string{"huawei"}
				case "tencent":
					regionInfo["regionType"] = []string{"tencent"}
				}
			}
			return regionInfo, nil
		}
	}
	return nil, fmt.Errorf("get rbd-api-server-cert secret is nil")
}
