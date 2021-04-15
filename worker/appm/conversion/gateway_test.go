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

package conversion

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	gomock "github.com/golang/mock/gomock"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/dao"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/gateway/annotations/parser"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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

func TestApplyTcpRule(t *testing.T) {
	testCase := map[string]string{
		"namespace": "e8539a9c33fd123456789e26d2bca431",
		parser.GetAnnotationWithPrefix("l4-enable"): "true",
		parser.GetAnnotationWithPrefix("l4-host"):   "127.0.0.1",
		parser.GetAnnotationWithPrefix("l4-port"):   "32145",
		"serviceName":   "default-svc",
		"containerPort": "10000",
	}

	serviceID := "43eaae441859eda35b02075d37d83589"
	containerPort, err := strconv.Atoi(testCase["containerPort"])
	if err != nil {
		t.Errorf("Can not convert %s(string) to int: %v", testCase["containerPort"], err)
	}
	port := &model.TenantServicesPort{
		TenantID:       testCase["namespace"],
		ServiceID:      serviceID,
		ContainerPort:  containerPort,
		Protocol:       "http",
		PortAlias:      "GRD835895000",
		IsInnerService: func() *bool { b := false; return &b }(),
		IsOuterService: func() *bool { b := true; return &b }(),
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testCase["serviceName"],
			Namespace: testCase["namespace"],
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "service-port",
					Port:       int32(containerPort),
					TargetPort: intstr.Parse(testCase["containerPort"]),
				},
			},
			Selector: map[string]string{
				"tier": "default",
			},
		},
	}

	externalPort, err := strconv.Atoi(testCase[parser.GetAnnotationWithPrefix("l4-port")])
	if err != nil {
		t.Errorf("Can not convert %s(string) to int: %v",
			testCase[parser.GetAnnotationWithPrefix("l4-port")], err)
	}
	tcpRule := &model.TCPRule{
		UUID:          "default",
		ServiceID:     serviceID,
		ContainerPort: port.ContainerPort,
		IP:            testCase[parser.GetAnnotationWithPrefix("l4-host")],
		Port:          externalPort,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dbmanager := db.NewMockManager(ctrl)

	serviceDao := dao.NewMockTenantServiceDao(ctrl)
	updateTime, _ := time.Parse(time.RFC3339, "2018-10-22T14:14:12Z")
	services := &model.TenantServices{
		TenantID:        testCase["namespace"],
		ServiceID:       serviceID,
		ServiceKey:      "application",
		ServiceAlias:    "grd83589",
		Comment:         "application info",
		ContainerCPU:    20,
		ContainerMemory: 128,
		Replicas:        1,
		DeployVersion:   "20181022200709",
		Category:        "application",
		CurStatus:       "undeploy",
		Status:          0,
		Namespace:       "goodrain",
		UpdateTime:      updateTime,
		ServiceOrigin:   "assistant",
	}
	serviceDao.EXPECT().GetServiceByID(serviceID).Return(services, nil)
	dbmanager.EXPECT().TenantServiceDao().Return(serviceDao)

	tenantDao := dao.NewMockTenantDao(ctrl)
	tenant := &model.Tenants{
		Name: "0enb7gyx",
		UUID: testCase["namespace"],
		EID:  "214ec4d212582eb36a84cc180aad2783",
	}
	tenantDao.EXPECT().GetTenantByUUID(services.TenantID).Return(tenant, nil)
	dbmanager.EXPECT().TenantDao().Return(tenantDao)

	appService := &v1.AppService{}
	appService.ServiceID = serviceID
	appService.CreaterID = "Rainbond"
	appService.TenantID = testCase["namespace"]

	replicationType := v1.TypeDeployment
	build, err := AppServiceBuilder(serviceID, string(replicationType), dbmanager, appService)
	if err != nil {
		t.Errorf("Unexpected occurred while creating AppServiceBuild: %v", err)
	}

	ing, err := build.applyTCPRule(tcpRule, service, testCase["namespace"])
	if err != nil {
		t.Errorf("Unexpected error occurred while applying stream rule: %v", err)
	}

	if ing.Namespace != testCase["namespace"] {
		t.Errorf("Expected %s for namespace but returned %s", testCase["namespace"], ing.Namespace)
	}
	if ing.Annotations[parser.GetAnnotationWithPrefix("l4-enable")] !=
		testCase[parser.GetAnnotationWithPrefix("l4-enable")] {
		t.Errorf("Expected %s for annotations[%s] but returned %s",
			testCase[parser.GetAnnotationWithPrefix("l4-enable")],
			parser.GetAnnotationWithPrefix("l4-enable"),
			ing.Annotations[parser.GetAnnotationWithPrefix("l4-enable")])
	}
	if ing.Annotations[parser.GetAnnotationWithPrefix("l4-host")] !=
		testCase[parser.GetAnnotationWithPrefix("l4-host")] {
		t.Errorf("Expected %s for annotations[%s] but returned %s",
			testCase[parser.GetAnnotationWithPrefix("l4-host")],
			parser.GetAnnotationWithPrefix("l4-host"),
			ing.Annotations[parser.GetAnnotationWithPrefix("l4-host")])
	}
	if ing.Annotations[parser.GetAnnotationWithPrefix("l4-port")] !=
		testCase[parser.GetAnnotationWithPrefix("l4-port")] {
		t.Errorf("Expected %s for annotations[%s] but returned %s",
			testCase[parser.GetAnnotationWithPrefix("l4-port")],
			parser.GetAnnotationWithPrefix("l4-port"),
			ing.Annotations[parser.GetAnnotationWithPrefix("l4-port")])
	}
	if ing.Spec.Backend.ServiceName != testCase["serviceName"] {
		t.Errorf("Expected %s for ServiceName but returned %s", testCase["serviceName"],
			ing.Spec.Backend.ServiceName)
	}
	if ing.Spec.Backend.ServicePort.IntVal != int32(containerPort) {
		t.Errorf("Expected %v for ServicePort but returned %v", containerPort,
			ing.Spec.Backend.ServicePort)
	}

	// create k8s resources
	c, err := clientcmd.BuildConfigFromFlags("", "/Users/abe/go/src/github.com/goodrain/rainbond/test/admin.kubeconfig")
	if err != nil {
		t.Fatalf("read kube config file error: %v", err)
	}
	clientSet, err := kubernetes.NewForConfig(c)
	if err != nil {
		t.Fatalf("create kube api client error: %v", err)
	}
	if _, err := clientSet.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testCase["namespace"],
		},
	}, metav1.CreateOptions{}); err != nil {
		t.Errorf("Can't create Namespace(%s): %v", testCase["namespace"], err)
	}
	if _, err := clientSet.ExtensionsV1beta1().Ingresses(ing.Namespace).Create(context.Background(), ing, metav1.CreateOptions{}); err != nil {
		t.Errorf("Can't create Ingress(%s): %v", ing.Name, err)
	}
	if err := clientSet.CoreV1().Namespaces().Delete(context.Background(), testCase["namespace"], metav1.DeleteOptions{}); err != nil {
		t.Errorf("Can't delete namespace(%s)", testCase["namespace"])
	}
}

func TestAppServiceBuild_ApplyHttpRule(t *testing.T) {
	testCase := map[string]string{
		"namespace":   "e8539a9c33f12345611cce26d2bca431",
		"domain":      "www.goodrain.com",
		"path":        "/dummy-path",
		"serviceName": "dummy-service-name",
		"servicePort": "10000",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dbmanager := db.NewMockManager(ctrl)

	serviceID := "43eaae441859eda35b02075d37d83589"
	httpRuleID := "1232aae441859eda35b02075d37d8f8d"
	containerPort, err := strconv.Atoi(testCase["servicePort"])
	if err != nil {
		t.Errorf("Can't convert %s(string) to int", testCase["servicePort"])
	}

	serviceDao := dao.NewMockTenantServiceDao(ctrl)
	updateTime, _ := time.Parse(time.RFC3339, "2018-10-22T14:14:12Z")
	services := &model.TenantServices{
		TenantID:        testCase["namespace"],
		ServiceID:       serviceID,
		ServiceKey:      "application",
		ServiceAlias:    "grd83589",
		Comment:         "application info",
		ContainerCPU:    20,
		ContainerMemory: 128,
		Replicas:        1,
		DeployVersion:   "20181022200709",
		Category:        "application",
		CurStatus:       "undeploy",
		Status:          0,
		Namespace:       "goodrain",
		UpdateTime:      updateTime,
		ServiceOrigin:   "assistant",
	}
	serviceDao.EXPECT().GetServiceByID(serviceID).Return(services, nil)
	dbmanager.EXPECT().TenantServiceDao().Return(serviceDao)

	tenantDao := dao.NewMockTenantDao(ctrl)
	tenant := &model.Tenants{
		Name: "0enb7gyx",
		UUID: testCase["namespace"],
		EID:  "214ec4d212582eb36a84cc180aad2783",
	}
	tenantDao.EXPECT().GetTenantByUUID(services.TenantID).Return(tenant, nil)
	dbmanager.EXPECT().TenantDao().Return(tenantDao)

	extensionDao := dao.NewMockRuleExtensionDao(ctrl)
	extensionDao.EXPECT().GetRuleExtensionByRuleID(httpRuleID).Return(nil, nil)
	dbmanager.EXPECT().RuleExtensionDao().Return(extensionDao)

	replicationType := v1.TypeDeployment
	build, err := AppServiceBuilder(serviceID, string(replicationType), dbmanager, nil)
	if err != nil {
		t.Errorf("Unexpected occurred while creating AppServiceBuild: %v", err)
	}

	httpRule := &model.HTTPRule{
		UUID:          httpRuleID,
		ServiceID:     serviceID,
		ContainerPort: containerPort,
		Domain:        testCase["domain"],
		Path:          testCase["path"],
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testCase["serviceName"],
			Namespace: build.tenant.UUID,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "service-port",
					Port:       int32(containerPort),
					TargetPort: intstr.FromInt(containerPort),
				},
			},
			Selector: map[string]string{
				"tier": "default",
			},
		},
	}

	ing, sec, err := build.applyHTTPRule(httpRule, containerPort, 0, service)
	if err != nil {
		t.Errorf("Unexpected error occurred whiling applying http rule: %v", err)
	}
	if sec != nil {
		t.Errorf("Expected nil for sec, but returned %v", sec)
	}
	if ing.ObjectMeta.Namespace != testCase["namespace"] {
		t.Errorf("Expected %s for namespace, but returned %s", testCase["namespace"], ing.ObjectMeta.Namespace)
	}
	if ing.Spec.Rules[0].Host != testCase["domain"] {
		t.Errorf("Expected %s for host, but returned %s", testCase["domain"], ing.Spec.Rules[0].Host)
	}
	if ing.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Path != testCase["path"] {
		t.Errorf("Expected %s for path, but returned %s", testCase["path"], ing.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Path)
	}
	if ing.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServiceName != testCase["serviceName"] {
		t.Errorf("Expected %s for serviceName, but returned %s", testCase["serviceName"],
			ing.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServiceName)
	}
	if fmt.Sprintf("%v", ing.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal) != testCase["servicePort"] {
		t.Errorf("Expected %s for servicePort, but returned %s", testCase["servicePort"],
			fmt.Sprintf("%v", ing.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal))
	}

	// create k8s resources
	c, err := clientcmd.BuildConfigFromFlags("", "/Users/abe/go/src/github.com/goodrain/rainbond/test/admin.kubeconfig")
	if err != nil {
		t.Fatalf("read kube config file error: %v", err)
	}
	clientSet, err := kubernetes.NewForConfig(c)
	if err != nil {
		t.Fatalf("create kube api client error: %v", err)
	}
	if _, err := clientSet.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testCase["namespace"],
		},
	}, metav1.CreateOptions{}); err != nil {
		t.Errorf("Can't create Namespace(%s): %v", testCase["namespace"], err)
	}
	if _, err := clientSet.ExtensionsV1beta1().Ingresses(ing.Namespace).Create(context.Background(), ing, metav1.CreateOptions{}); err != nil {
		t.Errorf("Can't create Ingress(%s): %v", ing.Name, err)
	}
	if err := clientSet.CoreV1().Namespaces().Delete(context.Background(), testCase["namespace"], metav1.DeleteOptions{}); err != nil {
		t.Errorf("Can't delete namespace(%s)", testCase["namespace"])
	}
}

func TestAppServiceBuild_ApplyHttpRuleWithCertificate(t *testing.T) {
	testCase := map[string]string{
		"namespace":     "foobar",
		"ruleID":        "abbe1e0038af47048f34809337531d76",
		"domain":        "www.goodrain.com",
		"path":          "/dummy-path",
		"serviceName":   "dummy-service-name",
		"servicePort":   "10000",
		"certificateID": "b2a87cb3d71e408c9ca7b9bfa9d0d9f5",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dbmanager := db.NewMockManager(ctrl)

	ruleID := "abbe1e0038af47048f34809337531d76"
	serviceID := "43eaae441859eda35b02075d37d83589"
	containerPort, err := strconv.Atoi(testCase["servicePort"])
	if err != nil {
		t.Errorf("Can't convert %s(string) to int", testCase["servicePort"])
	}

	serviceDao := dao.NewMockTenantServiceDao(ctrl)
	updateTime, _ := time.Parse(time.RFC3339, "2018-10-22T14:14:12Z")
	services := &model.TenantServices{
		TenantID:        testCase["namespace"],
		ServiceID:       serviceID,
		ServiceKey:      "application",
		ServiceAlias:    "grd83589",
		Comment:         "application info",
		ContainerCPU:    20,
		ContainerMemory: 128,
		Replicas:        1,
		DeployVersion:   "20181022200709",
		Category:        "application",
		CurStatus:       "undeploy",
		Status:          0,
		Namespace:       "goodrain",
		UpdateTime:      updateTime,
		ServiceOrigin:   "assistant",
	}
	serviceDao.EXPECT().GetServiceByID(serviceID).Return(services, nil)
	dbmanager.EXPECT().TenantServiceDao().Return(serviceDao)

	tenantDao := dao.NewMockTenantDao(ctrl)
	tenant := &model.Tenants{
		Name: "0enb7gyx",
		UUID: testCase["namespace"],
		EID:  "214ec4d212582eb36a84cc180aad2783",
	}
	tenantDao.EXPECT().GetTenantByUUID(services.TenantID).Return(tenant, nil)
	dbmanager.EXPECT().TenantDao().Return(tenantDao)

	extensionDao := dao.NewMockRuleExtensionDao(ctrl)
	extensionDao.EXPECT().GetRuleExtensionByRuleID(ruleID).Return(nil, nil)
	dbmanager.EXPECT().RuleExtensionDao().Return(extensionDao)

	certificateDao := dao.NewMockCertificateDao(ctrl)
	certificate := &model.Certificate{
		UUID:            testCase["certificateID"],
		CertificateName: "dummy-certificate-name",
		Certificate:     tlsCrt,
		PrivateKey:      tlsKey,
	}
	certificateDao.EXPECT().GetCertificateByID(testCase["certificateID"]).Return(certificate, nil)
	dbmanager.EXPECT().CertificateDao().Return(certificateDao)

	replicationType := v1.TypeDeployment
	build, err := AppServiceBuilder(serviceID, string(replicationType), dbmanager, nil)
	if err != nil {
		t.Errorf("Unexpected occurred while creating AppServiceBuild: %v", err)
	}

	httpRule := &model.HTTPRule{
		UUID:          ruleID,
		ServiceID:     serviceID,
		ContainerPort: containerPort,
		Domain:        testCase["domain"],
		Path:          testCase["path"],
		CertificateID: testCase["certificateID"],
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testCase["serviceName"],
			Namespace: build.tenant.UUID,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "service-port",
					Port:       int32(containerPort),
					TargetPort: intstr.FromInt(containerPort),
				},
			},
			Selector: map[string]string{
				"tier": "default",
			},
		},
	}

	ing, sec, err := build.applyHTTPRule(httpRule, containerPort, 0, service)
	if err != nil {
		t.Errorf("Unexpected error occurred whiling applying http rule: %v", err)
	}

	// create k8s resources
	c, err := clientcmd.BuildConfigFromFlags("", "/Users/abe/go/src/github.com/goodrain/rainbond/test/admin.kubeconfig")
	if err != nil {
		t.Fatalf("read kube config file error: %v", err)
	}
	clientSet, err := kubernetes.NewForConfig(c)
	if err != nil {
		t.Fatalf("create kube api client error: %v", err)
	}
	if _, err := clientSet.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testCase["namespace"],
		},
	}, metav1.CreateOptions{}); err != nil {
		t.Errorf("Can't create Serect(%s): %v", sec.Name, err)
	}
	if _, err := clientSet.CoreV1().Secrets(sec.Namespace).Create(context.Background(), sec, metav1.CreateOptions{}); err != nil {
		t.Errorf("Can't create Serect(%s): %v", sec.Name, err)
	}
	if _, err := clientSet.ExtensionsV1beta1().Ingresses(ing.Namespace).Create(context.Background(), ing, metav1.CreateOptions{}); err != nil {
		t.Errorf("Can't create Ingress(%s): %v", ing.Name, err)
	}
	if err := clientSet.CoreV1().Namespaces().Delete(context.Background(), testCase["namespace"], metav1.DeleteOptions{}); err != nil {
		t.Errorf("Can't delete namespace(%s)", testCase["namespace"])
	}

	if ing.ObjectMeta.Namespace != testCase["namespace"] {
		t.Errorf("Expected %s for namespace, but returned %s", testCase["namespace"], ing.ObjectMeta.Namespace)
	}
	if ing.Spec.Rules[0].Host != testCase["domain"] {
		t.Errorf("Expected %s for host, but returned %s", testCase["domain"], ing.Spec.Rules[0].Host)
	}
	if ing.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Path != testCase["path"] {
		t.Errorf("Expected %s for path, but returned %s", testCase["path"], ing.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Path)
	}
	if ing.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServiceName != testCase["serviceName"] {
		t.Errorf("Expected %s for serviceName, but returned %s", testCase["serviceName"],
			ing.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServiceName)
	}
	if fmt.Sprintf("%v", ing.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal) != testCase["servicePort"] {
		t.Errorf("Expected %s for servicePort, but returned %s", testCase["servicePort"],
			fmt.Sprintf("%v", ing.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntVal))
	}
	if sec.Namespace != testCase["namespace"] {
		t.Errorf("Expected %s for namespace, but returned %s", testCase["namespace"], sec.Namespace)
	}
}
