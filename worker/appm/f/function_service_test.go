package f

import (
	"context"
	"strings"
	"testing"

	appsv1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

// capability_id: rainbond.worker.appm.service-sync-errors
func TestApplyOneReturnsServiceSyncError(t *testing.T) {
	tests := []struct {
		name     string
		verb     string
		existing bool
		apiError error
	}{
		{
			name:     "get forbidden",
			verb:     "get",
			apiError: apierrors.NewForbidden(schema.GroupResource{Resource: "services"}, "vm-backend", nil),
		},
		{
			name:     "create forbidden",
			verb:     "create",
			apiError: apierrors.NewForbidden(schema.GroupResource{Resource: "services"}, "vm-backend", nil),
		},
		{
			name:     "update conflict",
			verb:     "update",
			existing: true,
			apiError: apierrors.NewConflict(schema.GroupResource{Resource: "services"}, "vm-backend", nil),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desired := testInternalService()
			objects := []runtime.Object{&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: desired.Namespace}}}
			if tt.existing {
				objects = append(objects, desired.DeepCopy())
			}
			client := fake.NewSimpleClientset(objects...)
			client.PrependReactor(tt.verb, "services", func(ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, tt.apiError
			})
			app := &appsv1.AppService{}
			app.SetTenant(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: desired.Namespace}})
			app.SetService(desired)

			err := ApplyOne(context.Background(), nil, client, app)
			if err == nil {
				t.Fatal("Service sync failure was reported as success")
			}
			if !strings.Contains(err.Error(), desired.Namespace+"/"+desired.Name) ||
				!strings.Contains(err.Error(), tt.apiError.Error()) {
				t.Fatalf("error must identify the failed Service and API cause, got %v", err)
			}
		})
	}
}

// capability_id: rainbond.worker.appm.service-sync-errors
func TestApplyOneReconcilesInternalService(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "create"
		if existing {
			name = "update"
		}
		t.Run(name, func(t *testing.T) {
			desired := testInternalService()
			objects := []runtime.Object{&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: desired.Namespace}}}
			if existing {
				old := desired.DeepCopy()
				old.Spec.ClusterIP = "10.43.1.10"
				old.Spec.Selector = map[string]string{"name": "old-backend"}
				old.Spec.Ports[0].Port = 8080
				objects = append(objects, old)
			}
			client := fake.NewSimpleClientset(objects...)
			app := &appsv1.AppService{}
			app.SetTenant(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: desired.Namespace}})
			app.SetService(desired)

			if err := ApplyOne(context.Background(), nil, client, app); err != nil {
				t.Fatalf("apply internal Service: %v", err)
			}
			got, err := client.CoreV1().Services(desired.Namespace).Get(context.Background(), desired.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("internal Service was not created: %v", err)
			}
			if got.Spec.Selector["name"] != "vm-backend" || len(got.Spec.Ports) != 1 || got.Spec.Ports[0].Port != 80 {
				t.Fatalf("Service does not route to desired backend and port: %+v", got.Spec)
			}
			if existing && got.Spec.ClusterIP != "10.43.1.10" {
				t.Fatalf("Service update changed allocated ClusterIP: %q", got.Spec.ClusterIP)
			}
		})
	}
}

func testInternalService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "vm-backend", Namespace: "team"},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"name": "vm-backend"},
			Ports:    []corev1.ServicePort{{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP}},
		},
	}
}
