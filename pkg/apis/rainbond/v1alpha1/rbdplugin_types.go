// RAINBOND, Application Management Platform
// Copyright (C) 2022-2022 Goodrain Co., Ltd.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PluginLabelPrefix string

const (
	PluginEnableLabel PluginLabelPrefix = "plugin.rainbond.io/enable"
)

// 为 PluginType 提供 String() 方法
func (p PluginLabelPrefix) String() string {
	return string(p)
}

func (p PluginLabelPrefix) Combine(v PluginLabelValue) string {
	return p.String() + "=" + v.String()
}

type PluginLabelValue string

const (
	True  PluginLabelValue = "true"
	False PluginLabelValue = "false"
)

func (p PluginLabelValue) String() string {
	return string(p)
}

type PluginType string

const (
	JSInject PluginType = "JSInject"
	Iframe   PluginType = "Iframe"
)

// 为 PluginType 提供 String() 方法
func (p PluginType) String() string {
	return string(p)
}

// PluginView View where the plugin is located
// +kubebuilder:validation:Enum=Platform;Team;Application;Component
type PluginView string

const (
	Platform    PluginView = "Platform"
	Team        PluginView = "Team"
	Application PluginView = "Application"
	Component   PluginView = "Component"
)

// 为 PluginView 提供 String() 方法
func (p PluginView) String() string {
	return string(p)
}

// RBDPluginSpec defines the desired state of RBDPlugin
type RBDPluginSpec struct {
	// DisplayName The alias is the name used for display, and if this field is not set, the name in the metadata will be used
	DisplayName string `json:"display_name,omitempty"`
	// +kubebuilder:validation:Enum=JSInject;Iframe
	PluginType PluginType `json:"plugin_type,omitempty"`
	// FrontendComponent Frontend component name
	FrontendComponent string `json:"frontend_component,omitempty"`
	// EntryPath Entry path for the plugin
	EntryPath string `json:"entry_path,omitempty"`
	// PluginViews View where the plugin is located
	PluginViews []PluginView `json:"plugin_views,omitempty"`
	// MenuTitle Menu title for the plugin
	MenuTitle string `json:"menu_title,omitempty"`
	// RoutePath Route path for the plugin
	RoutePath string `json:"route_path,omitempty"`
	// Namespace Namespace identifier for the plugin
	Namespace string `json:"namespace,omitempty"`
	// BackendService Backend service address (k8s FQDN)
	BackendService string `json:"backend_service,omitempty"`
	// FrontendService Frontend service address (k8s FQDN)
	FrontendService string `json:"frontend_service,omitempty"`
}

// RBDPluginStatus defines the observed state of RBDPlugin
type RBDPluginStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// RBDPlugin is the Schema for the rbdplugins API
type RBDPlugin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RBDPluginSpec   `json:"spec,omitempty"`
	Status RBDPluginStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RBDPluginList contains a list of RBDPlugin
type RBDPluginList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RBDPlugin `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RBDPlugin{}, &RBDPluginList{})
}
