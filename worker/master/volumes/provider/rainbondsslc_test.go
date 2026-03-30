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

package provider

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// capability_id: rainbond.worker.volume-provider.select-node
func TestSelectNode(t *testing.T) {
	client := fake.NewSimpleClientset(
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-a"},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
				Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.0.0.1"}},
				Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-b"},
			Status: corev1.NodeStatus{
				Allocatable: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("8Gi"),
				},
				Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.0.0.2"}},
				Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-a", Namespace: "default"},
			Spec: corev1.PodSpec{
				NodeName: "node-a",
				Containers: []corev1.Container{{
					Name: "app",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				}},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-b", Namespace: "default"},
			Spec: corev1.PodSpec{
				NodeName: "node-b",
				Containers: []corev1.Container{{
					Name: "app",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				}},
			},
		},
	)
	pr := &rainbondsslcProvisioner{
		name:    "rainbond.io/provisioner-sslc",
		kubecli: client,
	}
	node, err := pr.selectNode(context.TODO(), "linux", "")
	if err != nil {
		t.Fatal(err)
	}
	if node == nil || node.Name != "node-b" {
		t.Fatalf("expected node-b to be selected, got %#v", node)
	}
}

// capability_id: rainbond.worker.volume-provider.pvc-identifiers
func TestGetVolumeIDByPVCName(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{input: "manual17-gra02c40-0", want: 17},
		{input: "manual17", want: 17},
		{input: "data-sonar-gra7c815-0", want: 0},
	}

	for _, tt := range tests {
		if got := getVolumeIDByPVCName(tt.input); got != tt.want {
			t.Fatalf("getVolumeIDByPVCName(%q)=%d, want %d", tt.input, got, tt.want)
		}
	}
}

// capability_id: rainbond.worker.volume-provider.pvc-identifiers
func TestGetPodNameByPVCName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "manual17-gra02c40-0", want: "gra02c40-0"},
		{input: "manual17", want: "manual17"},
	}

	for _, tt := range tests {
		if got := getPodNameByPVCName(tt.input); got != tt.want {
			t.Fatalf("getPodNameByPVCName(%q)=%q, want %q", tt.input, got, tt.want)
		}
	}
}
