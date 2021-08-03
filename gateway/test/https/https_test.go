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

package https

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

const (
	tlsCrt string = `-----BEGIN CERTIFICATE-----
MIIDnjCCAoYCCQDNpEw8d114VjANBgkqhkiG9w0BAQsFADCBjzELMAkGA1UEBhMC
Q04xDzANBgNVBAgMBlBla2luZzEPMA0GA1UEBwwGUGVraW5nMREwDwYDVQQKDAhH
b29kcmFpbjELMAkGA1UECwwCSVQxGTAXBgNVBAMMEHd3dy5nb29kcmFpbi5jb20x
IzAhBgkqhkiG9w0BCQEWFGh1YW5ncmhAZ29vZHJhaW4uY29tMCAXDTE4MTAxOTA2
MzczMVoYDzIxMTgwOTI1MDYzNzMxWjCBjzELMAkGA1UEBhMCQ04xDzANBgNVBAgM
BlBla2luZzEPMA0GA1UEBwwGUGVraW5nMREwDwYDVQQKDAhHb29kcmFpbjELMAkG
A1UECwwCSVQxGTAXBgNVBAMMEHd3dy5nb29kcmFpbi5jb20xIzAhBgkqhkiG9w0B
CQEWFGh1YW5ncmhAZ29vZHJhaW4uY29tMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A
MIIBCgKCAQEA6II9+hrrlbVRtNSsy8vpBqP59eOQQ5eaLsGL9D4Gdx6CELw24DXQ
YzAmznDQMKdUn0QavdoVgpXtjJQ1ExG6JqM44Kg87+hjraGKGGcVO+h2ThkjkUUP
Aq2tkuoNgc6JcAk0zSeq5cC/Z4WT1s/gM555XwmAsFnujW33EM77t/c9qcaX7Gqi
CrpcGg+PViYPutf0KuKjPfWqCDCoqlAsZy8cBEwPxnAJ3JE+HrnjR7CQ+C7wiAyB
Bm3JKzHEa2V7QRXelJ02VRl7VdpwJCBajAYPwa9CeWkv5JuO4Y1mGQ9suiFCb3hc
lVTviGnSkR6plo8cBUIAewVcC3ogV3KwGwIDAQABMA0GCSqGSIb3DQEBCwUAA4IB
AQAJxEH8Zk8dZ/ZeRIroz6TQnuhjnoNvu4Wz/8M+EEb+B50JMq9miIau0MkCPKX5
+8BG0qmZ+Gg0254nzt2wBFta/YxkgK7oJpHKYRqN/ObEpPAY1Wy4dVacQKQbnM3s
CQs5crFvzh3ujfPtv8lFc2GQIW3APLfVsFciOtXyyqYvyVlSv/uRfzgfx/mSutCZ
LDEvtpppFg5Krrp53dn2WRN+fTYtN7PP0o7eJ9z9dsFHIYC29W7+4mnu222/g0yJ
JIUWCm/t3IoK9hdLZoIG9y4/AhjQ5WyKHFVI0+PZOELIHW4+Z+Jfx1UAmDtUi0fy
Tb7er4QI7buUG901slahtDkN
-----END CERTIFICATE-----`
	tlsKey string = `
-----BEGIN PRIVATE KEY-----
MIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQDogj36GuuVtVG0
1KzLy+kGo/n145BDl5ouwYv0PgZ3HoIQvDbgNdBjMCbOcNAwp1SfRBq92hWCle2M
lDUTEbomozjgqDzv6GOtoYoYZxU76HZOGSORRQ8Cra2S6g2BzolwCTTNJ6rlwL9n
hZPWz+AznnlfCYCwWe6NbfcQzvu39z2pxpfsaqIKulwaD49WJg+61/Qq4qM99aoI
MKiqUCxnLxwETA/GcAnckT4eueNHsJD4LvCIDIEGbckrMcRrZXtBFd6UnTZVGXtV
2nAkIFqMBg/Br0J5aS/km47hjWYZD2y6IUJveFyVVO+IadKRHqmWjxwFQgB7BVwL
eiBXcrAbAgMBAAECggEAVNVwl5jK7EzECx6uDY3Q8ENUKItnT8I412Z3Eh6vbTcM
bd6+hwAbkJU5E4nF7HqhPZszxqGTx5m8mtZYpySIryBO2GmKEl7QP8H5CP5TmRAw
Wj6B47c2yttjwX70frBFJUO2qEQY7sttCvCKCI7AVxUzY6Gr+qxVhfTheJiM74ns
MoZKrYsvaqwDAR7IvVZz6PPORyRvbfenwMVmpPaBApSNGZjYz4sTo6I6NGteq1Yu
AorVg/CLR5d4O+KMU0uyKuM2edOnGHj+svXyUZBxDzAFzDvKz8r5eQiKOYcAAzHG
VFt0M2LUj6FkXBLnP1pjYknBS5JiAy9/qbbwVriBQQKBgQD6+8XJgtW3Wg+ZrDkq
5JnUCdZTNULibrom5RWOG5O4M+zOXuIeGJaekkzbcZNp3VMqixtQzhz+QxEHoded
kn4NgLVqf76zqgMe1rcWPI82+Xum0E++baJgLxoc8Zck87wwVBAsJQ1E6WUDJqz+
gIZLmbCkKjSIC8BhvApI2u8EBwKBgQDtJ/A4fQvp20hgIvZnkYC+OXIXjK1M/RBI
5HFFcQwKwbd46vtDCjif+h/AY5rr8JumvVG3uIAXh/qxDmsbkJ4OWv+SWMPHUDbq
ixQ4gd+8MzQcElSQK2d458JUtZ96Kdhmun73eFXwKW65ct/+b6VxH2pgrIG4RJnS
5kbhXGM2TQKBgGOMPTTiCfaBaDKhlsMmjMUHadTzCSZamMcYkeYdlge3wLNR+wnI
4uTeTlGzyK5ytKvpJNp2BhXrb/PBA45iLlEYvdwR8we75ST0MQZG2t8JMTxG33o+
besMg6T7ReHIMtpQXWHFCHBOylvnmTIQtDOEMAXNH6zeTF33gXTIMYk9AoGAPIpH
foQdeHNsBG6obEPuk6DiiTR2QQMRFyqJ5+o14sEU7x89SR3g2qXlWR2UPMrNUUFf
DQFiYZ9q1awSl5TRZGTCfT9/qu/FNRaP8OTmkoqXsNrVD4ClB25SY4GB1pO8FG1j
YBUuCwLoqxqyJ6ekmj4kz8z5yGpqwjXavkjxYrkCgYACsrAeAbN1/DAGeRuyma66
TYOoMIw/XlYDfXXyxKxLYRvXerPoEVkwII6m9AS9o/bH6DDBqLJDNRhNGOK/3ZR0
j47sfX8KszGDIuoeR7dnGCTc1PGtQ1Uhn4Z6mm1NMJrmc7v/fkxIQoSlq1/o8fGv
QZ+yDlTdRpvoEP2mzW2cZA==
-----END PRIVATE KEY-----`
)

func TestHttps(t *testing.T) {
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

	secr := ensureSecret(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tls-secret",
			Namespace: "gateway",
		},
		Data: map[string][]byte{
			"tls.crt": []byte(tlsCrt),
			"tls.key": []byte(tlsKey),
		},
		Type: corev1.SecretTypeOpaque,
	}, clientSet, t)

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "https-ing",
			Namespace: ns.Name,
			Annotations: map[string]string{
				parser.GetAnnotationWithPrefix("force-ssl-redirect"): "true",
			},
		},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{
				{
					Hosts:      []string{"www.https.com"},
					SecretName: secr.Name,
				},
			},
			Rules: []networkingv1.IngressRule{
				{
					Host: "www.https.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path: "/https",
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "default-svc",
											Port: networkingv1.ServiceBackendPort{
												Number: 80,
											},
										},
									},
								},
							},
						},
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

func ensureSecret(service *corev1.Secret, clientSet kubernetes.Interface, t *testing.T) *corev1.Secret {
	t.Helper()
	serc, err := clientSet.CoreV1().Secrets(service.Namespace).Update(context.TODO(), service, metav1.UpdateOptions{})

	if err != nil {
		if k8sErrors.IsNotFound(err) {
			t.Logf("Secret %v not found, creating", service)

			serc, err = clientSet.CoreV1().Secrets(service.Namespace).Create(context.TODO(), service, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("error creating secret %+v: %v", service, err)
			}

			t.Logf("Secret %+v created", service)
			return serc
		}

		t.Fatalf("error updating secret %+v: %v", service, err)
	}

	t.Logf("Secret %+v updated", service)

	return serc
}
