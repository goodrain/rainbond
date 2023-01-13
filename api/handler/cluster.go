package handler

import (
	"context"
	"fmt"
	"github.com/goodrain/rainbond-operator/util/rbdutil"
	"github.com/goodrain/rainbond/api/client/prometheus"
	"github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/api/util/bcode"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	"github.com/goodrain/rainbond/util/constants"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	workerclient "github.com/goodrain/rainbond/worker/client"
	"github.com/goodrain/rainbond/worker/server/pb"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/disk"
	"github.com/sirupsen/logrus"
	"io"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/util/flushwriter"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"os"
	"runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	GetAppK8SResource(ctx context.Context, namespace, appID, name, resourceYaml, kind string) (dbmodel.K8sResource, *util.APIHandleError)
	UpdateAppK8SResource(ctx context.Context, namespace, appID, name, resourceYaml, kind string) (dbmodel.K8sResource, *util.APIHandleError)
	SyncAppK8SResources(ctx context.Context, resources *model.SyncResources) ([]*dbmodel.K8sResource, *util.APIHandleError)
	AppYamlResourceName(yamlResource model.YamlResource) (map[string]model.LabelResource, *util.APIHandleError)
	AppYamlResourceDetailed(yamlResource model.YamlResource, yamlImport bool) (model.ApplicationResource, *util.APIHandleError)
	AppYamlResourceImport(yamlResource model.YamlResource, components model.ApplicationResource) (model.AppComponent, *util.APIHandleError)
	RbdLog(w http.ResponseWriter, r *http.Request, podName string, follow bool) error
	GetRbdPods() (rbds []model.RbdResp, err error)
	CreateShellPod(regionName string) (pod *corev1.Pod, err error)
	DeleteShellPod(podName string) error
	ListPlugins() (rbds []*model.RainbondPlugins, err error)
	ListAbilities() (rbds []unstructured.Unstructured, err error)
	GetAbility(abilityID string) (rbd *unstructured.Unstructured, err error)
	UpdateAbility(abilityID string, ability *unstructured.Unstructured) error
	ListRainbondComponents(ctx context.Context) (res []*model.RainbondComponent, err error)
}

// NewClusterHandler -
func NewClusterHandler(clientset *kubernetes.Clientset, RbdNamespace, grctlImage string, config *rest.Config, mapper meta.RESTMapper, prometheusCli prometheus.Interface, rainbondClient versioned.Interface, statusCli *workerclient.AppRuntimeSyncClient, dynamicClient dynamic.Interface) ClusterHandler {
	return &clusterAction{
		namespace:      RbdNamespace,
		clientset:      clientset,
		config:         config,
		mapper:         mapper,
		grctlImage:     grctlImage,
		prometheusCli:  prometheusCli,
		rainbondClient: rainbondClient,
		statusCli:      statusCli,
		dynamicClient:  dynamicClient,
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
	for _, podNum := range podNumber.MetricData.MetricValues {
		instance = podNum.Metadata["instance"]
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

// GetNamespace Get namespace of the current cluster
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
	shellPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("shell-%v-", regionName),
			Namespace:    c.namespace,
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
	pod, err = c.clientset.CoreV1().Pods("rbd-system").Create(ctx, shellPod, metav1.CreateOptions{})
	if err != nil {
		logrus.Error("create shell pod error:", err)
		return nil, err
	}
	return pod, nil
}

// DeleteShellPod -
func (c *clusterAction) DeleteShellPod(podName string) (err error) {
	err = c.clientset.CoreV1().Pods("rbd-system").Delete(context.Background(), podName, metav1.DeleteOptions{})
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
	request := c.clientset.CoreV1().Pods("rbd-system").GetLogs(podName, &corev1.PodLogOptions{
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
	pods, err := c.clientset.CoreV1().Pods("rbd-system").List(context.Background(), metav1.ListOptions{})
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
	podList, err := c.clientset.CoreV1().Pods("rbd-system").List(ctx, metav1.ListOptions{
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
		appNameMap[appName] ++
		pods[appName] = append(pods[appName], pod)
		if pod.Status.Phase == "Running" {
			ComponentRunPods[appName] ++
		}
		ComponentAllPods[appName] ++
	}
	var appNames []string
	for name := range appNameMap{
		appNames = append(appNames, name)
	}
	// rainbond operator
	roPods, err := c.clientset.CoreV1().Pods("rbd-system").List(ctx, metav1.ListOptions{
		LabelSelector: fields.SelectorFromSet(map[string]string{
			"release": "rainbond-operator",
		}).String(),
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	pods["rainbond-operator"] = roPods.Items
	for _, ropod := range roPods.Items {
		if ropod.Status.Phase == "Running" {
			ComponentRunPods["rainbond-operator"] ++
		}
		ComponentAllPods["rainbond-operator"] ++
	}
	appNames = append(appNames, "rainbond-operator")
	for _, name := range appNames {
		res = append(res, &model.RainbondComponent{
			Name:    name,
			Pods:   pods[name],
			AllPods: ComponentAllPods[name],
			RunPods: ComponentRunPods[name],
		})
	}
	return res, nil
}

// ListPlugins -
func (c *clusterAction) ListPlugins() (plugins []*model.RainbondPlugins, err error) {
	ns := ""
	if os.Getenv("PluginNS") != "" {
		ns = os.Getenv("PluginNS")
	}
	list, err := c.rainbondClient.RainbondV1alpha1().RBDPlugins(ns).List(context.Background(), metav1.ListOptions{})
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
	for _, status := range statuses.AppStatuses {
		appStatuses[status.AppId] = status.Status
	}

	for _, plugin := range list.Items {
		appID := plugin.Labels["app_id"]
		status := "NIL"
		if appStatuses[appID] != "" {
			status = appStatuses[appID]
		}
		logrus.Infof("plugin Name: %v, namespace %v", plugin.Name, plugin.Namespace)
		plugins = append(plugins, &model.RainbondPlugins{
			RegionAppID: appID,
			Name:        plugin.Name,
			Namespace:   plugin.Namespace,
			Icon:        plugin.Spec.Icon,
			Description: plugin.Spec.Description,
			Version:     plugin.Spec.Version,
			Author:      plugin.Spec.Author,
			Status:      status,
		})
	}
	return plugins, nil
}

// ListAbilities -
func (c *clusterAction) ListAbilities() (rbds []unstructured.Unstructured, err error) {
	list, err := c.rainbondClient.RainbondV1alpha1().RBDAbilities(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "get rbd abilities")
	}
	groupKey := func(apiVersion, kind string) string {
		return fmt.Sprintf("%s-%s", strings.Replace(apiVersion, "/", "-", -1), kind)
	}

	var watchGroups = make(map[string]struct{})
	for _, ability := range list.Items {
		for _, group := range ability.Spec.WatchGroups {
			watchGroups[groupKey(group.APIVersion, group.Kind)] = struct{}{}
		}
	}

	for key := range watchGroups {
		split := strings.Split(key, "-")
		var group, version, kind string
		if len(split) == 3 {
			group = split[0]
			version = split[1]
			kind = split[2]
		}
		if len(split) == 2 {
			version = split[0]
			kind = split[1]
		}
		logrus.Infof("watch group: %v, %v, %v", group, version, kind)
		list, err := c.dynamicClient.Resource(schema.GroupVersionResource{
			Group:   group,
			Version: version,
			// TODO: 复数形式不一定是加s
			Resource: strings.ToLower(kind) + "s",
		}).Namespace(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "get watch group list")
		}
		rbds = append(rbds, list.Items...)
	}

	return rbds, nil
}

// GetAbility -
func (c *clusterAction) GetAbility(abilityID string) (*unstructured.Unstructured, error) {
	// abilityID: group-version-kind-name
	split := strings.SplitN(abilityID, "-", 4)
	var group, version, kind, name string
	if len(split) == 4 {
		group = split[0]
		version = split[1]
		kind = split[2]
		name = split[3]
	}
	if len(split) == 3 {
		version = split[0]
		kind = split[1]
		name = split[2]
	}
	logrus.Infof("get ability: %v, %v, %v, %v", group, version, kind, name)
	res := schema.GroupVersionResource{
		Group:   group,
		Version: version,
		// TODO: 根据不同资源转化对应的 Resource
		Resource: strings.ToLower(kind) + "es",
	}
	resource, err := c.dynamicClient.Resource(res).Namespace(metav1.NamespaceAll).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "get ability")
	}
	return resource, nil
}

// UpdateAbility -
func (c *clusterAction) UpdateAbility(abilityID string, ability *unstructured.Unstructured) error {
	// abilityID: group-version-kind-name
	split := strings.SplitN(abilityID, "-", 4)
	var group, version, kind, name string
	if len(split) == 4 {
		group = split[0]
		version = split[1]
		kind = split[2]
		name = split[3]
	}
	if len(split) == 3 {
		version = split[0]
		kind = split[1]
		name = split[2]
	}
	logrus.Infof("update ability: %v, %v, %v, %v", group, version, kind, name)
	res := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: strings.ToLower(kind) + "s",
	}
	_, err := c.dynamicClient.Resource(res).Namespace("rbd-system").Update(context.Background(), ability, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "update ability")
	}
	return nil
}
