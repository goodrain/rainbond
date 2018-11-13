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

package l4

import (
	"github.com/goodrain/rainbond/gateway/annotations/parser"
	api "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"testing"
)

func buildIngress() *extensions.Ingress {
	defaultBackend := extensions.IngressBackend{
		ServiceName: "default-backend",
		ServicePort: intstr.FromInt(80),
	}

	return &extensions.Ingress{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "foo",
			Namespace: api.NamespaceDefault,
		},
		Spec: extensions.IngressSpec{
			Backend: &extensions.IngressBackend{
				ServiceName: "default-backend",
				ServicePort: intstr.FromInt(80),
			},
			Rules: []extensions.IngressRule{
				{
					Host: "foo.bar.com",
					IngressRuleValue: extensions.IngressRuleValue{
						HTTP: &extensions.HTTPIngressRuleValue{
							Paths: []extensions.HTTPIngressPath{
								{
									Path:    "/foo",
									Backend: defaultBackend,
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestL4_Parse(t *testing.T) {
	ing := buildIngress()

	data := map[string]string{}
	data[parser.GetAnnotationWithPrefix("l4-enable")] = "true"
	data[parser.GetAnnotationWithPrefix("l4-host")] = "0.0.0.0"
	data[parser.GetAnnotationWithPrefix("l4-port")] = "12345"
	ing.SetAnnotations(data)


	i, err := NewParser(l4{}).Parse(ing)
	if err != nil {
		t.Errorf("Uxpected error with ingress: %v", err)
		return
	}

	cfg := i.(*Config)
	if !cfg.L4Enable {
		t.Errorf("Expected true as L4Enable but returned %v", cfg.L4Enable)
	}
	if cfg.L4Host != "0.0.0.0" {
		t.Errorf("Expected 0.0.0.0 as L4Host but returned %s", cfg.L4Host)
	}
	if cfg.L4Port != 12345 {
		t.Errorf("Expected 12345 as L4Port but returned %v", cfg.L4Port)
	}
}
