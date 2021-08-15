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
	"net/url"
	"strconv"
	"strings"

	validation "github.com/goodrain/rainbond/util/endpoint"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&ThirdComponent{}, &ThirdComponentList{})
}

// +genclient
// +kubebuilder:object:root=true

// ThirdComponent -
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=thirdcomponents,scope=Namespaced
type ThirdComponent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ThirdComponentSpec   `json:"spec,omitempty"`
	Status ThirdComponentStatus `json:"status,omitempty"`
}

// GetComponentID -
func (in *ThirdComponent) GetComponentID() string {
	return in.Name
}

// GetEndpointID -
func (in *ThirdComponent) GetEndpointID(endpoint *ThirdComponentEndpointStatus) string {
	return fmt.Sprintf("%s/%s/%s", in.Namespace, in.Name, string(endpoint.Address))
}

// GetNamespaceName -
func (in *ThirdComponent) GetNamespaceName() string {
	return fmt.Sprintf("%s/%s", in.Namespace, in.Name)
}

// +kubebuilder:object:root=true

// ThirdComponentList contains a list of ThirdComponent
type ThirdComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ThirdComponent `json:"items"`
}

// ThirdComponentSpec -
type ThirdComponentSpec struct {
	// health check probe
	// +optional
	Probe *Probe `json:"probe,omitempty"`
	// component regist ports
	Ports []*ComponentPort `json:"ports"`
	// endpoint source config
	EndpointSource ThirdComponentEndpointSource `json:"endpointSource"`
}

// NeedProbe -
func (in ThirdComponentSpec) NeedProbe() bool {
	if in.Probe == nil {
		return false
	}
	return in.IsStaticEndpoints()
}

// IsStaticEndpoints -
func (in ThirdComponentSpec) IsStaticEndpoints() bool {
	return len(in.EndpointSource.StaticEndpoints) > 0
}

// ThirdComponentEndpointSource -
type ThirdComponentEndpointSource struct {
	StaticEndpoints   []*ThirdComponentEndpoint `json:"endpoints,omitempty"`
	KubernetesService *KubernetesServiceSource  `json:"kubernetesService,omitempty"`
	//other source
	// NacosSource
	// EurekaSource
	// ConsulSource
	// CustomAPISource
}

// ThirdComponentEndpoint -
type ThirdComponentEndpoint struct {
	// The address including the port number.
	Address string `json:"address"`
	// Then Name of the Endpoint.
	// +optional
	Name string `json:"name"`
	// Address protocols, including: HTTP, TCP, UDP, HTTPS
	// +optional
	Protocol string `json:"protocol,omitempty"`
	// Specify a private certificate when the protocol is HTTPS
	// +optional
	ClientSecret string `json:"clientSecret,omitempty"`
}

// GetPort -
func (in *ThirdComponentEndpoint) GetPort() int {
	arr := strings.Split(in.Address, ":")
	if len(arr) != 2 {
		return 0
	}

	port, _ := strconv.Atoi(arr[1])
	return port
}

// GetIP -
func (in *ThirdComponentEndpoint) GetIP() string {
	arr := strings.Split(in.Address, ":")
	if len(arr) != 2 {
		return ""
	}

	return arr[0]
}

// KubernetesServiceSource -
type KubernetesServiceSource struct {
	// If not specified, the namespace is the namespace of the current resource
	// +optional
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
}

// Probe describes a health check to be performed against a container to determine whether it is
// alive or ready to receive traffic.
type Probe struct {
	// The action taken to determine the health of a container
	Handler `json:",inline" protobuf:"bytes,1,opt,name=handler"`
	// Number of seconds after which the probe times out.
	// Defaults to 1 second. Minimum value is 1.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty" protobuf:"varint,3,opt,name=timeoutSeconds"`
	// How often (in seconds) to perform the probe.
	// Default to 10 seconds. Minimum value is 1.
	// +optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty" protobuf:"varint,4,opt,name=periodSeconds"`
	// Minimum consecutive successes for the probe to be considered successful after having failed.
	// +optional
	SuccessThreshold int32 `json:"successThreshold,omitempty" protobuf:"varint,5,opt,name=successThreshold"`
	// Minimum consecutive failures for the probe to be considered failed after having succeeded.
	// Defaults to 3. Minimum value is 1.
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty" protobuf:"varint,6,opt,name=failureThreshold"`
}

// Equals -
func (in *Probe) Equals(target *Probe) bool {
	if in.TimeoutSeconds != target.TimeoutSeconds {
		return false
	}
	if in.PeriodSeconds != target.PeriodSeconds {
		return false
	}
	if in.SuccessThreshold != target.SuccessThreshold {
		return false
	}
	if in.FailureThreshold != target.FailureThreshold {
		return false
	}

	return in.Handler.Equals(&target.Handler)
}

// Handler defines a specific action that should be taken
type Handler struct {
	// HTTPGet specifies the http request to perform.
	// +optional
	HTTPGet *HTTPGetAction `json:"httpGet,omitempty"`
	// TCPSocket specifies an action involving a TCP port.
	// TCP hooks not yet supported
	// TODO: implement a realistic TCP lifecycle hook
	// +optional
	TCPSocket *TCPSocketAction `json:"tcpSocket,omitempty"`
}

// Equals -
func (in *Handler) Equals(target *Handler) bool {
	if in == nil && target == nil {
		return true
	}
	if in == nil || target == nil {
		return false
	}

	if !in.HTTPGet.Equals(target.HTTPGet) {
		return false
	}
	return in.TCPSocket.Equals(target.TCPSocket)
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

// Equals -
func (in *TCPSocketAction) Equals(target *TCPSocketAction) bool {
	return true
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

// Equals -
func (in *HTTPGetAction) Equals(target *HTTPGetAction) bool {
	if in == nil && target == nil {
		return true
	}
	if in == nil || target == nil {
		return false
	}

	if in.Path != target.Path {
		return false
	}
	if len(in.HTTPHeaders) != len(target.HTTPHeaders) {
		return false
	}

	headers := make(map[string]string)
	for _, header := range in.HTTPHeaders {
		headers[header.Name] = header.Value
	}
	for _, header := range target.HTTPHeaders {
		value, ok := headers[header.Name]
		if !ok {
			return false
		}
		if header.Value != value {
			return false
		}
	}
	return true
}

// HTTPHeader describes a custom header to be used in HTTP probes
type HTTPHeader struct {
	// The header field name
	Name string `json:"name"`
	// The header field value
	Value string `json:"value"`
}

// ComponentPhase -
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

// ThirdComponentStatus -
type ThirdComponentStatus struct {
	Phase     ComponentPhase                  `json:"phase"`
	Reason    string                          `json:"reason,omitempty"`
	Endpoints []*ThirdComponentEndpointStatus `json:"endpoints"`
}

// EndpointStatus -
type EndpointStatus string

const (
	//EndpointReady If a probe is configured, it means the probe has passed.
	EndpointReady EndpointStatus = "Ready"
	//EndpointNotReady it means the probe not passed.
	EndpointNotReady EndpointStatus = "NotReady"
	// EndpointUnhealthy means that the health prober failed.
	EndpointUnhealthy EndpointStatus = "Unhealthy"
)

// EndpointAddress -
type EndpointAddress string

// GetIP -
func (e EndpointAddress) GetIP() string {
	ip := e.getIP()
	if validation.IsDomainNotIP(ip) {
		return "1.1.1.1"
	}
	return ip
}

func (e EndpointAddress) getIP() string {
	info := strings.Split(string(e), ":")
	if len(info) == 2 {
		return info[0]
	}
	return ""
}

// GetPort -
func (e EndpointAddress) GetPort() int {
	if !validation.IsDomainNotIP(e.getIP()) {
		info := strings.Split(string(e), ":")
		if len(info) == 2 {
			port, _ := strconv.Atoi(info[1])
			return port
		}
		return 0
	}

	u, err := url.Parse(e.EnsureScheme())
	if err != nil {
		logrus.Errorf("parse address %s: %v", e.EnsureScheme(), err)
		return 0
	}
	logrus.Infof("url: %s; scheme: %s", e.EnsureScheme(), u.Scheme)
	if u.Scheme == "https" {
		return 443
	}

	return 80
}

// EnsureScheme -
func (e EndpointAddress) EnsureScheme() string {
	address := string(e)
	return ensureScheme(address)
}

func ensureScheme(address string) string {
	if strings.HasPrefix(address, "http://") || strings.HasPrefix(address, "https://") {
		return address
	}
	// The default scheme is http
	return "http://" + address
}

// NewEndpointAddress -
func NewEndpointAddress(host string, port int) *EndpointAddress {
	if !validation.IsDomainNotIP(host) {
		if net.ParseIP(host) == nil {
			return nil
		}
		if port < 0 || port > 65533 {
			return nil
		}
		ea := EndpointAddress(fmt.Sprintf("%s:%d", host, port))
		return &ea
	}

	_, err := url.Parse(ensureScheme(host))
	if err != nil {
		return nil
	}
	ea := EndpointAddress(host)
	return &ea
}

//ThirdComponentEndpointStatus endpoint status
type ThirdComponentEndpointStatus struct {
	// The address including the port number.
	Address EndpointAddress `json:"address"`
	// Then Name of the Endpoint.
	// +optional
	Name string `json:"name"`
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
