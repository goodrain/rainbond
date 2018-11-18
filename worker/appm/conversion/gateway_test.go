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
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/db/mysql"
	"github.com/goodrain/rainbond/gateway/annotations/parser"
	"github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"strconv"
	"testing"
)

func TestAppServiceBuild_ApplyRules(t *testing.T) {
	dbmanager := &mysql.MockManager{}

	serviceID := "43eaae441859eda35b02075d37d83589"
	replicationType := v1.TypeDeployment
	build, err := AppServiceBuilder(serviceID, string(replicationType), dbmanager)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	ports, _ := build.dbmanager.TenantServicesPortDao().GetOuterPorts(serviceID)

	mockService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-svc",
			Namespace: build.tenant.UUID,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "service-port",
					Port:       30000,
					TargetPort: intstr.FromInt(10000),
				},
			},
			Selector: map[string]string{
				"tier": "default",
			},
		},
	}

	ingresses, secret, err := build.ApplyRules(ports[0], mockService)
	fmt.Println(ingresses)
	fmt.Println(secret)
}

func TestAppServiceBuild_ApplyStreamRule(t *testing.T) {
	testCase := map[string]string{
		"namespace": "e8539a9c33fd418db11cce26d2bca431",
		parser.GetAnnotationWithPrefix("l4-enable"): "true",
		parser.GetAnnotationWithPrefix("l4-host"):   "127.0.0.1",
		parser.GetAnnotationWithPrefix("l4-port"):   "32145",
		"serviceName":   "default-svc",
		"servicePort":   "5000",
		"containerPort": "10000",
	}

	serviceID := "43eaae441859eda35b02075d37d83589"
	servicePort, err := strconv.Atoi(testCase["servicePort"])
	containerPort, err := strconv.Atoi(testCase["containerPort"])
	if err != nil {
		t.Errorf("Can not convert %s(string) to int: %v", testCase["servicePort"], err)
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
					Port:       int32(servicePort),
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
	streamRule := &model.StreamRule{
		ServiceID:        serviceID,
		ContainerPort:    port.ContainerPort,
		IP:               testCase[parser.GetAnnotationWithPrefix("l4-host")],
		Port:             mappingPort,
		LoadBalancerType: model.RoundRobinLBType,
	}

	ing, err := applyStreamRule(streamRule, port, service,
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
	if fmt.Sprintf("%v", ing.Spec.Backend.ServicePort.IntVal) != testCase["servicePort"] {
		t.Errorf("Expected %s for ServicePort but returned %v", testCase["servicePort"],
			ing.Spec.Backend.ServicePort)
	}

	fmt.Sprintln(ing)
}
