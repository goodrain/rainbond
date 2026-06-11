package cluster

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/index"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/log"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/internal/model"
	"github.com/furutachiKurea/kb-adapter-rbdplugin/service/kbkit"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetClusterEvents 获取指定 KubeBlocks Cluster 的运维事件列表
//
// 事件数据来源于与 Cluster 关联的 OpsRequest 资源，按创建时间降序排序
func (s *Service) GetClusterEvents(ctx context.Context, serviceID string, pagination model.Pagination) (*model.PaginatedResult[model.EventItem], error) {
	pagination.Validate()

	cluster, err := kbkit.GetClusterByServiceID(ctx, s.client, serviceID)
	if err != nil {
		return nil, fmt.Errorf("get cluster by service_id %s: %w", serviceID, err)
	}

	allOps, err := kbkit.GetAllOpsRequestsByCluster(ctx, s.client, cluster.Namespace, cluster.Name)
	if err != nil {
		return nil, fmt.Errorf("get all opsrequests for cluster %s: %w", cluster.Name, err)
	}

	// 转换所有 OpsRequest 为 EventItem
	events := make([]model.EventItem, 0, len(allOps))
	for _, ops := range allOps {
		event := s.convertOpsRequestToEventItem(&ops)
		// 只保留 block mechanica 支持的 OpsType
		if event.OpsType == "" {
			continue
		}
		log.Debug("convert opsrequest to eventItem", log.Any("eventItem", event))
		events = append(events, event)
	}

	podEvents, err := s.getClusterPodEventItems(ctx, cluster)
	if err != nil {
		log.Warn("Failed to get cluster pod events",
			log.String("cluster", cluster.Name),
			log.String("namespace", cluster.Namespace),
			log.Err(err))
	} else {
		events = append(events, podEvents...)
	}

	// 按创建时间降序
	sort.Slice(events, func(i, j int) bool {
		return events[i].CreateTime > events[j].CreateTime
	})

	result := kbkit.Paginate(events, pagination.Page, pagination.PageSize)

	log.Debug("get paginated events",
		log.String("cluster", cluster.Name),
		log.Any("events", events),
		log.Int("page", pagination.Page),
		log.Int("pageSize", pagination.PageSize),
		log.Any("result", result),
	)

	return &model.PaginatedResult[model.EventItem]{
		Items: result,
		Total: len(events),
	}, nil
}

// getClusterPodEventItems 将数据库实例 Pod 的 Warning 事件纳入组件事件列表
func (s *Service) getClusterPodEventItems(ctx context.Context, cluster *kbappsv1.Cluster) ([]model.EventItem, error) {
	pods, err := s.getClusterPods(ctx, cluster)
	if err != nil {
		return nil, err
	}

	eventItems := make([]model.EventItem, 0)
	for _, pod := range pods {
		events, err := getRawPodEventsByIndex(ctx, s.client, pod.Name, cluster.Namespace)
		if err != nil {
			return nil, err
		}
		for i := range events {
			eventItem, ok := convertPodEventToEventItem(pod.Name, &events[i])
			if !ok {
				continue
			}
			eventItems = append(eventItems, eventItem)
		}
	}

	return eventItems, nil
}

func getRawPodEventsByIndex(ctx context.Context, c client.Client, podName, namespace string) ([]corev1.Event, error) {
	var eventList corev1.EventList

	indexKey := fmt.Sprintf("%s/%s", namespace, podName)
	if err := c.List(ctx, &eventList, client.MatchingFields{index.NamespacePodNameField: indexKey}); err != nil {
		log.Warn("Index query for pod events failed",
			log.String("indexKey", indexKey),
			log.String("pod", podName),
			log.String("namespace", namespace),
			log.Err(err))
		return []corev1.Event{}, nil
	}

	return eventList.Items, nil
}

func convertPodEventToEventItem(podName string, event *corev1.Event) (model.EventItem, bool) {
	if event.Type != corev1.EventTypeWarning {
		return model.EventItem{}, false
	}

	eventTime := podEventTimestamp(event)
	return model.EventItem{
		OpsName:     fmt.Sprintf("%s/%s", podName, event.Name),
		OpsType:     podEventType(event),
		UserName:    "system",
		Status:      "failure",
		FinalStatus: "complete",
		Message:     event.Message,
		Reason:      event.Reason,
		CreateTime:  formatTimeWithOffset(eventTime),
		EndTime:     formatTimeWithOffset(eventTime),
	}, true
}

func podEventTimestamp(event *corev1.Event) time.Time {
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp.Time
	}
	if !event.FirstTimestamp.IsZero() {
		return event.FirstTimestamp.Time
	}
	if !event.EventTime.IsZero() {
		return event.EventTime.Time
	}
	return event.CreationTimestamp.Time
}

func podEventType(event *corev1.Event) string {
	if event.Reason != "" {
		return event.Reason
	}
	return event.Type
}

// convertOpsRequestToEventItem 将 OpsRequest 转换为 EventItem
func (s *Service) convertOpsRequestToEventItem(opsRequest *opsv1alpha1.OpsRequest) model.EventItem {
	var message, reason, status, finalStatus, endTime string

	if !opsRequest.Status.CompletionTimestamp.IsZero() {
		endTime = formatTimeWithOffset(opsRequest.Status.CompletionTimestamp.Time)
	}

	switch opsRequest.Status.Phase {
	case opsv1alpha1.OpsSucceedPhase:
		status = "success"
		finalStatus = "complete"
		message = "Operation completed successfully"
	case opsv1alpha1.OpsFailedPhase:
		status = "failure"
		finalStatus = "complete"
		// 优先从 condition 中获取详细失败信息
		if cond := findFailedCondition(opsRequest.Status.Conditions); cond != nil {
			message = cond.Message
			reason = cond.Reason
		} else {
			message = "Operation failed with unknown reason"
		}
	case opsv1alpha1.OpsCancelledPhase:
		status = "failure"
		finalStatus = "complete"
		message = "Operation was cancelled"
	case opsv1alpha1.OpsAbortedPhase:
		status = "failure"
		finalStatus = "complete"
		message = "Operation was aborted"
	case opsv1alpha1.OpsPendingPhase:
		status = "pending"
		finalStatus = "running"
		message = "Operation is pending"
	case opsv1alpha1.OpsCreatingPhase:
		status = "running"
		finalStatus = "running"
		message = "Operation is being created"
	case opsv1alpha1.OpsRunningPhase:
		status = "running"
		finalStatus = "running"
		message = "Operation is running"
	case opsv1alpha1.OpsCancellingPhase:
		status = "cancelling"
		finalStatus = "running"
		message = "Operation is being cancelled"
	default:
		status = "unknown"
		finalStatus = "running"
		message = "Operation status unknown"
	}

	return model.EventItem{
		OpsName:     opsRequest.Name,
		OpsType:     toRainbondOptType(opsRequest.Spec.Type),
		UserName:    "system",
		Status:      status,
		FinalStatus: finalStatus,
		Message:     message,
		Reason:      reason,
		CreateTime:  formatTimeWithOffset(opsRequest.CreationTimestamp.Time),
		EndTime:     endTime,
	}
}

// toRainbondOptType 将 OpsType 转换为 Rainbond 支持的 OpsType 的 string 值
//
// 忽略会与 Rainbond event 重复的 OpsType，只保留 KubeBlocks 特有的事件类型
func toRainbondOptType(opsType opsv1alpha1.OpsType) string {
	switch opsType {
	case opsv1alpha1.VerticalScalingType:
		// Vertical Scaling
		return "vertical-service"
	case opsv1alpha1.HorizontalScalingType:
		// Horizontal Scaling
		return "horizontal-service"
	case opsv1alpha1.VolumeExpansionType:
		// Storage Expansion
		return "update-service-volume"
	case opsv1alpha1.BackupType:
		return "backup-database"
	case opsv1alpha1.ReconfiguringType:
		return "reconfiguring-cluster"
	case opsv1alpha1.RestoreType:
		return "restore-database"
	default:
		return ""
	}
}

// findFailedCondition 查找失败状态的 Condition
func findFailedCondition(conditions []metav1.Condition) *metav1.Condition {
	for _, cond := range conditions {
		if cond.Status == metav1.ConditionFalse {
			return &cond
		}
	}
	return nil
}

// formatTimeWithOffset 将时间格式化为带数字时区偏移的 RFC3339 格式
// 形如: 2025-09-09T16:51:59+08:00
func formatTimeWithOffset(t time.Time) string {
	localTime := t.In(time.Local)
	return localTime.Format(time.RFC3339)
}
