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
	"github.com/goodrain/rainbond/db/mysql"
	"github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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
