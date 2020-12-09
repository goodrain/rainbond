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
	"github.com/crossplane/oam-kubernetes-runtime/apis/core/v1alpha2"
	"github.com/goodrain/rainbond-oam/pkg/ram/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type builder struct {
	oamApp *v1alpha2.ApplicationConfiguration
	ram    v1alpha1.RainbondApplicationConfig
}

//Builder oam application model builder
type Builder interface {
	// build oam application
	Build() *v1alpha2.ApplicationConfiguration
}

//WorkloadBuilder workload builder
type WorkloadBuilder interface {
	Build() runtime.RawExtension
	Output() []v1alpha2.DataOutput
	Kind() string
}

//NewBuilder new oam model builder
func NewBuilder(ram v1alpha1.RainbondApplicationConfig) Builder {
	var oam v1alpha2.ApplicationConfiguration
	return &builder{
		oamApp: &oam,
		ram:    ram,
	}
}

//NewWorkloadBuilder new workload builder
func NewWorkloadBuilder(com v1alpha1.Component, plugins []v1alpha1.Plugin) WorkloadBuilder {
	switch com.DeployType {
	case v1alpha1.StateMultipleDeployType, v1alpha1.StateSingletonDeployType:
		return &statefulWorkloadBuilder{
			com:     com,
			plugins: plugins,
		}
	case v1alpha1.StatelessMultipleDeployType, v1alpha1.StatelessSingletionDeployType:
		return &containerWorkloadBuilder{
			com:     com,
			plugins: plugins,
		}
	default:
		return &containerWorkloadBuilder{
			com:     com,
			plugins: plugins,
		}
	}
}

func (b *builder) Build() *v1alpha2.ApplicationConfiguration {
	return b.oamApp
}

func (b *builder) buildApplication() {
	b.oamApp.Name = b.ram.AppName
}

func (b *builder) buildComponent() {
	var components []v1alpha2.Component
	var configurationComponents []v1alpha2.ApplicationConfigurationComponent
	for i := range b.ram.Components {
		rcom := b.ram.Components[i]
		builder := NewWorkloadBuilder(*rcom, b.ram.Plugins)
		cw := builder.Build()
		output := builder.Output()
		component := v1alpha2.Component{
			ObjectMeta: metav1.ObjectMeta{
				Name:        rcom.ServiceCname,
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			Spec: v1alpha2.ComponentSpec{
				Workload: cw,
			},
		}
		components = append(components, component)
		var acc = v1alpha2.ApplicationConfigurationComponent{
			ComponentName: component.GetName(),
			DataOutputs:   output,
		}
		// Handle dependencies between components
		for _, dep := range rcom.DepServiceMapList {
			for _, env := range b.getDepComponentConnectionInfo(dep.DepServiceKey) {
				acc.DataInputs = append(acc.DataInputs, v1alpha2.DataInput{
					ValueFrom: v1alpha2.DataInputValueFrom{
						DataOutputName: env.AttrName,
					},
					//TODO:
					ToFieldPaths: func() []string {
						return []string{""}
					}(),
				})
			}
		}
		configurationComponents = append(configurationComponents, acc)
	}
	b.oamApp.Spec.Components = configurationComponents
}

func (b *builder) getDepComponentConnectionInfo(componentKey string) []v1alpha1.ComponentEnv {
	for _, com := range b.ram.Components {
		if com.ServiceKey == componentKey {
			return com.ServiceConnectInfoMapList
		}
	}
	return nil
}

func (b *builder) buildTrait() {

}
