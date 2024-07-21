package model

import (
	corev1 "k8s.io/api/core/v1"
)

// ClusterResource -
type ClusterResource struct {
	AllNode             int     `json:"all_node"`
	NotReadyNode        int     `json:"notready_node"`
	ComputeNode         int     `json:"compute_node"`
	NotReadyComputeNode int     `json:"notready_compute_node"`
	ManageNode          int     `json:"manage_node"`
	NotReadyManageNode  int     `json:"notready_manage_node"`
	EtcdNode            int     `json:"etcd_node"`
	NotReadyEtcdNode    int     `json:"notready_etcd_node"`
	Tenant              int     `json:"tenant"`
	CapCPU              int     `json:"cap_cpu"`
	CapMem              int     `json:"cap_mem"`
	CapGPU              int     `json:"cap_gpu"`
	ReqCPU              float32 `json:"req_cpu"`
	ReqMem              int     `json:"req_mem"`
	ReqGPU              float32 `json:"req_gpu"`
	RainbondReqMem      int     `json:"rbd_req_mem"` // Resources to embody rainbond scheduling
	RainbondReqCPU      float32 `json:"rbd_req_cpu"`
	RainbondReqGPU      float32 `json:"rbd_req_gpu"`
	CapDisk             uint64  `json:"cap_disk"`
	ReqDisk             uint64  `json:"req_disk"`
	Pods                int64   `json:"pods"`
	Components          int64   `json:"components"`
	Applications        int64   `json:"applications"`
	ResourceProxyStatus bool    `json:"resource_proxy_status"`
	K8sVersion          string  `json:"k8s_version"`
	NodeReady           int32   `json:"node_ready"`
	RunningPods         int     `json:"running_pods"`
}

// NodeResource is a collection of compute resource.
type NodeResource struct {
	MilliCPU         int64 `json:"milli_cpu"`
	Memory           int64 `json:"memory"`
	NvidiaGPU        int64 `json:"nvidia_gpu"`
	EphemeralStorage int64 `json:"ephemeral_storage"`
	// We store allowedPodNumber (which is Node.Status.Allocatable.Pods().Value())
	// explicitly as int, to avoid conversions and improve performance.
	AllowedPodNumber int `json:"allowed_pod_number"`
}

// NewResource creates a Resource from ResourceList
func NewResource(rl corev1.ResourceList) *NodeResource {
	r := &NodeResource{}
	r.Add(rl)
	return r
}

// Add adds ResourceList into Resource.
func (r *NodeResource) Add(rl corev1.ResourceList) {
	if r == nil {
		return
	}

	for rName, rQuant := range rl {
		switch rName {
		case corev1.ResourceCPU:
			r.MilliCPU += rQuant.MilliValue()
		case corev1.ResourceMemory:
			r.Memory += rQuant.Value()
		case corev1.ResourcePods:
			r.AllowedPodNumber += int(rQuant.Value())
		case corev1.ResourceEphemeralStorage:
			r.EphemeralStorage += rQuant.Value()
		}
	}
}

// ExceptionNode -
type ExceptionNode struct {
	Name          string `json:"name"`
	ExceptionType string `json:"exception_type"`
	Reason        string `json:"reason"`
}

// GatewayResource -
type GatewayResource struct {
	Name           string           `json:"name"`
	Namespace      string           `json:"namespace"`
	LoadBalancerIP []string         `json:"load_balancer_ip,omitempty"`
	NodePortIP     []string         `json:"node_port_ip,omitempty"`
	ListenerNames  []string         `json:"listener_names"`
	ProtocolPort   map[string]int32 `json:"protocol_port"`
}
