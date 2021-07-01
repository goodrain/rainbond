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

package v1alpha1

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&ThirdComponent{}, &ThirdComponentList{})
}

// +genclient
// +kubebuilder:object:root=true

// HelmApp -
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=thirdcomponents,scope=Namespaced
type ThirdComponent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ThirdComponentSpec   `json:"spec,omitempty"`
	Status ThirdComponentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ThirdComponentList contains a list of ThirdComponent
type ThirdComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ThirdComponent `json:"items"`
}

type ThirdComponentSpec struct {
	// health check probe
	// +optional
	Probe *HealthProbe `json:"probe,omitempty"`
	// component regist ports
	Ports []*ComponentPort `json:"ports"`
	// endpoint source config
	EndpointSource ThirdComponentEndpointSource `json:"endpointSource"`
}

type ThirdComponentEndpointSource struct {
	StaticEndpoints   []*ThirdComponentEndpoint `json:"endpoints,omitempty"`
	KubernetesService *KubernetesServiceSource  `json:"kubernetesService,omitempty"`
	//other source
	// NacosSource
	// EurekaSource
	// ConsulSource
	// CustomAPISource
}

type ThirdComponentEndpoint struct {
	// The address including the port number.
	Address string `json:"address"`
	// Address protocols, including: HTTP, TCP, UDP, HTTPS
	// +optional
	Protocol string `json:"protocol,omitempty"`
	// Specify a private certificate when the protocol is HTTPS
	// +optional
	ClentSecret string `json:"clientSecret,omitempty"`
}

type KubernetesServiceSource struct {
	// If not specified, the namespace is the namespace of the current resource
	// +optional
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
}

type HealthProbe struct {
	// HTTPGet specifies the http request to perform.
	// +optional
	HTTPGet *HTTPGetAction `json:"httpGet,omitempty"`
	// TCPSocket specifies an action involving a TCP port.
	// TCP hooks not yet supported
	// TODO: implement a realistic TCP lifecycle hook
	// +optional
	TCPSocket *TCPSocketAction `json:"tcpSocket,omitempty"`
}

//ComponentPort component port define
type ComponentPort struct {
	Name      string `json:"name"`
	Port      int    `json:"port"`
	OpenInner bool   `json:"openInner"`
	OpenOuter bool   `json:"openOuter"`
}

//TCPSocketAction enable tcp check
type TCPSocketAction struct {
}

//HTTPGetAction enable http check
type HTTPGetAction struct {
	// Path to access on the HTTP server.
	// +optional
	Path string `json:"path,omitempty"`
	// Custom headers to set in the request. HTTP allows repeated headers.
	// +optional
	HTTPHeaders []HTTPHeader `json:"httpHeaders,omitempty"`
}

// HTTPHeader describes a custom header to be used in HTTP probes
type HTTPHeader struct {
	// The header field name
	Name string `json:"name"`
	// The header field value
	Value string `json:"value"`
}

type ComponentPhase string

// These are the valid statuses of pods.
const (
	// ComponentPending means the component has been accepted by the system, but one or more of the service or endpoint
	// can not create success
	ComponentPending ComponentPhase = "Pending"
	// ComponentRunning means the the service and endpoints create success.
	ComponentRunning ComponentPhase = "Running"
	// ComponentFailed means that found endpoint from source failure
	ComponentFailed ComponentPhase = "Failed"
)

type ThirdComponentStatus struct {
	Phase     ComponentPhase                  `json:"phase"`
	Reason    string                          `json:"reason,omitempty"`
	Endpoints []*ThirdComponentEndpointStatus `json:"endpoints"`
}

type EndpointStatus string

const (
	//EndpointReady If a probe is configured, it means the probe has passed.
	EndpointReady EndpointStatus = "Ready"
	//EndpointNotReady it means the probe not passed.
	EndpointNotReady EndpointStatus = "NotReady"
)

type EndpointAddress string

func (e EndpointAddress) GetIP() string {
	info := strings.Split(string(e), ":")
	if len(info) == 2 {
		return info[0]
	}
	return ""
}

func (e EndpointAddress) GetPort() int {
	info := strings.Split(string(e), ":")
	if len(info) == 2 {
		port, _ := strconv.Atoi(info[1])
		return port
	}
	return 0
}

func NewEndpointAddress(host string, port int) *EndpointAddress {
	if net.ParseIP(host) == nil {
		return nil
	}
	if port < 0 || port > 65533 {
		return nil
	}
	ea := EndpointAddress(fmt.Sprintf("%s:%d", host, port))
	return &ea
}

//ThirdComponentEndpointStatus endpoint status
type ThirdComponentEndpointStatus struct {
	// The address including the port number.
	Address EndpointAddress `json:"address"`
	// Reference to object providing the endpoint.
	// +optional
	TargetRef *v1.ObjectReference `json:"targetRef,omitempty" protobuf:"bytes,2,opt,name=targetRef"`
	// ServicePort if address build from kubernetes endpoint, The corresponding service port
	ServicePort int `json:"servicePort,omitempty"`
	//Status endpoint status
	Status EndpointStatus `json:"status"`
	//Reason probe not passed reason
	Reason string `json:"reason,omitempty"`
}
