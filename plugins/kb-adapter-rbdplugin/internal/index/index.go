// Package index 用于在 controller-runtime 中注册字段索引
package index

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	datav1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// ServiceIDLabel 通过此键关联 KubeBlocks Component 和 Cluster
	ServiceIDLabel = "service_id"

	// InstanceLabel 用于索引 Pod 和 Backup 的 instance
	InstanceLabel = "app.kubernetes.io/instance"

	// ServiceIDField 通过 service_id 索引 KubeBlocks Component 和 Cluster
	ServiceIDField = "index.serviceID"

	// NamespaceInstanceField namespace/instance，
	// 用于索引 Cluster 的 Pod（Cluster 副本）和 Backup
	NamespaceInstanceField = "index.namespace.instance"

	// NamespaceClusterOpsTypeField namespace/cluster/opsType，
	// 用于索引 OpsRequest
	NamespaceClusterOpsTypeField = "index.namespace.cluster.opsType"

	// NamespaceClusterComponentField namespace/cluster/component，
	// 用于索引 InstanceSet
	NamespaceClusterComponentField = "index.namespace.cluster.component"

	// NamespacePodNameField namespace/podName，
	// 用于索引 Pod 相关的 Event
	NamespacePodNameField = "index.namespace.podName"
)

// Register 在缓存注册字段索引。
func Register(ctx context.Context, mgr ctrl.Manager) error {
	indexer := mgr.GetFieldIndexer()

	// 为 KubeBlocks Cluster 按 service_id 建立索引
	if err := indexer.IndexField(ctx, &kbappsv1.Cluster{}, ServiceIDField, func(obj client.Object) []string {
		labels := obj.GetLabels()
		if labels == nil {
			return nil
		}
		if v, ok := labels[ServiceIDLabel]; ok && v != "" {
			return []string{v}
		}
		return nil
	}); err != nil {
		return err
	}

	// 为 Deployment 按 service_id 建立索引
	if err := indexer.IndexField(ctx, &appsv1.Deployment{}, ServiceIDField, func(obj client.Object) []string {
		labels := obj.GetLabels()
		if labels == nil {
			return nil
		}
		if v, ok := labels[ServiceIDLabel]; ok && v != "" {
			return []string{v}
		}
		return nil
	}); err != nil {
		return err
	}

	// 为 pod 按 namespace/instance 建立索引
	if err := indexer.IndexField(ctx, &corev1.Pod{}, NamespaceInstanceField, func(obj client.Object) []string {
		pod := obj.(*corev1.Pod)
		if instance, ok := pod.Labels[InstanceLabel]; ok && instance != "" {
			return []string{fmt.Sprintf("%s/%s", pod.Namespace, instance)}
		}
		return nil
	}); err != nil {
		return err
	}

	// 为 backup 按 namespace/instance 建立索引
	if err := indexer.IndexField(ctx, &datav1alpha1.Backup{}, NamespaceInstanceField, func(obj client.Object) []string {
		backup := obj.(*datav1alpha1.Backup)
		if instance, ok := backup.Labels[InstanceLabel]; ok && instance != "" {
			return []string{fmt.Sprintf("%s/%s", backup.Namespace, instance)}
		}
		return nil
	}); err != nil {
		return err
	}

	// 为 opsrequest 按 namespace/clusterName/opsType 建立索引
	if err := indexer.IndexField(ctx, &opv1alpha1.OpsRequest{}, NamespaceClusterOpsTypeField, func(obj client.Object) []string {
		ops := obj.(*opv1alpha1.OpsRequest)
		return []string{fmt.Sprintf("%s/%s/%s", ops.Namespace, ops.Spec.ClusterName, ops.Spec.Type)}
	}); err != nil {
		return err
	}

	// 为 InstanceSet 按 namespace/cluster/component 建立索引
	if err := indexer.IndexField(ctx, &workloadsv1.InstanceSet{}, NamespaceClusterComponentField, func(obj client.Object) []string {
		instanceSet := obj.(*workloadsv1.InstanceSet)
		if clusterName, ok := instanceSet.Labels[InstanceLabel]; ok && clusterName != "" {
			if componentName, ok := instanceSet.Labels["apps.kubeblocks.io/component-name"]; ok {
				return []string{fmt.Sprintf("%s/%s/%s", instanceSet.Namespace, clusterName, componentName)}
			}
		}
		return nil
	}); err != nil {
		return err
	}

	// 为 Event 按 namespace/podName 建立索引
	if err := indexer.IndexField(ctx, &corev1.Event{}, NamespacePodNameField, func(obj client.Object) []string {
		event := obj.(*corev1.Event)
		if event.InvolvedObject.Kind == "Pod" && event.InvolvedObject.Name != "" {
			return []string{fmt.Sprintf("%s/%s", event.Namespace, event.InvolvedObject.Name)}
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}
