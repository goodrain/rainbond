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
// 文件: component_properties.go
// 说明: 该文件实现了组件属性管理功能的核心组件。文件中定义了用于处理和管理平台中组件属性的相关方法，
// 以支持组件的灵活配置和管理。通过这些方法，Rainbond 平台能够确保组件的属性设置和调整的有效性和一致性，
// 提供强大的组件配置管理能力。

package componentdefinition

import "github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"

// ThirdComponentProperties third component properties
type ThirdComponentProperties struct {
	Kubernetes *ThirdComponentKubernetes          `json:"kubernetes,omitempty"`
	Endpoints  []*v1alpha1.ThirdComponentEndpoint `json:"endpoints,omitempty"`
	Port       []*ThirdComponentPort              `json:"port"`
	Probe      *v1alpha1.Probe                    `json:"probe,omitempty"`
}

// ThirdComponentPort -
type ThirdComponentPort struct {
	Name      string `json:"name"`
	Port      int    `json:"port"`
	OpenInner bool   `json:"openInner"`
	OpenOuter bool   `json:"openOuter"`
}

// ThirdComponentKubernetes -
type ThirdComponentKubernetes struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}
