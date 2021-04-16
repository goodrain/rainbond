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

package http

import (
	"context"

	"github.com/goodrain/rainbond/gateway/annotations/parser"
	"github.com/goodrain/rainbond/gateway/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	extensions "k8s.io/api/extensions/v1beta1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"

	api_meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"testing"
	"time"
)

func TestHttpDefault(t *testing.T) {
	clientSet, err := controller.NewClientSet("/Users/abe/go/src/github.com/goodrain/rainbond/test/admin.kubeconfig")
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
						{
							Name:            "default-pod2",
							Image:           "tomcat",
							ImagePullPolicy: "IfNotPresent",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 10010,
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
					Name:       "service-port",
					Port:       port,
					TargetPort: intstr.FromInt(10000),
				},
				{
					Name:       "service-port2",
					Port:       port,
					TargetPort: intstr.FromInt(10010),
				},
			},
			Selector: map[string]string{
				"tier": "default",
			},
		},
	}
	_ = ensureService(service, clientSet, t)
	time.Sleep(3 * time.Second)

	ingress := &extensions.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-ing",
			Namespace: ns.Name,
		},
		Spec: extensions.IngressSpec{
			Rules: []extensions.IngressRule{
				{
					Host: "www.http-router.com",
					IngressRuleValue: extensions.IngressRuleValue{
						HTTP: &extensions.HTTPIngressRuleValue{
							Paths: []extensions.HTTPIngressPath{
								{
									Path: "/http-router",
									Backend: extensions.IngressBackend{
										ServiceName: "default-svc",
										ServicePort: intstr.FromInt(80),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	_ = ensureIngress(ingress, clientSet, t)
}

func TestHttpCookie(t *testing.T) {
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
			Name:      "router-cookie-deploy",
			Namespace: ns.Name,
		},
		Spec: v1beta1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"tier": "router-cookie",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"tier": "router-cookie",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "router-cookie-pod",
							Image:           "tomcat",
							ImagePullPolicy: "IfNotPresent",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 10001,
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
	var port int32 = 30001
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "router-cookie-svc",
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
				"tier": "router-cookie",
			},
		},
	}
	_ = ensureService(service, clientSet, t)

	time.Sleep(3 * time.Second)

	ingress := &extensions.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "router-cookie-ing",
			Namespace: ns.Name,
			Annotations: map[string]string{
				parser.GetAnnotationWithPrefix("cookie"): "ck1:cv1;ck2:cv2;",
			},
		},
		Spec: extensions.IngressSpec{
			Rules: []extensions.IngressRule{
				{
					Host: "www.http-router.com",
					IngressRuleValue: extensions.IngressRuleValue{
						HTTP: &extensions.HTTPIngressRuleValue{
							Paths: []extensions.HTTPIngressPath{
								{
									Path: "/http-router",
									Backend: extensions.IngressBackend{
										ServiceName: "router-cookie-svc",
										ServicePort: intstr.FromInt(80),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	_ = ensureIngress(ingress, clientSet, t)
}

func TestHttpHeader(t *testing.T) {
	clientSet, err := controller.NewClientSet("/Users/abe/go/src/github.com/goodrain/rainbond/test/admin.kubeconfig")
	if err != nil {
		t.Errorf("can't create Kubernetes's client: %v", err)
	}

	ns := ensureNamespace(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gateway",
		},
	}, clientSet, t)
	if err != nil {
		t.Errorf("can't create namespace named %s: %v", "gateway", err)
	}

	// create deployment
	var replicas int32 = 3
	deploy := &v1beta1.Deployment{
		ObjectMeta: api_meta_v1.ObjectMeta{
			Name:      "router-header-deploy",
			Namespace: ns.Name,
		},
		Spec: v1beta1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"tier": "router-header",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"tier": "router-header",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "router-header-pod",
							Image:           "tomcat",
							ImagePullPolicy: "IfNotPresent",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 10002,
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
	var port int32 = 30002
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "router-header-svc",
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
				"tier": "router-header",
			},
		},
	}
	_ = ensureService(service, clientSet, t)

	time.Sleep(3 * time.Second)

	ingress := &extensions.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "router-header-ing",
			Namespace: ns.Name,
			Annotations: map[string]string{
				parser.GetAnnotationWithPrefix("header"): "hk1:hv1;hk2:hv2;",
			},
		},
		Spec: extensions.IngressSpec{
			Rules: []extensions.IngressRule{
				{
					Host: "www.http-router.com",
					IngressRuleValue: extensions.IngressRuleValue{
						HTTP: &extensions.HTTPIngressRuleValue{
							Paths: []extensions.HTTPIngressPath{
								{
									Path: "/http-router",
									Backend: extensions.IngressBackend{
										ServiceName: "router-header-svc",
										ServicePort: intstr.FromInt(80),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	_ = ensureIngress(ingress, clientSet, t)
}

func Test_ListIngress(t *testing.T) {
	clientSet, err := controller.NewClientSet("/Users/abe/go/src/github.com/goodrain/rainbond/test/admin.kubeconfig")
	if err != nil {
		t.Errorf("can't create Kubernetes's client: %v", err)
	}

	ings, err := clientSet.ExtensionsV1beta1().Ingresses("gateway").List(context.TODO(), api_meta_v1.ListOptions{})
	if err != nil {
		t.Fatalf("error listing ingresses: %v", err)
	}
	for _, ing := range ings.Items {
		t.Log(ing)
	}
}

func TestHttpUpstreamHashBy(t *testing.T) {
	clientSet, err := controller.NewClientSet("/Users/abe/go/src/github.com/goodrain/rainbond/test/admin.kubeconfig")
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
			Name:      "upstreamhashby-deploy",
			Namespace: ns.Name,
		},
		Spec: v1beta1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"tier": "upstreamhashby",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"tier": "upstreamhashby",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "upstreamhashby-pod",
							Image:           "tomcat",
							ImagePullPolicy: "IfNotPresent",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 10011,
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
	var port int32 = 30011
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "upstreamhashby-svc",
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
				"tier": "upstreamhashby",
			},
		},
	}
	_ = ensureService(service, clientSet, t)

	time.Sleep(3 * time.Second)

	ingress := &extensions.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "upstreamhashby-ing",
			Namespace: ns.Name,
			Annotations: map[string]string{
				parser.GetAnnotationWithPrefix("upstream-hash-by"): "$request_uri",
			},
		},
		Spec: extensions.IngressSpec{
			Rules: []extensions.IngressRule{
				{
					Host: "www.http-upstreamhashby.com",
					IngressRuleValue: extensions.IngressRuleValue{
						HTTP: &extensions.HTTPIngressRuleValue{
							Paths: []extensions.HTTPIngressPath{
								{
									Path: "/",
									Backend: extensions.IngressBackend{
										ServiceName: "upstreamhashby-svc",
										ServicePort: intstr.FromInt(80),
									},
								},
							},
						},
					},
				},
			},
		},
	}
	_ = ensureIngress(ingress, clientSet, t)
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

func ensureIngress(ingress *extensions.Ingress, clientSet kubernetes.Interface, t *testing.T) *extensions.Ingress {
	t.Helper()
	ing, err := clientSet.ExtensionsV1beta1().Ingresses(ingress.Namespace).Update(context.TODO(), ingress, metav1.UpdateOptions{})

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			t.Logf("Ingress %v not found, creating", ingress)

			ing, err = clientSet.ExtensionsV1beta1().Ingresses(ingress.Namespace).Create(context.TODO(), ingress, metav1.CreateOptions{})
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
