package controller

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/worker/appm/store"
	appmtypes "github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

type closedAppStore struct {
	store.Storer
}

func (closedAppStore) GetAppService(string) *appmtypes.AppService {
	return nil
}

func (closedAppStore) GetCrd(string) (*apiextensionsv1.CustomResourceDefinition, error) {
	return nil, nil
}

// capability_id: rainbond.lifecycle.stop-preserves-service
func TestStopPreservesGeneratedService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "java",
			Namespace: namespace.Name,
			Labels: map[string]string{
				"creator":    "Rainbond",
				"service_id": "service-1",
			},
		},
		Spec: corev1.ServiceSpec{ClusterIP: "10.0.0.8"},
	}
	client := k8sfake.NewSimpleClientset(namespace, service)
	logger := event.NewMockLogger(ctrl)
	logger.EXPECT().Info(gomock.Any(), gomock.Any())

	app := appmtypes.AppService{
		AppServiceBase: appmtypes.AppServiceBase{ServiceID: "service-1", ServiceAlias: "java"},
		Logger:         logger,
	}
	app.SetTenant(namespace)
	app.SetService(service)

	controller := &stopController{
		manager: &Manager{client: client, store: closedAppStore{}},
		ctx:     context.Background(),
	}
	if err := controller.stopOne(app); err != nil {
		t.Fatalf("stop component: %v", err)
	}

	got, err := client.CoreV1().Services(namespace.Name).Get(context.Background(), service.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected service to remain after stop: %v", err)
	}
	if got.Spec.ClusterIP != service.Spec.ClusterIP {
		t.Fatalf("expected cluster IP %q, got %q", service.Spec.ClusterIP, got.Spec.ClusterIP)
	}
}

func TestShouldDeleteManifestWithRuntimeClient(t *testing.T) {
	testCases := []struct {
		name     string
		manifest *unstructured.Unstructured
		want     bool
	}{
		{
			name: "nil manifest",
			want: false,
		},
		{
			name:     "skip kubevirt virtual machine",
			manifest: &unstructured.Unstructured{Object: map[string]interface{}{"kind": "VirtualMachine"}},
			want:     false,
		},
		{
			name:     "delete other custom resource",
			manifest: &unstructured.Unstructured{Object: map[string]interface{}{"kind": "VirtualService"}},
			want:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldDeleteManifestWithRuntimeClient(tc.manifest); got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
