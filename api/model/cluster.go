package model

import (
	corev1 "k8s.io/api/core/v1"
)

//ClusterResource -
type ClusterResource struct {
	AllNode                          int           `json:"all_node"`
	NotReadyNode                     int           `json:"notready_node"`
	ComputeNode                      int           `json:"compute_node"`
	Tenant                           int           `json:"tenant"`
	CapCPU                           int           `json:"cap_cpu"`
	CapMem                           int           `json:"cap_mem"`
	HealthCapCPU                     int           `json:"health_cap_cpu"`
	HealthCapMem                     int           `json:"health_cap_mem"`
	UnhealthCapCPU                   int           `json:"unhealth_cap_cpu"`
	UnhealthCapMem                   int           `json:"unhealth_cap_mem"`
	ReqCPU                           float32       `json:"req_cpu"`
	ReqMem                           int           `json:"req_mem"`
	RainbondReqMem                   int           `json:"rbd_req_mem"` // Resources to embody rainbond scheduling
	RainbondReqCPU                   float32       `json:"rbd_req_cpu"`
	HealthReqCPU                     float32       `json:"health_req_cpu"`
	HealthReqMem                     int           `json:"health_req_mem"`
	UnhealthReqCPU                   float32       `json:"unhealth_req_cpu"`
	UnhealthReqMem                   int           `json:"unhealth_req_mem"`
	CapDisk                          uint64        `json:"cap_disk"`
	ReqDisk                          uint64        `json:"req_disk"`
	MaxAllocatableMemoryNodeResource *NodeResource `json:"max_allocatable_memory_node_resource"`
	ResourceProxyStatus              bool          `json:"resource_proxy_status"`
	K8sVersion                       string        `json:"k8s_version"`
	NodeReady                        int32         `json:"node_ready"`
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
