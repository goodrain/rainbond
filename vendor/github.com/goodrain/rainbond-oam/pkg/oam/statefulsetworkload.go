// RAINBOND, Application Management Platform
// Copyright (C) 2020-2020 Goodrain Co., Ltd.

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

package oam

import (
	v1alpha2 "github.com/crossplane/oam-kubernetes-runtime/apis/core/v1alpha2"
	"github.com/goodrain/rainbond-oam/pkg/ram/v1alpha1"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type statefulWorkloadBuilder struct {
	com     v1alpha1.Component
	plugins []v1alpha1.Plugin
	output  []v1alpha2.DataOutput
}

func (s *statefulWorkloadBuilder) Build() runtime.RawExtension {
	var statefulset = &apps.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        s.com.ServiceCname,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
		Spec: apps.StatefulSetSpec{
			Replicas:    Int32(s.com.ExtendMethodRule.MinNode),
			Template:    s.buildPodTemplate(),
			ServiceName: "",
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": s.com.ServiceName,
				},
			},
			UpdateStrategy: apps.StatefulSetUpdateStrategy{
				Type: apps.RollingUpdateStatefulSetStrategyType,
			},
		},
	}
	return runtime.RawExtension{Object: statefulset}
}

func (s *statefulWorkloadBuilder) buildPodTemplate() core.PodTemplateSpec {
	var podT = core.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:        s.com.ServiceCname,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
		Spec: core.PodSpec{
			Volumes:          s.buildVolume(),
			Containers:       s.buildPodContainer(),
			InitContainers:   s.buildPodInitContainer(),
			RestartPolicy:    core.RestartPolicyAlways,
			ImagePullSecrets: []core.LocalObjectReference{},
		},
	}
	return podT
}

func (s *statefulWorkloadBuilder) buildVolume() []core.Volume {
	return nil
}

func (s *statefulWorkloadBuilder) buildPodContainer() []core.Container {
	return nil
}

func (s *statefulWorkloadBuilder) buildPodInitContainer() []core.Container {
	return nil
}

func (s *statefulWorkloadBuilder) Kind() string {
	return "StatefulsetWorkload"
}

func (s *statefulWorkloadBuilder) Output() []v1alpha2.DataOutput {
	return s.output
}
