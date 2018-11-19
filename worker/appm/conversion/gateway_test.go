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
	"fmt"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/dao"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/gateway/annotations/parser"
	"github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/rafrombrc/gomock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"strconv"
	"testing"
	"time"
)

func TestApplyTcpRule(t *testing.T) {
	testCase := map[string]string{
		"namespace": "e8539a9c33fd418db11cce26d2bca431",
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
		IsInnerService: false,
		IsOuterService: true,
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

	mappingPort, err := strconv.Atoi(testCase[parser.GetAnnotationWithPrefix("l4-port")])
	if err != nil {
		t.Errorf("Can not convert %s(string) to int: %v",
			testCase[parser.GetAnnotationWithPrefix("l4-port")], err)
	}
	tcpRule := &model.TcpRule{
		ServiceID:        serviceID,
		ContainerPort:    port.ContainerPort,
		IP:               testCase[parser.GetAnnotationWithPrefix("l4-host")],
		Port:             mappingPort,
		LoadBalancerType: model.RoundRobinLBType,
	}

	ing, err := applyTcpRule(tcpRule, service,
		testCase[parser.GetAnnotationWithPrefix("l4-port")], testCase["namespace"])
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

	fmt.Sprintln(ing)
}

func TestAppServiceBuild_ApplyHttpRule(t *testing.T) {
	testCase := map[string]string{
		"namespace": "e8539a9c33fd418db11cce26d2bca431",
		"domain": "www.goodrain.com",
		"path": "/dummy-path",
		"serviceName": "dummy-service-name",
		"servicePort": "10000",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dbmanager := db.NewMockManager(ctrl)

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
		ServiceVersion:  "latest",
		ImageName:       "goodrain.me/runner:latest",
		ContainerCPU:    20,
		ContainerMemory: 128,
		ContainerCMD:    "start_web",
		VolumePath:      "vol43eaae4418",
		ExtendMethod:    "stateless",
		Replicas:        1,
		DeployVersion:   "20181022200709",
		Category:        "application",
		CurStatus:       "undeploy",
		Status:          0,
		ServiceType:     "application",
		Namespace:       "goodrain",
		VolumeType:      "shared",
		PortType:        "multi_outer",
		UpdateTime:      updateTime,
		ServiceOrigin:   "assistant",
		CodeFrom:        "gitlab_demo",
		Domain:          "0enb7gyx",
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
	extensionDao.EXPECT().GetRuleExtensionByServiceID(serviceID).Return(nil, nil)
	dbmanager.EXPECT().RuleExtensionDao().Return(extensionDao)

	replicationType := v1.TypeDeployment
	build, err := AppServiceBuilder(serviceID, string(replicationType), dbmanager)
	if err != nil {
		t.Errorf("Unexpected occurred while creating AppServiceBuild: %v", err)
	}

	httpRule := &model.HttpRule{
		ServiceID:        serviceID,
		ContainerPort:    containerPort,
		Domain:           testCase["domain"],
		Path:             testCase["path"],
		LoadBalancerType: model.RoundRobinLBType,
	}

	port := &model.TenantServicesPort{
		TenantID:      tenant.UUID,
		ServiceID:     serviceID,
		ContainerPort: containerPort,
		MappingPort:containerPort,
		Protocol: "http",
		IsInnerService: false,
		IsOuterService: true,
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

	ing, sec, err := build.applyHttpRule(httpRule, port, service)
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
}