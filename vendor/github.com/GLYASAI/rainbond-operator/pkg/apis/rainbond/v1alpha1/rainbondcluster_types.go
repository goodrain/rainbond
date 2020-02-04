package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InstallMode is the mode of Rainbond cluster installation
type InstallMode string

const (
	// InstallationModeWithPackage means some Rainbond images are from the specified image repository, some are from the installation package.
	InstallationModeWithPackage InstallMode = "WithPackage"
	// InstallationModeWithoutPackage means all Rainbond images are from the specified image repository, not the installation package.
	InstallationModeWithoutPackage InstallMode = "WithoutPackage"

	// LoadBalancerWidth is the width how we describe load balancer
	LoadBalancerWidth = 16

	// LabelNodeRolePrefix is a label prefix for node roles
	// It's copied over to here until it's merged in core: https://github.com/kubernetes/kubernetes/pull/39112
	LabelNodeRolePrefix = "node-role.kubernetes.io/"

	// NodeLabelRole specifies the role of a node
	NodeLabelRole = "kubernetes.io/role"
)

// ImageHub image hub
type ImageHub struct {
	Domain    string `json:"domain,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
}

// Database defines the connection information of database.
type Database struct {
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// EtcdConfig defines the configuration of etcd client.
type EtcdConfig struct {
	// Endpoints is a list of URLs.
	Endpoints []string `json:"endpoints,omitempty"`
	// Whether to use tls to connect to etcd
	SecretName string `json:"secretName,omitempty"`
}

// KubeletConfig defines the configuration of kubelet.
type KubeletConfig struct {
	// Whether to use tls to connect to etcd
	SecretName string `json:"secretName,omitempty"`
}

// FstabLine represents a line in file /etc/fstab.
type FstabLine struct {
	FileSystem string `json:"fileSystem,omitempty"`
	MountPoint string `json:"mountPoint,omitempty"`
	Type       string `json:"type,omitempty"`
	Options    string `json:"options,omitempty"`
	Dump       int    `json:"dump,omitempty"`
	Pass       int    `json:"pass,omitempty"`
}

// RainbondClusterSpec defines the desired state of RainbondCluster
type RainbondClusterSpec struct {
	// Domain name of the image repository which Rainbond is installed
	// Default goodrain.me
	// +optional
	RainbondImageRepositoryDomain string `json:"rainbondImageRepositoryHost,omitempty"`
	// Suffix of component default domain name
	SuffixHTTPHost string `json:"suffixHTTPHost"`
	// Ingress IP addresses of rbd-gateway. If not specified,
	// the IP of the node where the rbd-gateway is located will be used.
	GatewayIngressIPs []string `json:"gatewayIngressIPs,omitempty"`
	// Information about the node where the gateway is located.
	// If not specified, the gateway will run on nodes where all ports do not conflict.
	GatewayNodes []NodeAvailPorts `json:"gatewayNodes,omitempty"`
	// InstallMode is the mode of Rainbond cluster installation.
	InstallMode InstallMode `json:"installMode,omitempty"`

	ImageHub *ImageHub `json:"imageHub,omitempty"`
	// the storage class that rainbond component will be used.
	// rainbond-operator will create one if StorageClassName is empty
	StorageClassName string `json:"storageClassName,omitempty"`
	// the region database information that rainbond component will be used.
	// rainbond-operator will create one if DBInfo is empty
	RegionDatabase *Database `json:"regionDatabase,omitempty"`
	// the ui database information that rainbond component will be used.
	// rainbond-operator will create one if DBInfo is empty
	UIDatabase *Database `json:"uiDatabase,omitempty"`
	// the etcd connection information that rainbond component will be used.
	// rainbond-operator will create one if EtcdConfig is empty
	EtcdConfig *EtcdConfig `json:"etcdConfig,omitempty"`

	KubeletConfig *KubeletConfig `json:"kubeletConfig,omitempty"`

	Version string `json:"version,omitempty"`

	FstabLines []FstabLine `json:"fstabLines,omitempty"`
}

// RainbondClusterPhase is a label for the condition of a rainbondcluster at the current time.
type RainbondClusterPhase string

// These are the valid statuses of rainbondcluster.
const (
	// RainbondClusterWaiting -
	RainbondClusterWaiting RainbondClusterPhase = "Waiting"
	// RainbondClusterPreparing -
	RainbondClusterPreparing RainbondClusterPhase = "Preparing"
	// RainbondClusterPackageProcessing means the installation package is being processed.
	RainbondClusterPackageProcessing RainbondClusterPhase = "PackageProcessing"
	// RainbondClusterRunning means all of the rainbond components has been created.
	// And at least one component is not ready.
	RainbondClusterPending RainbondClusterPhase = "Pending"
	// RainbondClusterRunning means all of the rainbond components has been created.
	// For each component controller(eg. deploy, sts, ds), at least one Pod is already Ready.
	RainbondClusterRunning RainbondClusterPhase = "Running"
)

var RainbondClusterPhase2Range = map[RainbondClusterPhase]int{
	RainbondClusterWaiting:           0,
	RainbondClusterPreparing:         1,
	RainbondClusterPackageProcessing: 2,
	RainbondClusterPending:           3,
	RainbondClusterRunning:           4,
}

// RainbondClusterConditionType is a valid value for RainbondClusterConditionType.Type
type RainbondClusterConditionType string

// These are valid conditions of rainbondcluster.
const (
	// StorageReady indicates whether the storage is ready.
	StorageReady RainbondClusterConditionType = "StorageReady"
	// ImageRepositoryReady indicates whether the image repository is ready.
	ImageRepositoryInstalled RainbondClusterConditionType = "ImageRepositoryInstalled"
	// PackageExtracted indicates whether the installation package has been decompressed.
	PackageExtracted RainbondClusterConditionType = "PackageExtracted"
	// ImagesPushed means that all images from the installation package has been pushed successfully.
	ImagesPushed RainbondClusterConditionType = "ImagesPushed"
)

// ConditionStatus condition status
type ConditionStatus string

// These are valid condition statuses. "ConditionTrue" means a resource is in the condition.
// "ConditionFalse" means a resource is not in the condition. "ConditionUnknown" means rainbond operator
// can't decide if a resource is in the condition or not.
const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)

// RainbondClusterCondition contains details for the current condition of this rainbondcluster.
type RainbondClusterCondition struct {
	// Type is the type of the condition.
	Type RainbondClusterConditionType `json:"type"`
	// Status is the status of the condition.
	Status ConditionStatus `json:"status"`
	// Last time we probed the condition.
	// +optional
	LastProbeTime *metav1.Time `json:"lastProbeTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
	// Unique, one-word, CamelCase reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition.
	// +optional
	Message string `json:"message,omitempty"`
}

// NodeAvailPorts node avail port
type NodeAvailPorts struct {
	NodeName string `json:"nodeName,omitempty"`
	NodeIP   string `json:"nodeIP,omitempty"`
	Ports    []int  `json:"ports,omitempty"`
}

// StorageClass storage class
type StorageClass struct {
	Name        string `json:"name"`
	Provisioner string `json:"provisioner"`
}

type ControllerStatus struct {
	Name          string `json:"name,omitempty"`
	Replicas      int32  `json:"replicas,omitempty"`
	ReadyReplicas int32  `json:"readyReplicas,omitempty"`
}

// RainbondClusterStatus defines the observed state of RainbondCluster
type RainbondClusterStatus struct {
	// Rainbond cluster phase
	Phase      RainbondClusterPhase       `json:"phase,omitempty"`
	Conditions []RainbondClusterCondition `json:"conditions,omitempty"`
	// A human readable message indicating details about why the pod is in this condition.
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,3,opt,name=message"`
	// A brief CamelCase message indicating details about why the pod is in this state.
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,4,opt,name=reason"`

	// +optional
	NodeAvailPorts []*NodeAvailPorts `json:"NodeAvailPorts,omitempty"`
	// List of existing StorageClasses in the cluster
	// +optional
	StorageClasses []*StorageClass `json:"storageClasses,omitempty"`
	// Destination path of the installation package extraction.
	PkgDestPath string `json:"pkgDestPath"`
	// A list of controller statuses associated with rbdcomponent.
	ControllerStatues []*ControllerStatus `json:"controllerStatus,omitempty"`

	MasterRoleLabel string `json:"masterRoleLabel,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RainbondCluster is the Schema for the rainbondclusters API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=rainbondclusters,scope=Namespaced
type RainbondCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RainbondClusterSpec    `json:"spec,omitempty"`
	Status *RainbondClusterStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RainbondClusterList contains a list of RainbondCluster
type RainbondClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RainbondCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RainbondCluster{}, &RainbondClusterList{})
}

func (in *RainbondCluster) GatewayIngressIP() string {
	if len(in.Spec.GatewayIngressIPs) > 0 && in.Spec.GatewayIngressIPs[0] != "" {
		return in.Spec.GatewayIngressIPs[0]
	}
	if len(in.Spec.GatewayNodes) > 0 {
		return in.Spec.GatewayNodes[0].NodeIP
	}
	if in.Status != nil && len(in.Status.NodeAvailPorts) > 0 {
		return in.Status.NodeAvailPorts[0].NodeIP
	}
	return ""
}

func (in *Database) RegionDataSource() string {
	return fmt.Sprintf("--mysql=%s:%s@tcp(%s:%d)/region", in.Username, in.Password, in.Host, in.Port)
}

func (in *RainbondClusterStatus) MasterNodeLabel() map[string]string {
	switch in.MasterRoleLabel {
	case LabelNodeRolePrefix + "master":
		return map[string]string{
			in.MasterRoleLabel: "",
		}
	case NodeLabelRole:
		return map[string]string{
			NodeLabelRole: "master",
		}
	}

	return nil
}
