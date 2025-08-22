package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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

// BackupSchedule
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
	Status   ClusterStatus `json:"status"`
	Replicas []PodStatus   `json:"replicas"`
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

// PodStatus 副本状态信息
type PodStatus struct {
	Name   string          `json:"name"`
	Status corev1.PodPhase `json:"status"`
	Ready  bool            `json:"ready"`
}

// BackupItem 备份项信息
type BackupItem struct {
	Name   string                  `json:"name"`
	Status datav1alpha.BackupPhase `json:"status"`
	Time   time.Time               `json:"time"`
}

// ExpansionContext 伸缩操作的上下文，包含所有相关字段
type ExpansionContext struct {
	Cluster       *kbappsv1.Cluster
	ComponentName string

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
type HorizontalScalingOpsParams struct {
	Cluster       *kbappsv1.Cluster
	ComponentName string
	DeltaReplicas int32
}

// VerticalScalingOpsParams 用于垂直伸缩的 OpsRequest
type VerticalScalingOpsParams struct {
	Cluster       *kbappsv1.Cluster
	ComponentName string
	CPU           resource.Quantity
	Memory        resource.Quantity
}

// VolumeExpansionOpsParams 用于存储扩容的 OpsRequest
type VolumeExpansionOpsParams struct {
	Cluster                 *kbappsv1.Cluster
	ComponentName           string
	VolumeClaimTemplateName string
	Storage                 resource.Quantity
}
