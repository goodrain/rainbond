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

package v1

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "k8s.io/api/apps/v1"
)

func TestGetStatefulsetModifiedConfiguration(t *testing.T) {
	var replicas int32 = 1
	var replicasnew int32 = 2
	bytes, err := getStatefulsetModifiedConfiguration(&v1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "teststatefulset",
			Labels: map[string]string{
				"version": "1",
			},
		},
		Spec: v1.StatefulSetSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					NodeName: "v1",
					NodeSelector: map[string]string{
						"test": "1111",
					},
					Containers: []corev1.Container{
						corev1.Container{
							Image: "nginx",
							Name:  "nginx1",
							Env: []corev1.EnvVar{
								corev1.EnvVar{
									Name:  "version",
									Value: "V1",
								},
								corev1.EnvVar{
									Name:  "delete",
									Value: "true",
								},
							},
						},
						corev1.Container{
							Image: "nginx",
							Name:  "nginx2",
						},
					},
				},
			},
		},
	}, &v1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "teststatefulset",
			Labels: map[string]string{
				"version": "2",
			},
		},
		Spec: v1.StatefulSetSpec{
			Replicas: &replicasnew,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					NodeName: "v2",
					NodeSelector: map[string]string{
						"test": "1111",
					},
					Containers: []corev1.Container{
						corev1.Container{
							Image: "nginx",
							Name:  "nginx1",
							Env: []corev1.EnvVar{
								corev1.EnvVar{
									Name:  "version",
									Value: "V2",
								},
							},
						},
						corev1.Container{
							Image: "nginx",
							Name:  "nginx3",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(bytes))
}
