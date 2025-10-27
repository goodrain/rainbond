package model

import (
	"fmt"
	"strconv"
	"strings"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	datav1alpha "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	Hourly BackupFrequency = "hourly"
	Daily  BackupFrequency = "daily"
	Weekly BackupFrequency = "weekly"
)

// BackupFrequency 备份频率类型
type BackupFrequency string

// ClusterInput 创建集群请求 - 组合了创建集群需要的所有信息
type ClusterInput struct {
	ClusterInfo
	ClusterResource
	ClusterBackup
	RBDService RBDService `json:"rbdService"`
}

// ExpansionInput 集群扩容请求
type ExpansionInput struct {
	RBDService
	ClusterResource
}

// ClusterInfo KubeBlocks Cluster 的基本信息
type ClusterInfo struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Type         string `json:"type"`
	Version      string `json:"version"`
	StorageClass string `json:"storageClass"`

	// TerminationPolicy 删除策略
	//
	// 尽管在 rbd-app-ui 中的设置项是与备份数据的删除策略，但实际上是 Cluster.spec 中的内容，故放在 ClusterInfo 中
	//
	// https://kubeblocks.io/docs/preview/user_docs/references/api-reference/cluster#apps.kubeblocks.io/v1.TerminationPolicyType
	TerminationPolicy kbappsv1.TerminationPolicyType `json:"terminationPolicy"`
}

// RBDService rainbond service 标识
type RBDService struct {
	ServiceID string `json:"service_id" param:"service-id"`
}

// ClusterBackup KubeBlocks Cluster 的备份相关配置
type ClusterBackup struct {
	BackupRepo      string                      `json:"backupRepo"`
	Schedule        BackupSchedule              `json:"schedule"`
	RetentionPeriod datav1alpha.RetentionPeriod `json:"retentionPeriod"`
}

// ClusterResource 创建 KubeBlocks Cluster 时的资源规格
type ClusterResource struct {
	CPU      string `json:"cpu"`
	Memory   string `json:"memory"`
	Storage  string `json:"storage"`
	Replicas int32  `json:"replicas"`
}

// ParsedResources 解析后的资源信息
type ParsedResources struct {
	CPU     resource.Quantity
	Memory  resource.Quantity
	Storage resource.Quantity
}

// ParseResources 解析字符串形式的资源配置为 resource.Quantity 类型
func (cr *ClusterResource) ParseResources() (*ParsedResources, error) {
	cpuQuantity, err := resource.ParseQuantity(cr.CPU)
	if err != nil {
		return nil, fmt.Errorf("invalid CPU quantity %q: %w", cr.CPU, err)
	}

	memoryQuantity, err := resource.ParseQuantity(cr.Memory)
	if err != nil {
		return nil, fmt.Errorf("invalid memory quantity %q: %w", cr.Memory, err)
	}

	storageQuantity, err := resource.ParseQuantity(cr.Storage)
	if err != nil {
		return nil, fmt.Errorf("invalid storage quantity %q: %w", cr.Storage, err)
	}

	return &ParsedResources{
		CPU:     cpuQuantity,
		Memory:  memoryQuantity,
		Storage: storageQuantity,
	}, nil
}

// BackupSchedule backup schedule 配置
type BackupSchedule struct {
	Frequency BackupFrequency `json:"frequency"`
	DayOfWeek int32           `json:"dayOfWeek"`
	Hour      int32           `json:"hour"`
	Minute    int32           `json:"minute"`
}

// Cron 生成 cron 表达式
func (s *BackupSchedule) Cron() string {
	switch s.Frequency {
	case Hourly:
		return fmt.Sprintf("%d * * * *", s.Minute)
	case Daily:
		return fmt.Sprintf("%d %d * * *", s.Minute, s.Hour)
	case Weekly:
		return fmt.Sprintf("%d %d * * %d", s.Minute, s.Hour, s.DayOfWeek)
	default:
		return ""
	}
}

// Uncron 从 cron 表达式解析成 BackupSchedule,
// 支持 hourly, daily, weekly
func (s *BackupSchedule) Uncron(cronExpr string) error {
	if cronExpr == "" {
		return fmt.Errorf("empty cron expression")
	}

	// 简单的解析逻辑，假设格式正确
	// 实际项目中可能需要使用更复杂的 cron 解析库
	parts := strings.Fields(cronExpr)
	if len(parts) != 5 {
		return fmt.Errorf("invalid cron expression format: %s", cronExpr)
	}

	minute := parts[0]
	hour := parts[1]
	dayOfWeek := parts[4]

	// 解析分钟
	if minute == "*" {
		s.Minute = 0
	} else {
		if m, err := strconv.Atoi(minute); err == nil {
			s.Minute = int32(m)
		} else {
			return fmt.Errorf("invalid minute in cron: %s", minute)
		}
	}

	// 解析小时
	if hour == "*" {
		s.Hour = 0
	} else {
		if h, err := strconv.Atoi(hour); err == nil {
			s.Hour = int32(h)
		} else {
			return fmt.Errorf("invalid hour in cron: %s", hour)
		}
	}

	// 解析星期几
	if dayOfWeek == "*" {
		s.DayOfWeek = 0
		s.Frequency = Daily
	} else {
		if d, err := strconv.Atoi(dayOfWeek); err == nil {
			s.DayOfWeek = int32(d)
			s.Frequency = Weekly
		} else {
			return fmt.Errorf("invalid day of week in cron: %s", dayOfWeek)
		}
	}

	// 判断频率类型
	if s.DayOfWeek > 0 {
		s.Frequency = Weekly
	} else if s.Hour > 0 {
		s.Frequency = Daily
	} else {
		s.Frequency = Hourly
	}

	return nil
}

// ConnectInfo 数据库连接信息
type ConnectInfo struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

// ClusterDetail Cluster 的详细信息
type ClusterDetail struct {
	Basic    BasicInfo             `json:"basic"`
	Resource ClusterResourceStatus `json:"resource"`
	Backup   BackupInfo            `json:"backup"`
}

// BasicInfo Cluster 的基本信息
type BasicInfo struct {
	ClusterInfo
	RBDService
	Status             ClusterStatus `json:"status"`
	Replicas           []Status      `json:"replicas"`
	IsSupportBackup    bool          `json:"support_backup"`
	IsSupportParameter bool          `json:"support_parameter"`
}

// ClusterResourceStatus Cluster 的实际资源状态信息
type ClusterResourceStatus struct {
	// CPU：m（毫核）
	CPUMilli int64 `json:"cpu"`

	// 内存：Mi（兆字节）
	MemoryMi int64 `json:"memory"`

	// 磁盘：Gi（吉字节）
	StorageGi int64 `json:"storage"`

	// 副本数
	Replicas int32 `json:"replicas"`
}

// BackupInfo Cluster 的备份信息
type BackupInfo struct {
	ClusterBackup
}

// ClusterStatus Cluster 的状态信息
type ClusterStatus struct {
	Status   string `json:"status"`
	StatusCN string `json:"status_cn"`
	// StartTime Cluster 最后一次进入 Running/Ready 的时间，ISO 8601 格式（UTC）
	StartTime string `json:"start_time,omitempty"`
}

// Status 副本状态信息
type Status struct {
	Name       string             `json:"name"`
	Component  string             `json:"component,omitempty"`
	Status     corev1.PodPhase    `json:"status"`
	Ready      bool               `json:"ready"`
	Containers []ReplicaContainer `json:"containers,omitempty"`
}

// ReplicaContainer 副本中的容器
type ReplicaContainer struct {
	Name string `json:"name"`
}

// ComponentName 组件名称
type ComponentName string

// ExpansionContext 伸缩操作的上下文
//
// Components 中记录了各个组件的 Desire Status，支持多组件不同规格伸缩
type ExpansionContext struct {
	Cluster    *kbappsv1.Cluster
	Components map[ComponentName]ComponentExpansionContext // 组件名称 -> 伸缩操作的上下文
}

// ComponentExpansionContext 单个组件伸缩操作的上下文
type ComponentExpansionContext struct {
	// 水平伸缩
	CurrentReplicas int32
	DesiredReplicas int32

	// 垂直伸缩
	CurrentCPU resource.Quantity
	CurrentMem resource.Quantity
	DesiredCPU resource.Quantity
	DesiredMem resource.Quantity

	// 存储扩容
	HasPVC          bool
	VolumeTplName   string
	CurrentStorage  resource.Quantity
	DesiredStorage  resource.Quantity
	StorageClassRef *string
}

// HorizontalScalingOpsParams 用于水平伸缩的 OpsRequest
// 支持多组件同规格伸缩
type HorizontalScalingOpsParams struct {
	Cluster    *kbappsv1.Cluster
	Components []ComponentHorizontalScaling
}

// ComponentHorizontalScaling 单个组件水平伸缩
type ComponentHorizontalScaling struct {
	Name          string
	DeltaReplicas int32
}

// VerticalScalingOpsParams 用于垂直伸缩的 OpsRequest
// 支持多组件同规格伸缩
type VerticalScalingOpsParams struct {
	Cluster    *kbappsv1.Cluster
	Components []ComponentVerticalScaling
}

// ComponentVerticalScaling 单个组件垂直伸缩
type ComponentVerticalScaling struct {
	Name   string
	CPU    resource.Quantity
	Memory resource.Quantity
}

// VolumeExpansionOpsParams 用于存储扩容的 OpsRequest
// 支持多组件同规格扩容
type VolumeExpansionOpsParams struct {
	Cluster    *kbappsv1.Cluster
	Components []ComponentVolumeExpansion
}

// ComponentVolumeExpansion 单个组件存储扩容
type ComponentVolumeExpansion struct {
	Name                    string
	VolumeClaimTemplateName string
	Storage                 resource.Quantity
}

type BatchOperationResult struct {
	Succeeded []string         `json:"succeeded"`
	Failed    map[string]error `json:"failed"`
}

func NewBatchOperationResult() *BatchOperationResult {
	return &BatchOperationResult{
		Succeeded: make([]string, 0),
		Failed:    make(map[string]error),
	}
}

func (result *BatchOperationResult) AddSucceeded(serviceID string) {
	result.Succeeded = append(result.Succeeded, serviceID)
}

func (result *BatchOperationResult) AddFailed(serviceID string, err error) {
	result.Failed[serviceID] = err
}

// PodDetail 用于适配 Rainbond 中的 Instance
//
// 形如：
//
//	"bean": {
//	       "name": "pod-xxx-abcdef",
//	       "node_ip": "10.0.0.1",
//	       "start_time": "2023-07-20T08:21:00Z",
//	       "ip": "172.20.0.2",
//	       "version": "v1.2.3",
//
//	      "namespace": "default",
//
//	      "status": {
//	        "type_str": "running",
//	        "reason": "ContainersNotReady",
//	        "message": "Waiting for container to start",
//	        "advice": "OutOfMemory"
//	      },
//	      "containers": [
//	        {
//		   	   "component_def": "postgresql-12-1.0.0", // 来自 cluster 而不是从 pod 获取
//	          "limit_memory": "512Mi",
//	          "limit_cpu": "0.5",
//	          "started": "2023-07-20T08:22:00Z",
//	          "state": "Running",
//	          "reason": ""
//	        }
//	      ],
//	      "events": [
//	        {
//	          "type": "Normal",
//	          "reason": "Pulled",
//	          "age": "5m",
//	          "message": "Successfully pulled image"
//	        }
//	      ]
//	    }
//	  }
type PodDetail struct {
	Name       string      `json:"name"`
	NodeIP     string      `json:"node_ip"`
	StartTime  string      `json:"start_time"`
	IP         string      `json:"ip"`
	Version    string      `json:"version"` //  Cluster.spec.componentSpecs.componentDef:componentDef: postgresql-12-1.0.0
	Namespace  string      `json:"namespace"`
	Status     PodStatus   `json:"status"`
	Containers []Container `json:"containers"`
	Events     []PodEvent  `json:"events"`
}

// PodStatus 当前 Pod 的状态
type PodStatus struct {
	TypeStr string `json:"type_str"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Advice  string `json:"advice"`
}

// Container Pod 中的 Container 信息
type Container struct {
	ComponentDef string `json:"component_def"`
	LimitMemory  string `json:"limit_memory"`
	LimitCPU     string `json:"limit_cpu"`
	Started      string `json:"started"`
	State        string `json:"state"`
	Reason       string `json:"reason"`
}

// PodEvent Pod 中的 Event 信息
type PodEvent struct {
	Type    string `json:"type"`
	Reason  string `json:"reason"`
	Age     string `json:"age"`
	Message string `json:"message"`
}

// EventItem 用于适配 Rainbond 的操作记录
type EventItem struct {
	OpsName     string `json:"event_id"`
	OpsType     string `json:"opt_type"`
	UserName    string `json:"user_name,omitempty"`
	Status      string `json:"status"`
	FinalStatus string `json:"final_status"`
	Message     string `json:"message"`
	Reason      string `json:"reason"`
	CreateTime  string `json:"create_time"`
	EndTime     string `json:"end_time"`
}
