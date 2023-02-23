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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RBDPluginSpec defines the desired state of RBDPlugin
type RBDPluginSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of RBDPlugin. Edit rbdplugin_types.go to remove/update
	Author      string `json:"author,omitempty"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	// Alias The alias is the name used for display, and if this field is not set, the name in the metadata will be used
	Alias string `json:"alias,omitempty"`
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
