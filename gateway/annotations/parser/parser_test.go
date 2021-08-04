/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package parser

import (
	"testing"

	api "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func buildIngress() *networkingv1.Ingress {
	return &networkingv1.Ingress{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "foo",
			Namespace: api.NamespaceDefault,
		},
		Spec: extensions.IngressSpec{},
	}
}

func TestGetBoolAnnotation(t *testing.T) {
	ing := buildIngress()

	_, err := GetBoolAnnotation("", nil)
	if err == nil {
		t.Errorf("expected error but retuned nil")
	}

	tests := []struct {
		name   string
		field  string
		value  string
		exp    bool
		expErr bool
	}{
		{"valid - false", "bool", "false", false, false},
		{"valid - true", "bool", "true", true, false},
	}

	data := map[string]string{}
	ing.SetAnnotations(data)

	for _, test := range tests {
		data[GetAnnotationWithPrefix(test.field)] = test.value

		u, err := GetBoolAnnotation(test.field, ing)
		if test.expErr {
			if err == nil {
				t.Errorf("%v: expected error but retuned nil", test.name)
			}
			continue
		}
		if u != test.exp {
			t.Errorf("%v: expected \"%v\" but \"%v\" was returned", test.name, test.exp, u)
		}

		delete(data, test.field)
	}
}

func TestGetStringAnnotation(t *testing.T) {
	ing := buildIngress()

	_, err := GetStringAnnotation("", nil)
	if err == nil {
		t.Errorf("expected error but retuned nil")
	}

	tests := []struct {
		name   string
		field  string
		value  string
		exp    string
		expErr bool
	}{
		{"valid - A", "string", "A", "A", false},
		{"valid - B", "string", "B", "B", false},
	}

	data := map[string]string{}
	ing.SetAnnotations(data)

	for _, test := range tests {
		data[GetAnnotationWithPrefix(test.field)] = test.value

		s, err := GetStringAnnotation(test.field, ing)
		if test.expErr {
			if err == nil {
				t.Errorf("%v: expected error but retuned nil", test.name)
			}
			continue
		}
		if s != test.exp {
			t.Errorf("%v: expected \"%v\" but \"%v\" was returned", test.name, test.exp, s)
		}

		delete(data, test.field)
	}
}

func TestGetIntAnnotation(t *testing.T) {
	ing := buildIngress()

	_, err := GetIntAnnotation("", nil)
	if err == nil {
		t.Errorf("expected error but retuned nil")
	}

	tests := []struct {
		name   string
		field  string
		value  string
		exp    int
		expErr bool
	}{
		{"valid - A", "string", "1", 1, false},
		{"valid - B", "string", "2", 2, false},
	}

	data := map[string]string{}
	ing.SetAnnotations(data)

	for _, test := range tests {
		data[GetAnnotationWithPrefix(test.field)] = test.value

		s, err := GetIntAnnotation(test.field, ing)
		if test.expErr {
			if err == nil {
				t.Errorf("%v: expected error but retuned nil", test.name)
			}
			continue
		}
		if s != test.exp {
			t.Errorf("%v: expected \"%v\" but \"%v\" was returned", test.name, test.exp, s)
		}

		delete(data, test.field)
	}
}

func TestGetStringAnnotationWithPrefix(t *testing.T) {
	ing := buildIngress()

	_, err := GetIntAnnotation("", nil)
	if err == nil {
		t.Errorf("expected error but retuned nil")
	}

	data := map[string]string{
		"nginx.ingress.kubernetes.io/proxy-body-size": "0k",
		"nginx.ingress.kubernetes.io/proxy-buffering": "off",
		"nginx.ingress.kubernetes.io/set-header-foo":  "value1",
		"nginx.ingress.kubernetes.io/set-header-bar":  "value2",
	}
	ing.SetAnnotations(data)

	m, err := GetStringAnnotationWithPrefix("set-header-", ing)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(m)
}
