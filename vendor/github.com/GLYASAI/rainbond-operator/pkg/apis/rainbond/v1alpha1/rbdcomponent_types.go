package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LogLevel -
type LogLevel string

const (
	// LogLevelDebug -
	LogLevelDebug LogLevel = "debug"
	// LogLevelInfo -
	LogLevelInfo LogLevel = "info"
	// LogLevelWarning -
	LogLevelWarning LogLevel = "warning"
	// LogLevelError -
	LogLevelError LogLevel = "error"
)

// RbdComponentSpec defines the desired state of RbdComponent
type RbdComponentSpec struct {
	// Number of desired pods. This is a pointer to distinguish between explicit
	// zero and not specified. Defaults to 1.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// type of rainbond component
	Type string `json:"type,omitempty"`
	// version of rainbond component
	Version  string   `json:"version,omitempty"`
	LogLevel LogLevel `json:"logLevel,omitempty"`
	// Docker image name.
	Image string `json:"image,omitempty"`
	// Image pull policy.
	// One of Always, Never, IfNotPresent.
	// Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
	// Cannot be updated.
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	PackagePath string `json:"packagePath,omitempty"`
}

// ControllerType -
type ControllerType string

const (
	// ControllerTypeDeployment -
	ControllerTypeDeployment ControllerType = "deployment"
	// ControllerTypeDaemonSet -
	ControllerTypeDaemonSet ControllerType = "daemonset"
	// ControllerTypeStatefulSet -
	ControllerTypeStatefulSet ControllerType = "statefuleset"
	// ControllerTypeUnknown -
	ControllerTypeUnknown ControllerType = "unknown"
)

func (c ControllerType) String() string {
	return string(c)
}

// RbdComponentStatus defines the observed state of RbdComponent
type RbdComponentStatus struct {
	// Type of Controller owned by RbdComponent
	ControllerType ControllerType `json:"controllerType"`
	// ControllerName represents the Controller associated with RbdComponent
	// The controller could be Deployment, StatefulSet or DaemonSet
	ControllerName string `json:"controllerName"`

	PackageExtracted bool `json:"packageExtracted"`

	ImagesLoaded bool `json:"imagesLoaded"`

	ImagesPushed bool `json:"imagesPushed"`

	Reason string `json:"reason"`

	Message string `json:"message"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RbdComponent is the Schema for the rbdcomponents API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=rbdcomponents,scope=Namespaced
type RbdComponent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RbdComponentSpec    `json:"spec,omitempty"`
	Status *RbdComponentStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RbdComponentList contains a list of RbdComponent
type RbdComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RbdComponent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RbdComponent{}, &RbdComponentList{})
}

func (in *RbdComponent) GetLabels() map[string]string {
	return map[string]string{
		"creator":  "Rainbond",
		"belongTo": "RainbondOperator",
		"name":     in.Name,
	}
}

func (in *RbdComponent) ImagePullPolicy() corev1.PullPolicy {
	if in.Spec.ImagePullPolicy == "" {
		return corev1.PullAlways
	}
	return in.Spec.ImagePullPolicy
}

func (in *RbdComponent) LogLevel() LogLevel {
	if in.Spec.LogLevel == "" {
		return LogLevelInfo
	}
	return in.Spec.LogLevel
}
