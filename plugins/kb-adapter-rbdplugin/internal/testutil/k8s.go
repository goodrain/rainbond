package testutil

import (
	"context"
	"fmt"

	"github.com/furutachiKurea/block-mechanica/internal/index"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	datav1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// _scheme 包含所有测试需要的 API 类型
var _scheme = func() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = kbappsv1.AddToScheme(s)
	_ = datav1alpha1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = appsv1.AddToScheme(s)
	_ = storagev1.AddToScheme(s)
	_ = opv1alpha1.AddToScheme(s)
	_ = parametersv1alpha1.AddToScheme(s)
	_ = workloadsv1.AddToScheme(s)
	return s
}()

// NewFakeClient 创建支持字段索引的测试客户端（统一入口）
func NewFakeClient(objs ...client.Object) client.Client {
	return NewFakeClientWithIndexes(objs...)
}

// NewFakeClientWithIndexes 创建一个支持字段索引的测试客户端
func NewFakeClientWithIndexes(objs ...client.Object) client.Client {
	builder := fake.NewClientBuilder().WithScheme(_scheme).WithObjects(objs...)

	// 添加 Cluster 的 service_id 索引
	builder = builder.WithIndex(&kbappsv1.Cluster{}, index.ServiceIDField, func(obj client.Object) []string {
		labels := obj.GetLabels()
		if labels == nil {
			return nil
		}
		if v, ok := labels[index.ServiceIDLabel]; ok && v != "" {
			return []string{v}
		}
		return nil
	})

	// 添加 Deployment 的 service_id 索引
	builder = builder.WithIndex(&appsv1.Deployment{}, index.ServiceIDField, func(obj client.Object) []string {
		labels := obj.GetLabels()
		if labels == nil {
			return nil
		}
		if v, ok := labels[index.ServiceIDLabel]; ok && v != "" {
			return []string{v}
		}
		return nil
	})

	// 添加 OpsRequest 的 namespace/cluster/opsType 索引
	builder = builder.WithIndex(&opv1alpha1.OpsRequest{}, index.NamespaceClusterOpsTypeField, func(obj client.Object) []string {
		opsRequest := obj.(*opv1alpha1.OpsRequest)
		if opsRequest.Spec.ClusterName != "" {
			return []string{fmt.Sprintf("%s/%s/%s", opsRequest.Namespace, opsRequest.Spec.ClusterName, opsRequest.Spec.Type)}
		}
		return nil
	})

	// 添加 Backup 的 namespace/instance 索引
	builder = builder.WithIndex(&datav1alpha1.Backup{}, index.NamespaceInstanceField, func(obj client.Object) []string {
		backup := obj.(*datav1alpha1.Backup)
		if instance, ok := backup.Labels[index.InstanceLabel]; ok && instance != "" {
			return []string{fmt.Sprintf("%s/%s", backup.Namespace, instance)}
		}
		return nil
	})

	// 添加 Pod 的 namespace/instance 索引
	builder = builder.WithIndex(&corev1.Pod{}, index.NamespaceInstanceField, func(obj client.Object) []string {
		pod := obj.(*corev1.Pod)
		if instance, ok := pod.Labels[index.InstanceLabel]; ok && instance != "" {
			return []string{fmt.Sprintf("%s/%s", pod.Namespace, instance)}
		}
		return nil
	})

	// 添加 InstanceSet 的 namespace/cluster/component 索引
	builder = builder.WithIndex(&workloadsv1.InstanceSet{}, index.NamespaceClusterComponentField, func(obj client.Object) []string {
		instanceSet := obj.(*workloadsv1.InstanceSet)
		if clusterName, ok := instanceSet.Labels[index.InstanceLabel]; ok && clusterName != "" {
			if componentName, ok := instanceSet.Labels["apps.kubeblocks.io/component-name"]; ok {
				return []string{fmt.Sprintf("%s/%s/%s", instanceSet.Namespace, clusterName, componentName)}
			}
		}
		return nil
	})

	// 添加 Pod 事件的 namespace/pod 索引
	builder = builder.WithIndex(&corev1.Event{}, index.NamespacePodNameField, func(obj client.Object) []string {
		event := obj.(*corev1.Event)
		if event.InvolvedObject.Kind == "Pod" && event.InvolvedObject.Name != "" {
			return []string{fmt.Sprintf("%s/%s", event.Namespace, event.InvolvedObject.Name)}
		}
		return nil
	})

	return builder.Build()
}

// CreateObjects 创建多个对象
func CreateObjects(ctx context.Context, c client.Client, objs []client.Object) error {
	for _, obj := range objs {
		if err := c.Create(ctx, obj); err != nil {
			return err
		}
	}
	return nil
}

// ErrorClientBuilder 允许按需配置操作失败的 fake client
type ErrorClientBuilder struct {
	client         client.Client
	createErr      error
	createTypeErrs map[string]error // 按类型的 Create 错误
	listErr        error
	getErr         error
	updateErr      error
	patchErr       error
	deleteErr      error
	deleteAllOfErr error
}

// NewErrorClientBuilder 创建可配置失败行为的 client builder
func NewErrorClientBuilder(objs ...client.Object) *ErrorClientBuilder {
	return &ErrorClientBuilder{
		client:         NewFakeClient(objs...),
		createTypeErrs: make(map[string]error),
	}
}

// WithCreateError 指定 Create 操作的错误
func (b *ErrorClientBuilder) WithCreateError(err error) *ErrorClientBuilder {
	b.createErr = err
	return b
}

// WithCreateErrorForType 为特定类型的对象指定 Create 错误
func (b *ErrorClientBuilder) WithCreateErrorForType(obj client.Object, err error) *ErrorClientBuilder {
	typeName := fmt.Sprintf("%T", obj)
	b.createTypeErrs[typeName] = err
	return b
}

// WithListError 指定 List 操作的错误
func (b *ErrorClientBuilder) WithListError(err error) *ErrorClientBuilder {
	b.listErr = err
	return b
}

// WithGetError 指定 Get 操作的错误
func (b *ErrorClientBuilder) WithGetError(err error) *ErrorClientBuilder {
	b.getErr = err
	return b
}

// WithUpdateError 指定 Update 操作的错误
func (b *ErrorClientBuilder) WithUpdateError(err error) *ErrorClientBuilder {
	b.updateErr = err
	return b
}

// WithPatchError 指定 Patch 操作的错误
func (b *ErrorClientBuilder) WithPatchError(err error) *ErrorClientBuilder {
	b.patchErr = err
	return b
}

// WithDeleteError 指定 Delete 操作的错误
func (b *ErrorClientBuilder) WithDeleteError(err error) *ErrorClientBuilder {
	b.deleteErr = err
	return b
}

// WithDeleteAllOfError 指定 DeleteAllOf 操作的错误
func (b *ErrorClientBuilder) WithDeleteAllOfError(err error) *ErrorClientBuilder {
	b.deleteAllOfErr = err
	return b
}

// Build 构建最终的 client
func (b *ErrorClientBuilder) Build() client.Client {
	return &errorClient{
		Client:         b.client,
		createErr:      b.createErr,
		createTypeErrs: b.createTypeErrs,
		listErr:        b.listErr,
		getErr:         b.getErr,
		updateErr:      b.updateErr,
		patchErr:       b.patchErr,
		deleteErr:      b.deleteErr,
		deleteAllOfErr: b.deleteAllOfErr,
	}
}

type errorClient struct {
	client.Client
	createErr      error
	createTypeErrs map[string]error
	listErr        error
	getErr         error
	updateErr      error
	patchErr       error
	deleteErr      error
	deleteAllOfErr error
}

// Create 实现 Create 方法，根据配置返回错误
func (f *errorClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	// 如果指定了类型特定的错误，则返回该错误
	typeName := fmt.Sprintf("%T", obj)
	if err, ok := f.createTypeErrs[typeName]; ok {
		return err
	}

	if f.createErr != nil {
		return f.createErr
	}
	return f.Client.Create(ctx, obj, opts...)
}

// List 实现 List 方法，根据配置返回错误
func (f *errorClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if f.listErr != nil {
		return f.listErr
	}
	return f.Client.List(ctx, list, opts...)
}

// Get 实现 Get 方法，根据配置返回错误
func (f *errorClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	if f.getErr != nil {
		return f.getErr
	}
	return f.Client.Get(ctx, key, obj, opts...)
}

// Update 实现 Update 方法，根据配置返回错误
func (f *errorClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	return f.Client.Update(ctx, obj, opts...)
}

// Patch 实现 Patch 方法，根据配置返回错误
func (f *errorClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if f.patchErr != nil {
		return f.patchErr
	}
	return f.Client.Patch(ctx, obj, patch, opts...)
}

// Delete 实现 Delete 方法，根据配置返回错误
func (f *errorClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	return f.Client.Delete(ctx, obj, opts...)
}

// DeleteAllOf 实现 DeleteAllOf 方法，根据配置返回错误
func (f *errorClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	if f.deleteAllOfErr != nil {
		return f.deleteAllOfErr
	}
	return f.Client.DeleteAllOf(ctx, obj, opts...)
}
