// RAINBOND, Application Management Platform
// Copyright (C) 2021-2021 Goodrain Co., Ltd.

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

package componentdefinition

import (
	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/oam-dev/kubevela/apis/core.oam.dev/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var cueTemplate = `
output: {
	apiVersion: "rainbond.io/v1alpha1"
	kind:       "ThirdComponent"
	metadata: {
		name: context.componentID
		namespace: context.namespace
	}
	spec: {
		endpointSource: {
			if parameter["kubernetes"] != _|_ {
				kubernetesService: {
					namespace: parameter["kubernetes"]["namespace"],
					name: parameter["kubernetes"]["name"]
				}
			}
		}
		if parameter["port"] != _|_ {
			ports: parameter["port"]
		}
	}
}

parameter: {
	kubernetes?: {
		namespace?: string
		name: string
	}
	port?: [...{
		name:   string
		port:   >0 & <=65533
		openInner: bool
		openOuter: bool
	}]
}
`
var thirdComponetDefineName = "core-thirdcomponent"
var thirdComponetDefine = v1alpha1.ComponentDefinition{
	TypeMeta: v1.TypeMeta{
		Kind:       "ComponentDefinition",
		APIVersion: "rainbond.io/v1alpha1",
	},
	ObjectMeta: v1.ObjectMeta{
		Name: thirdComponetDefineName,
		Annotations: map[string]string{
			"definition.oam.dev/description": "Rainbond built-in component type that defines third-party service components.",
		},
	},
	Spec: v1alpha1.ComponentDefinitionSpec{
		Workload: common.WorkloadTypeDescriptor{
			Type: "ThirdComponent",
			Definition: common.WorkloadGVK{
				APIVersion: "rainbond.io/v1alpha1",
				Kind:       "ThirdComponent",
			},
		},
		Schematic: &v1alpha1.Schematic{
			CUE: &common.CUE{
				Template: cueTemplate,
			},
		},
	},
}
