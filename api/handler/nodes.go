package handler

import (
	"context"
	"fmt"
	"github.com/goodrain/rainbond/api/client/prometheus"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/shirou/gopsutil/disk"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"runtime"
	"strings"

	"github.com/goodrain/rainbond/api/model"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"time"
)

const (
	// NodeRolesLabelPrefix -
	NodeRolesLabelPrefix = "node-role.kubernetes.io"
	// NodeInternalIP -
	NodeInternalIP = "InternalIP"
	// NodeExternalIP -
	NodeExternalIP = "ExternalIP"
	// UnSchedulAble-
	UnSchedulAble = "unschedulable"
	// ReSchedulAble -
	ReSchedulAble = "reschedulable"
	// NodeUp -
	NodeUp = "up"
	// NodeDown -
	NodeDown = "down"
	// Evict -
	Evict = "evict"
	//EvictionKind -
	EvictionKind = "Eviction"
	//EvictionSubresource -
	EvictionSubresource = "pods/eviction"
)

// NodesHandler -
type NodesHandler interface {
	GetNodes(ctx context.Context) ([]model.NodeInfo, error)
	GetNodeInfo(ctx context.Context, nodeName string) (model.NodeInfo, error)
	NodeAction(ctx context.Context, nodeName, action string) error
	GetLabels(ctx context.Context, nodeName string) (map[string]string, error)
	UpdateLabels(ctx context.Context, nodeName string, labels map[string]string) (map[string]string, error)
	GetTaints(ctx context.Context, nodeName string) ([]v1.Taint, error)
	UpdateTaints(ctx context.Context, nodeName string, taints []v1.Taint) ([]v1.Taint, error)
}

// NewNodesHandler -
func NewNodesHandler(clientset *kubernetes.Clientset, RbdNamespace string, config *rest.Config, mapper meta.RESTMapper, prometheusCli prometheus.Interface) NodesHandler {
	return &nodesHandle{
		namespace:     RbdNamespace,
		clientset:     clientset,
		config:        config,
		mapper:        mapper,
		prometheusCli: prometheusCli,
	}
}

type nodesHandle struct {
	namespace        string
	clientset        *kubernetes.Clientset
	clusterInfoCache *model.ClusterResource
	cacheTime        time.Time
	config           *rest.Config
	mapper           meta.RESTMapper
	client           client.Client
	prometheusCli    prometheus.Interface
}

//GetNodes -
func (n *nodesHandle) GetNodes(ctx context.Context) (res []model.NodeInfo, err error) {
	nodeList, err := n.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("get node list error: %v", err)
		return nil, err
	}
	for _, node := range nodeList.Items {
		nodeinfo, err := n.HandleNodeInfo(node)
		if err != nil {
			logrus.Error("get node info handle error:", err)
			return res, err
		}
		res = append(res, nodeinfo)
	}
	return res, nil
}

//GetNodeInfo -
func (n *nodesHandle) GetNodeInfo(ctx context.Context, nodeName string) (res model.NodeInfo, err error) {
	node, err := n.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		logrus.Error("get node info error:", err)
		return res, err
	}
	res, err = n.HandleNodeInfo(*node)
	if err != nil {
		logrus.Error("get node info handle error:", err)
		return res, err
	}
	var diskstauts *disk.UsageStat
	if runtime.GOOS != "windows" {
		diskstauts, _ = disk.Usage("/")
	} else {
		diskstauts, _ = disk.Usage(`z:\\`)
	}
	var diskCap, reqDisk  uint64
	if diskstauts != nil {
		diskCap = diskstauts.Total
		reqDisk = diskstauts.Used
	}
	res.Resource.CapDisk = diskCap
	res.Resource.ReqDisk = reqDisk
	return res, nil
}

// HandleNode -
func (n *nodesHandle) HandleNodeInfo(node v1.Node) (nodeinfo model.NodeInfo, err error) {
	var condition model.NodeCondition
	for _, addr := range node.Status.Addresses {
		switch addr.Type {
		case NodeInternalIP:
			nodeinfo.InternalIP = addr.Address
		case NodeExternalIP:
			nodeinfo.ExternalIP = addr.Address
		default:
			continue
		}
	}
	for _, cond := range node.Status.Conditions {
		condition.Type = string(cond.Type)
		condition.Status = string(cond.Status)
		condition.Message = cond.Message
		condition.Reason = cond.Reason
		condition.LastHeartbeatTime = cond.LastHeartbeatTime
		condition.LastTransitionTime = cond.LastTransitionTime
		nodeinfo.Conditions = append(nodeinfo.Conditions, condition)
	}
	// get node roles
	var roles []string
	for k, v := range node.Labels {
		if strings.HasPrefix(k, NodeRolesLabelPrefix) && v == "true" {
			// string handle : node-role.kubernetes.io/worker: "true"
			role := strings.Split(strings.Split(k, "/")[1], ":")[0]
			roles = append(roles, role)
		}
		continue
	}
	// req resource from Prometheus
	var query string
	query = fmt.Sprintf(`sum(rbd_api_exporter_cluster_pod_memory{node_name="%v"})`, node.Name)
	podMemoryMetric := n.prometheusCli.GetMetric(query, time.Now())

	query = fmt.Sprintf(`sum(rbd_api_exporter_cluster_pod_cpu{node_name="%v"})`, node.Name)
	podCPUMetric := n.prometheusCli.GetMetric(query, time.Now())

	query = fmt.Sprintf(`sum(rbd_api_exporter_cluster_pod_ephemeral_storage{node_name="%v"})`, node.Name)
	podEphemeralStorageMetric := n.prometheusCli.GetMetric(query, time.Now())

	for i, memory := range podMemoryMetric.MetricData.MetricValues {
		nodeinfo.Resource.ReqMemory = int(memory.Sample.Value()) / 1024 / 1024
		nodeinfo.Resource.ReqCPU = float32(podCPUMetric.MetricData.MetricValues[i].Sample.Value()) / 1000
		nodeinfo.Resource.ReqStorageEq = float32(podEphemeralStorageMetric.MetricData.MetricValues[i].Sample.Value()) / 1024 / 1024
	}
	// cap resource
	nodeinfo.Resource.CapMemory = int(node.Status.Capacity.Memory().Value()) / 1024 / 1024
	nodeinfo.Resource.CapCPU = int(node.Status.Capacity.Cpu().Value())
	nodeinfo.Resource.CapStorageEq = int(node.Status.Capacity.StorageEphemeral().Value()) / 1024 / 1024

	nodeinfo.Name = node.Name
	nodeinfo.Unschedulable = node.Spec.Unschedulable
	nodeinfo.KernelVersion = node.Status.NodeInfo.KernelVersion
	nodeinfo.ContainerRunTime = node.Status.NodeInfo.ContainerRuntimeVersion
	nodeinfo.Architecture = node.Status.NodeInfo.Architecture
	nodeinfo.OperatingSystem = node.Status.NodeInfo.OperatingSystem
	nodeinfo.CreateTime = node.CreationTimestamp.Time
	nodeinfo.OSVersion = node.Status.NodeInfo.OSImage
	nodeinfo.Roles = roles

	return nodeinfo, nil
}

//NodeAction -
func (n *nodesHandle) NodeAction(ctx context.Context, nodeName, action string) error {
	switch action {
	case UnSchedulAble:
		// true can't scheduler ,false can scheduler
		data := fmt.Sprintf(`{"spec":{"unschedulable":%t}}`, true)
		_, err := n.clientset.CoreV1().Nodes().Patch(ctx, nodeName, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
		if err != nil {
			logrus.Error("unschedulable error:", err)
			return err
		}
	case ReSchedulAble:
		// true can't scheduler ,false can scheduler
		data := fmt.Sprintf(`{"spec":{"unschedulable":%t}}`, false)
		_, err := n.clientset.CoreV1().Nodes().Patch(ctx, nodeName, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
		if err != nil {
			logrus.Error("reschedulable error:", err)
			return err
		}
	case Evict:
		// unschedulable
		data := fmt.Sprintf(`{"spec":{"unschedulable":%t}}`, true)
		_, err := n.clientset.CoreV1().Nodes().Patch(ctx, nodeName, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
		if err != nil {
			logrus.Error("unschedulable error:", err)
			return err
		}
		err = n.DeleteOrEvictPodsSimple(nodeName)
		if err != nil {
			logrus.Error("delete or evict pods error:", err)
		}
	default:
		logrus.Info("not support this action")
	}
	return nil
}

//DeleteOrEvictPodsSimple Evict the Pod from a node
func (n *nodesHandle) DeleteOrEvictPodsSimple(nodeName string) error {
	nodePods, err := n.GetNodePods(nodeName)
	if err != nil {
		logrus.Infof("get pods of node %s failed ", nodeName)
		return err
	}
	policyGroupVersion, err := n.SupportEviction()
	if err != nil {
		return err
	}
	if policyGroupVersion == "" {
		return fmt.Errorf("the server can not support eviction subresource")
	}
	for _, v := range nodePods {
		n.evictPod(v, policyGroupVersion)
	}
	return nil
}

// SupportEviction uses Discovery API to find out if the server support eviction subresource
// If support, it will return its groupVersion; Otherwise, it will return ""
func (n *nodesHandle) SupportEviction() (string, error) {
	discoveryClient := n.clientset.Discovery()
	groupList, err := discoveryClient.ServerGroups()
	if err != nil {
		return "", err
	}
	foundPolicyGroup := false
	var policyGroupVersion string
	for _, group := range groupList.Groups {
		if group.Name == "policy" {
			foundPolicyGroup = true
			policyGroupVersion = group.PreferredVersion.GroupVersion
			break
		}
	}
	if !foundPolicyGroup {
		return "", nil
	}
	resourceList, err := discoveryClient.ServerResourcesForGroupVersion("v1")
	if err != nil {
		return "", err
	}
	for _, resource := range resourceList.APIResources {
		if resource.Name == EvictionSubresource && resource.Kind == EvictionKind {
			return policyGroupVersion, nil
		}
	}
	return "", nil
}

// GetNodePods -
func (n *nodesHandle) GetNodePods(nodeName string) (pods []v1.Pod, err error) {
	podList, err := n.clientset.CoreV1().Pods(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": nodeName}).String()})
	if err != nil {
		return nil, err
	}
	return podList.Items, err
}

// GetNodeScheduler Scheduler status
func (n *nodesHandle) GetNodeScheduler(ctx context.Context, nodeName string) (status bool, err error) {
	node, err := n.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		logrus.Error("get node scheduler status error:", err)
		return false, err
	}
	return node.Spec.Unschedulable, err
}

//evictPod -
func (n *nodesHandle) evictPod(pod v1.Pod, policyGroupVersion string) error {
	deleteOptions := &metav1.DeleteOptions{}
	eviction := &v1beta1.Eviction{
		TypeMeta: metav1.TypeMeta{
			APIVersion: policyGroupVersion,
			Kind:       EvictionKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		DeleteOptions: deleteOptions,
	}
	// Remember to change change the URL manipulation func when Evction's version change
	return n.clientset.PolicyV1beta1().Evictions(eviction.Namespace).Evict(context.Background(), eviction)
}

// GetLabels -
func (n *nodesHandle) GetLabels(ctx context.Context, nodeName string) (map[string]string, error) {
	node, err := n.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		logrus.Error("[GetLabels] get node error:", err)
		return nil, err
	}
	return node.Labels, err
}

// UpdateLabels -
func (n *nodesHandle) UpdateLabels(ctx context.Context, nodeName string, labels map[string]string) (map[string]string, error) {
	labelsByte, err := ffjson.Marshal(labels)
	if err != nil {
		return nil, err
	}
	data := fmt.Sprintf(`{"metadata":{"labels":%s}}`, string(labelsByte))
	node, err := n.clientset.CoreV1().Nodes().Patch(ctx, nodeName, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
	if err != nil {
		logrus.Error("[UpdateLabels] update node labels error:", err)
		return nil, err
	}
	return node.Labels, nil
}

// GetTaint -
func (n *nodesHandle) GetTaints(ctx context.Context, nodeName string) ([]v1.Taint, error) {
	node, err := n.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		logrus.Error("[GetTaint] get node error:", err)
		return nil, err
	}
	return node.Spec.Taints, err
}

// UpdateTaint -
func (n *nodesHandle) UpdateTaints(ctx context.Context, nodeName string, taints []v1.Taint) ([]v1.Taint, error) {
	taintsByte, err := ffjson.Marshal(taints)
	if err != nil {
		return nil, err
	}
	data := fmt.Sprintf(`{"spec":{"taints":%s}}`, string(taintsByte))
	node, err := n.clientset.CoreV1().Nodes().Patch(ctx, nodeName, types.StrategicMergePatchType, []byte(data), metav1.PatchOptions{})
	if err != nil {
		logrus.Error("[UpdateTaints] update node taints error:", err)
		return nil, err
	}
	return node.Spec.Taints, err
}
