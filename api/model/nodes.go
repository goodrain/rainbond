package model

import (
	"time"

	"github.com/goodrain/rainbond/pkg/nodecert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeInfo -
type NodeInfo struct {
	CertificateSource     string                 `json:"certificate_source"`
	CertificateObservedAt string                 `json:"certificate_observed_at"`
	CertificateCheckedAt  string                 `json:"certificate_checked_at"`
	CertificateStatus     string                 `json:"certificate_status"`
	CertificateWarnings   []CertificateWarning   `json:"certificate_warnings,omitempty"`
	Certificates          []nodecert.Certificate `json:"certificates"`
	Name                  string                 `json:"name"`
	CreateTime            time.Time              `json:"create_time"`
	InternalIP            string                 `json:"internal_ip"`
	ExternalIP            string                 `json:"external_ip"`
	Roles                 []string               `json:"roles"` //compute, manage, storage, gateway
	Conditions            []NodeCondition        `json:"conditions"`
	Unschedulable         bool                   `json:"unschedulable"`
	ContainerRunTime      string                 `json:"container_run_time"`
	Architecture          string                 `json:"architecture"`
	KernelVersion         string                 `json:"kernel_version"`
	OperatingSystem       string                 `json:"operating_system"`
	OSVersion             string                 `json:"os_version"`
	Resource              Resource               `json:"resource"`
	// DiskStatsStatus distinguishes missing measurements from a healthy disk.
	DiskStatsStatus string `json:"disk_stats_status"`
}

// CertificateWarning preserves a native risk that has no exact parsed expiration.
type CertificateWarning struct {
	Reason     string `json:"reason"`
	ObservedAt string `json:"observed_at"`
}

type Resource struct {
	ReqCPU           float32 `json:"req_cpu"`
	CapCPU           int     `json:"cap_cpu"`
	ReqMemory        int     `json:"req_memory"`
	CapMemory        int     `json:"cap_memory"`
	ReqStorageEq     float32 `json:"req_storage_eq"`
	CapStorageEq     int     `json:"cap_storage_eq"`
	CapDisk          uint64  `json:"cap_disk"`
	ReqDisk          uint64  `json:"req_disk"`
	CapContainerDisk uint64  `json:"cap_container_disk"`
	ReqContainerDisk uint64  `json:"req_container_disk"`
}

type NodeCondition struct {
	// Type of node condition.
	Type string `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status string `json:"status"`
	// Last time we got an update on a given condition.
	LastHeartbeatTime metav1.Time `json:"last_heartbeat_time"`
	// Last time the condition transit from one status to another.
	LastTransitionTime metav1.Time `json:"last_transition_time"`
	// (brief) reason for the condition's last transition.
	Reason string `json:"reason"`
	// Human readable message indicating details about last transition.
	Message string `json:"message"`
}
