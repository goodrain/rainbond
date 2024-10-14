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

type Author struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RBDPluginSpec defines the desired state of RBDPlugin
type RBDPluginSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of RBDPlugin. Edit rbdplugin_types.go to remove/update
	// DisplayName The alias is the name used for display, and if this field is not set, the name in the metadata will be used
	DisplayName string `json:"display_name,omitempty"`
	Description string `json:"description,omitempty"`
	// +kubebuilder:validation:Enum=JSInject;Iframe
	PluginType PluginType `json:"plugin_type,omitempty"`

	PluginView []PluginView `json:"plugin_views,omitempty"`

	Authors     []Author `json:"authors,omitempty"`
	Icon        string   `json:"icon,omitempty"`
	Version     string   `json:"version,omitempty"`
	FrontedPath string   `json:"fronted_path,omitempty"`
	Backend     string   `json:"backend,omitempty"`
	// AccessUrls Access URL defines the accessible address of the plug-in.
	// If this field is not set, all accessible addresses under the application will be listed in the platform.
	AccessURLs []string `json:"access_urls,omitempty"`
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
