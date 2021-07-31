// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package tcp

import (
	"context"
	"testing"
	"time"

	"github.com/goodrain/rainbond/gateway/annotations/parser"
	"github.com/goodrain/rainbond/gateway/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	api_meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func TestTcp(t *testing.T) {
	clientSet, err := controller.NewClientSet("/Users/abe/Documents/admin.kubeconfig")
	if err != nil {
		t.Errorf("can't create Kubernetes's client: %v", err)
	}

	ns := ensureNamespace(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gateway",
		},
	}, clientSet, t)

	// create deployment
	var replicas int32 = 3
	deploy := &v1beta1.Deployment{
		ObjectMeta: api_meta_v1.ObjectMeta{
			Name:      "default-deploy",
			Namespace: ns.Name,
		},
		Spec: v1beta1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"tier": "default",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"tier": "default",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "default-pod",
							Image:           "tomcat",
							ImagePullPolicy: "IfNotPresent",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 10000,
								},
							},
						},
					},
				},
			},
		},
	}
	_ = ensureDeploy(deploy, clientSet, t)

	// create service
	var port int32 = 30000
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-svc",
			Namespace: ns.Name,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: "service-port",
					Port: port,
				},
			},
			Selector: map[string]string{
				"tier": "default",
			},
		},
	}
	_ = ensureService(service, clientSet, t)
	time.Sleep(3 * time.Second)

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tcp-ing",
			Namespace: ns.Name,
			Annotations: map[string]string{
				parser.GetAnnotationWithPrefix("l4-enable"): "true",
				parser.GetAnnotationWithPrefix("l4-host"):   "127.0.0.1",
				parser.GetAnnotationWithPrefix("l4-port"):   "32145",
			},
		},
		Spec: networkingv1.IngressSpec{
			DefaultBackend: &networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: "default-svc",
					Port: networkingv1.ServiceBackendPort{
						Number: 30000,
					},
				},
			},
		},
	}
	ensureIngress(ingress, clientSet, t)
}

func ensureNamespace(ns *corev1.Namespace, clientSet kubernetes.Interface, t *testing.T) *corev1.Namespace {
	t.Helper()
	n, err := clientSet.CoreV1().Namespaces().Update(context.TODO(), ns, metav1.UpdateOptions{})

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			t.Logf("Namespace %v not found, creating", ns)

			n, err = clientSet.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("error creating namespace %+v: %v", ns, err)
			}

			t.Logf("Namespace %+v created", ns)
			return n
		}

		t.Fatalf("error updating namespace %+v: %v", ns, err)
	}

	t.Logf("Namespace %+v updated", ns)

	return n
}

func ensureDeploy(deploy *v1beta1.Deployment, clientSet kubernetes.Interface, t *testing.T) *v1beta1.Deployment {
	t.Helper()
	dm, err := clientSet.ExtensionsV1beta1().Deployments(deploy.Namespace).Update(context.TODO(), deploy, metav1.UpdateOptions{})

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			t.Logf("Deployment %v not found, creating", deploy)

			dm, err = clientSet.ExtensionsV1beta1().Deployments(deploy.Namespace).Create(context.TODO(), deploy, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("error creating deployment %+v: %v", deploy, err)
			}

			t.Logf("Deployment %+v created", deploy)
			return dm
		}

		t.Fatalf("error updating ingress %+v: %v", deploy, err)
	}

	t.Logf("Deployment %+v updated", deploy)

	return dm
}

func ensureService(service *corev1.Service, clientSet kubernetes.Interface, t *testing.T) *corev1.Service {
	t.Helper()
	clientSet.CoreV1().Services(service.Namespace).Delete(context.TODO(), service.Name, metav1.DeleteOptions{})

	svc, err := clientSet.CoreV1().Services(service.Namespace).Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("error creating service %+v: %v", service, err)
	}

	t.Logf("Service %+v created", service)
	return svc
}

func ensureIngress(ingress *networkingv1.Ingress, clientSet kubernetes.Interface, t *testing.T) *networkingv1.Ingress {
	t.Helper()
	ing, err := clientSet.NetworkingV1().Ingresses(ingress.Namespace).Update(context.TODO(), ingress, metav1.UpdateOptions{})

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			t.Logf("Ingress %v not found, creating", ingress)

			ing, err = clientSet.NetworkingV1().Ingresses(ingress.Namespace).Create(context.TODO(), ingress, metav1.CreateOptions{})
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
