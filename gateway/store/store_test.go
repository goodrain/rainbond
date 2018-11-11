package store

import (
	"fmt"
	"github.com/eapache/channels"
	"k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	"time"

	//corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"sync/atomic"
	"testing"
)

func TestStore(t *testing.T) {
	clientSet := fake.NewSimpleClientset()

	t.Run("should return one event for add, update and delete of ingress", func(t *testing.T) {
		ns := createNamespace(clientSet, t)
		defer deleteNamespace(ns, clientSet, t)

		stopCh := make(chan struct{})
		updateCh := channels.NewRingChannel(1024)

		var add uint64

		storer := New(clientSet,
			ns,
			updateCh)

		storer.Run(stopCh)

		time.Sleep(5 * time.Second)

		go func(ch *channels.RingChannel) {
			for {
				evt, ok := <-ch.Out()
				if !ok {
					return
				}

				e := evt.(Event)
				if e.Obj == nil {
					continue
				}
				if _, ok := e.Obj.(*extensions.Ingress); !ok {
					continue
				}

				switch e.Type {
				case CreateEvent:
					atomic.AddUint64(&add, 1)
				case UpdateEvent:
					fmt.Print("update...\n")
				}
			}
		}(updateCh)

		ensureIngress(&extensions.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dummy",
				Namespace: ns,
				SelfLink:  fmt.Sprintf("/apis/extensions/v1beta1/namespaces/%s/ingresses/dummy", ns),
			},
			Spec: extensions.IngressSpec{
				Rules: []extensions.IngressRule{
					{
						Host: "dummy",
						IngressRuleValue: extensions.IngressRuleValue{
							HTTP: &extensions.HTTPIngressRuleValue{
								Paths: []extensions.HTTPIngressPath{
									{
										Path: "/",
										Backend: extensions.IngressBackend{
											ServiceName: "http-svc",
											ServicePort: intstr.FromInt(80),
										},
									},
								},
							},
						},
					},
				},
			},
		}, clientSet, t)

		time.Sleep(3 * time.Second)

		if atomic.LoadUint64(&add) != 1 {
			t.Errorf("expected 1 event of type Create but %v occurred", add)
		}
	})
}

func createNamespace(clientSet kubernetes.Interface, t *testing.T) string {
	t.Helper()
	t.Log("Creating temporal namespace")

	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "store-test",
		},
	}

	ns, err := clientSet.CoreV1().Namespaces().Create(namespace)
	if err != nil {
		t.Errorf("error creating the namespace: %v", err)
	}
	t.Logf("Temporal namespace %v created", ns)

	return ns.Name
}

func deleteNamespace(ns string, clientSet kubernetes.Interface, t *testing.T) {
	t.Helper()
	t.Logf("Deleting temporal namespace %v", ns)

	err := clientSet.CoreV1().Namespaces().Delete(ns, &metav1.DeleteOptions{})
	if err != nil {
		t.Errorf("error deleting the namespace: %v", err)
	}
	t.Logf("Temporal namespace %v deleted", ns)
}

func ensureIngress(ingress *extensions.Ingress, clientSet kubernetes.Interface, t *testing.T) *extensions.Ingress {
	t.Helper()
	ing, err := clientSet.Extensions().Ingresses(ingress.Namespace).Update(ingress)

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			t.Logf("Ingress %v not found, creating", ingress)

			ing, err = clientSet.Extensions().Ingresses(ingress.Namespace).Create(ingress)
			if err != nil {
				t.Fatalf("error creating ingress %+v: %v", ingress, err)
			}

			t.Logf("Ingress %+v created", ingress)
			return ing
		}

		t.Fatalf("error updating ingress %+v: %v", ingress, err)
	}

	t.Logf("Ingress %+v updated", ingress)

	return ing
}