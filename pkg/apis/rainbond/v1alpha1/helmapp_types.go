/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The phase of helm app
type HelmAppStatusPhase string

// The phase of helm app
const (
	HelmAppStatusPhaseInitialing  HelmAppStatusPhase = "initialing"
	HelmAppStatusPhaseDetecting   HelmAppStatusPhase = "detecting"
	HelmAppStatusPhaseConfiguring HelmAppStatusPhase = "configuring"
	HelmAppStatusPhaseInstalling  HelmAppStatusPhase = "installing"
	HelmAppStatusPhaseInstalled   HelmAppStatusPhase = "installed"
)

// The status of helm app
// Except for `not-configured`, the other statues are the native statues of helm.
type HelmAppStatusStatus string

// The status of helm app
const (
	HelmAppStatusNotConfigured HelmAppStatusStatus = "not-configured"

	// HelmAppStatusunknown indicates that a release is in an uncertain state.
	HelmAppStatusunknown HelmAppStatusStatus = "unknown"

	// HelmAppStatusDeployed indicates that the release has been pushed to Kubernetes.
	HelmAppStatusDeployed HelmAppStatusStatus = "deployed"

	// HelmAppStatusUninstalled indicates that a release has been uninstalled from Kubernetes.
	HelmAppStatusUninstalled HelmAppStatusStatus = "uninstalled"

	// HelmAppStatusSuperseded indicates that this release object is outdated and a newer one exists.
	HelmAppStatusSuperseded HelmAppStatusStatus = "superseded"

	// HelmAppStatusFailed indicates that the release was not successfully deployed.
	HelmAppStatusFailed HelmAppStatusStatus = "failed"

	// HelmAppStatusUninstalling indicates that a uninstall operation is underway.
	HelmAppStatusUninstalling HelmAppStatusStatus = "uninstalling"

	// HelmAppStatusPendingInstall indicates that an install operation is underway.
	HelmAppStatusPendingInstall HelmAppStatusStatus = "pending-install"

	// HelmAppStatusPendingUpgrade indicates that an upgrade operation is underway.
	HelmAppStatusPendingUpgrade HelmAppStatusStatus = "pending-upgrade"

	// HelmAppStatusPendingRollback indicates that an rollback operation is underway.
	HelmAppStatusPendingRollback HelmAppStatusStatus = "pending-rollback"
)

// RbdComponentConditionType is a valid value for RbdComponentCondition.Type
type HelmAppConditionType string

// These are valid conditions of helm app.
const (
	//  HelmAppRepoReady indicates whether the helm repository is ready.
	HelmAppRepoReady HelmAppConditionType = "RepoReady"
	//  HelmAppChartReady indicates whether the chart is ready.
	HelmAppChartReady HelmAppConditionType = "ChartReady"
	// HelmAppPreInstalled indicates whether the helm app has been pre installed.
	HelmAppPreInstalled HelmAppConditionType = "PreInstalled"
	// HelmAppPreInstalled indicates whether the chart has been parsed.
	HelmAppChartParsed HelmAppConditionType = "ChartParsed"
	// HelmAppInstalled indicates whether the helm app has been installed.
	HelmAppInstalled HelmAppConditionType = "HelmAppInstalled"
)

// HelmAppCondition contains details for the current condition of this helm application.
type HelmAppCondition struct {
	// Type is the type of the condition.
	Type HelmAppConditionType `json:"type" protobuf:"bytes,1,opt,name=type,casttype=PodConditionType"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Status corev1.ConditionStatus `json:"status" protobuf:"bytes,2,opt,name=status,casttype=ConditionStatus"`
	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty" protobuf:"bytes,4,opt,name=lastTransitionTime"`
	// Unique, one-word, CamelCase reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,5,opt,name=reason"`
	// Human-readable message indicating details about last transition.
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,6,opt,name=message"`
}

// HelmAppSpec defines the desired state of HelmApp
type HelmAppSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	EID string `json:"eid"`

	// The prerequisite status.
	// +kubebuilder:validation:Enum=NotConfigured;Configured
	PreStatus string `json:"preStatus,omitempty"`

	// The application name.
	// TODO: validation
	TemplateName string `json:"appName"`

	// The application version.
	// TODO: validation
	Version string `json:"version"`

	// The application revision.
	Revision *int32 `json:"revision,omitempty"`

	// The values.yaml of the helm app, encoded by base64.
	Values string `json:"values,omitempty"`

	// The helm app store.
	// TODO: validation. not null
	AppStore *HelmAppStore `json:"appStore"`
}

// FullName returns the full name of the app store.
func (in *HelmAppSpec) FullName() string {
	if in.AppStore == nil {
		return ""
	}
	return in.EID + "-" + in.AppStore.Name
}

// HelmAppStore represents a helm repo.
type HelmAppStore struct {
	// The verision of the helm app store.
	Version string `json:"version"`

	Name string `json:"name"`

	// The url of helm repo, sholud be a helm native repo url or a git url.
	URL string `json:"url"`

	// The branch of a git repo.
	Branch string `json:"branch,omitempty"`

	Username string `json:"username,omitempty"`

	Password string `json:"password,omitempty"`
}

// HelmAppStatus defines the observed state of HelmApp
type HelmAppStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The status of helm app.
	Status HelmAppStatusStatus `json:"status"`

	Phase HelmAppStatusPhase `json:"phase"`

	// Current state of helm app.
	Conditions []HelmAppCondition `json:"conditions,omitempty"`

	CurrentValues string `json:"currentValues,omitempty"`

	CurrentRevision string `json:"currentRevision,omitempty"`

	ValuesTemplate string `json:"valuesTemplate,omitempty"`

	Readme string `json:"readme,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true

// HelmApp -
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=helmapps,scope=Namespaced
type HelmApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HelmAppSpec   `json:"spec,omitempty"`
	Status HelmAppStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HelmAppList contains a list of HelmApp
type HelmAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HelmApp `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HelmApp{}, &HelmAppList{})
}
