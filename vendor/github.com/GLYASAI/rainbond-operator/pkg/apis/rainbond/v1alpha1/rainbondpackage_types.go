package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RainbondPackageSpec defines the desired state of RainbondPackage
type RainbondPackageSpec struct {
	// The path where the rainbond package is located.
	PkgPath string `json:"pkgPath"`
}

// RainbondPackagePhase is a label for the condition of a rainbondcluster at the current time.
type RainbondPackagePhase string

// These are the valid statuses of rainbondcluster.
const (
	// RainbondPackageFailed meas an unknown error occurred while processing the installation package
	RainbondPackageFailed RainbondPackagePhase = "Failed"
	// RainbondPackageWaiting means waiting for prerequisites to be ready
	RainbondPackageWaiting RainbondPackagePhase = "Waiting"
	// RainbondPackageExtracting means that the prerequisites are in place
	// and the installation package is being extracted.
	RainbondPackageExtracting RainbondPackagePhase = "Extracting"
	// RainbondPackageLoading means that the installation package has been extracted
	// and the image is being loaded to the host.
	RainbondPackageLoading RainbondPackagePhase = "Loading"
	// RainbondPackagePushing means that the image has been loaded,
	// and the image is being pushed to the private image repository.
	RainbondPackagePushing RainbondPackagePhase = "Pushing"
	// RainbondPackageCompleted the processing of the installation package has been completed,
	// including extracting the package, loading the images, and pushing the images.
	RainbondPackageCompleted RainbondPackagePhase = "Completed"
)

// RainbondPackageStatus defines the observed state of RainbondPackage
type RainbondPackageStatus struct {
	// The phase of a RainbondPackage is a simple, high-level summary of where the Pod is in its lifecycle.
	// The conditions array, the reason and message fields, and the individual container status
	// arrays contain more detail about the pod's status.
	// +optional
	Phase RainbondPackagePhase `json:"phase,omitempty"`
	// A human readable message indicating details about why the pod is in this condition.
	// +optional
	Message string `json:"message,omitempty"`
	// A brief CamelCase message indicating details about why the pod is in this state.
	// +optional
	Reason          string `json:"reason,omitempty"`
	FilesNumber     int32  `json:"filesNumber,omitempty"`
	NumberExtracted int32  `json:"numberExtracted,omitempty"`
	// The number of images that should be load and pushed.
	ImagesNumber int32 `json:"imagesNumber"`
	// ImagesPushed contains the images have been pushed.
	ImagesPushed map[string]struct{} `json:"imagesPushed,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RainbondPackage is the Schema for the rainbondpackages API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=rainbondpackages,scope=Namespaced
type RainbondPackage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RainbondPackageSpec    `json:"spec,omitempty"`
	Status *RainbondPackageStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RainbondPackageList contains a list of RainbondPackage
type RainbondPackageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RainbondPackage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RainbondPackage{}, &RainbondPackageList{})
}
