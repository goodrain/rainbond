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

// RBDAbilitySpec defines the desired state of RBDAbility
type RBDAbilitySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of RBDAbility. Edit rbdplugin_types.go to remove/update
	WatchGroups []WatchGroup `json:"watchGroups,omitempty"`
}

// WatchGroup Defines what types of resources are listed.
// For example, if apiVersion is apps/v1, kind is Deployment,
// it means that the platform will list all Deployment resources.
type WatchGroup struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
}

// RBDAbilityStatus defines the observed state of RBDAbility
type RBDAbilityStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// RBDAbility is the Schema for the rbdplugins API
type RBDAbility struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RBDAbilitySpec   `json:"spec,omitempty"`
	Status RBDAbilityStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// RBDAbilityList contains a list of RBDAbility
type RBDAbilityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RBDAbility `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RBDAbility{}, &RBDAbilityList{})
}
