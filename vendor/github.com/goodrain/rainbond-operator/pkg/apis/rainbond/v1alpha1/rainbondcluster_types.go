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

// FstabLine represents a line in file /etc/fstab.
type FstabLine struct {
	Device     string `json:"fileSystem,omitempty"`
	MountPoint string `json:"mountPoint,omitempty"`
	Type       string `json:"type,omitempty"`
	Options    string `json:"options,omitempty"`
	Dump       int    `json:"dump,omitempty"`
	Pass       int    `json:"pass,omitempty"`
}

// RainbondShareStorage -
type RainbondShareStorage struct {
	StorageClassName string     `json:"storageClassName"`
	FstabLine        *FstabLine `json:"fstabLine"`
}

// RainbondClusterSpec defines the desired state of RainbondCluster
type RainbondClusterSpec struct {
	// Repository of each Rainbond component image, eg. docker.io/rainbond.
	// +optional
	RainbondImageRepository string `json:"rainbondImageRepository,omitempty"`
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
	// User-specified private image repository, replacing goodrain.me.
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
	// define install rainbond version, This is usually image tag
	InstallVersion       string               `json:"installVersion,omitempty"`
	RainbondShareStorage RainbondShareStorage `json:"rainbondShareStorage,omitempty"`
	// Whether the configuration has been completed
	ConfigCompleted bool `json:"configCompleted,omitempty"`
	//InstallPackageConfig define install package download config
	InstallPackageConfig InstallPackageConfig `json:"installPackageConfig,omitempty"`
}

//InstallPackageConfig define install package download config
type InstallPackageConfig struct {
	URL string `json:"url,omitempty"`
	MD5 string `json:"md5,omitempty"`
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

// RainbondClusterStatus defines the observed state of RainbondCluster
type RainbondClusterStatus struct {
	// Master node name list
	MasterNodeNames []string          `json:"nodeNames,omitempty"`
	NodeAvailPorts  []*NodeAvailPorts `json:"NodeAvailPorts,omitempty"`
	// List of existing StorageClasses in the cluster
	// +optional
	StorageClasses []*StorageClass `json:"storageClasses,omitempty"`
	// Destination path of the installation package extraction.
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

//GatewayIngressIPs get all gateway ips
func (in *RainbondCluster) GatewayIngressIPs() (ips []string) {
	// custom ip ,contain eip
	if len(in.Spec.GatewayIngressIPs) > 0 && in.Spec.GatewayIngressIPs[0] != "" {
		return in.Spec.GatewayIngressIPs
	}
	// user select gateway node ip
	if len(in.Spec.GatewayNodes) > 0 {
		for _, node := range in.Spec.GatewayNodes {
			ips = append(ips, node.NodeIP)
		}
		return
	}
	// all available gateway node ip
	if in.Status != nil && len(in.Status.NodeAvailPorts) > 0 {
		for _, node := range in.Status.NodeAvailPorts {
			ips = append(ips, node.NodeIP)
		}
		return
	}
	return nil
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

func (in *RainbondClusterStatus) FirstMasterNodeLabel() map[string]string {
	if len(in.MasterNodeNames) == 0 {
		return nil
	}
	return map[string]string{
		"kubernetes.io/hostname": in.MasterNodeNames[0],
	}
}
