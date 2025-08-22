package testutil

import (
	"context"
	"errors"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	datav1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// _scheme 包含所有测试需要的 API 类型
var _scheme = func() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = kbappsv1.AddToScheme(s)
	_ = datav1alpha1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = storagev1.AddToScheme(s)
	_ = opv1alpha1.AddToScheme(s)
	return s
}()

// errorClient 用于模拟错误情况的客户端, 始终返回错误
type errorClient struct {
	client.Client
}

// Create 实现 Create 方法返回错误
func (e *errorClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return errors.New("create failed")
}

// List 实现 List 方法返回错误
func (e *errorClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return errors.New("list failed")
}

// NewFakeClient
func NewFakeClient(objs ...client.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(_scheme).
		WithObjects(objs...).
		Build()
}

// NewErrorClient
func NewErrorClient() client.Client {
	return &errorClient{}
}

// ClientSetup setup 函数
type ClientSetup func() client.Client

// NewNormalClientSetup 普通客户端的 setup 函数
func NewNormalClientSetup(objs ...client.Object) ClientSetup {
	return func() client.Client {
		return NewFakeClient(objs...)
	}
}

// NewErrorClientSetup 错误客户端的 setup 函数
func NewErrorClientSetup() ClientSetup {
	return func() client.Client {
		return NewErrorClient()
	}
}
