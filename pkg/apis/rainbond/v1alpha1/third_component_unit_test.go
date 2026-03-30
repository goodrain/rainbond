package v1alpha1

import "testing"

// capability_id: rainbond.third-component.identity-fields
func TestThirdComponentIdentityHelpers(t *testing.T) {
	component := &ThirdComponent{}
	component.Name = "demo"
	component.Namespace = "default"
	endpoint := &ThirdComponentEndpointStatus{Address: EndpointAddress("1.2.3.4:8080")}

	if got := component.GetComponentID(); got != "demo" {
		t.Fatalf("unexpected component id: %q", got)
	}
	if got := component.GetNamespaceName(); got != "default/demo" {
		t.Fatalf("unexpected namespace/name: %q", got)
	}
	if got := component.GetEndpointID(endpoint); got != "default/demo/1.2.3.4:8080" {
		t.Fatalf("unexpected endpoint id: %q", got)
	}
}

// capability_id: rainbond.third-component.probe-required
func TestThirdComponentSpecNeedProbe(t *testing.T) {
	spec := ThirdComponentSpec{}
	if spec.NeedProbe() {
		t.Fatal("did not expect probe without configuration")
	}

	spec.Probe = &Probe{}
	if spec.NeedProbe() {
		t.Fatal("did not expect probe without static endpoints")
	}

	spec.EndpointSource.StaticEndpoints = []*ThirdComponentEndpoint{{Address: "1.2.3.4:8080"}}
	if !spec.NeedProbe() {
		t.Fatal("expected probe when static endpoints are configured")
	}
}

// capability_id: rainbond.third-component.static-endpoints-detect
func TestThirdComponentSpecIsStaticEndpoints(t *testing.T) {
	spec := ThirdComponentSpec{}
	if spec.IsStaticEndpoints() {
		t.Fatal("did not expect static endpoints")
	}
	spec.EndpointSource.StaticEndpoints = []*ThirdComponentEndpoint{{Address: "example.com"}}
	if !spec.IsStaticEndpoints() {
		t.Fatal("expected static endpoints to be detected")
	}
}

// capability_id: rainbond.third-component.probe-equals
func TestProbeEquals(t *testing.T) {
	left := &Probe{
		TimeoutSeconds:   3,
		PeriodSeconds:    10,
		SuccessThreshold: 1,
		FailureThreshold: 3,
		Handler:          Handler{TCPSocket: &TCPSocketAction{}},
	}
	right := &Probe{
		TimeoutSeconds:   3,
		PeriodSeconds:    10,
		SuccessThreshold: 1,
		FailureThreshold: 3,
		Handler:          Handler{TCPSocket: &TCPSocketAction{}},
	}
	if !left.Equals(right) {
		t.Fatal("expected probes to be equal")
	}

	right.TimeoutSeconds = 5
	if left.Equals(right) {
		t.Fatal("expected timeout difference to break equality")
	}
}

// capability_id: rainbond.third-component.http-get-equals
func TestHTTPGetActionEquals(t *testing.T) {
	left := &HTTPGetAction{
		Path: "/healthz",
		HTTPHeaders: []HTTPHeader{
			{Name: "X-A", Value: "1"},
			{Name: "X-B", Value: "2"},
		},
	}
	right := &HTTPGetAction{
		Path: "/healthz",
		HTTPHeaders: []HTTPHeader{
			{Name: "X-B", Value: "2"},
			{Name: "X-A", Value: "1"},
		},
	}
	if !left.Equals(right) {
		t.Fatal("expected header order to be ignored")
	}

	right.HTTPHeaders[1].Value = "9"
	if left.Equals(right) {
		t.Fatal("expected different header value to break equality")
	}
}

// capability_id: rainbond.third-component.handler-equals
func TestHandlerEquals(t *testing.T) {
	if !(&Handler{}).Equals(&Handler{}) {
		t.Fatal("expected empty handlers to be equal")
	}
	if (&Handler{HTTPGet: &HTTPGetAction{Path: "/a"}}).Equals(&Handler{TCPSocket: &TCPSocketAction{}}) {
		t.Fatal("did not expect different handler types to be equal")
	}
}

// capability_id: rainbond.third-component.endpoint-address-port
func TestEndpointAddressGetPort(t *testing.T) {
	if got := EndpointAddress("10.0.0.1:8080").GetPort(); got != 8080 {
		t.Fatalf("expected 8080, got %d", got)
	}
	if got := EndpointAddress("example.com").GetPort(); got != 80 {
		t.Fatalf("expected default http port 80, got %d", got)
	}
	if got := EndpointAddress("https://example.com").GetPort(); got != 443 {
		t.Fatalf("expected https port 443, got %d", got)
	}
	if got := EndpointAddress("example.com:8443").GetPort(); got != 8443 {
		t.Fatalf("expected explicit port 8443, got %d", got)
	}
}

// capability_id: rainbond.third-component.endpoint-address-ip
func TestEndpointAddressGetIP(t *testing.T) {
	if got := EndpointAddress("10.0.0.1:8080").GetIP(); got != "10.0.0.1" {
		t.Fatalf("unexpected ip: %q", got)
	}
	if got := EndpointAddress("example.com:8080").GetIP(); got != "1.1.1.1" {
		t.Fatalf("expected domain endpoint to map to sentinel ip, got %q", got)
	}
}

// capability_id: rainbond.third-component.endpoint-address-scheme
func TestEndpointAddressEnsureScheme(t *testing.T) {
	if got := EndpointAddress("example.com").EnsureScheme(); got != "http://example.com" {
		t.Fatalf("unexpected ensured scheme: %q", got)
	}
	if got := EndpointAddress("https://example.com").EnsureScheme(); got != "https://example.com" {
		t.Fatalf("unexpected preserved scheme: %q", got)
	}
}

// capability_id: rainbond.third-component.legacy-endpoint-port
func TestThirdComponentEndpointGetPortAndIP(t *testing.T) {
	endpoint := &ThirdComponentEndpoint{Address: "10.0.0.1:8080"}
	if endpoint.GetIP() != "10.0.0.1" || endpoint.GetPort() != 8080 {
		t.Fatalf("unexpected endpoint split: ip=%q port=%d", endpoint.GetIP(), endpoint.GetPort())
	}
}

// capability_id: rainbond.third-component.endpoint-address-construct
func TestNewEndpointAddress(t *testing.T) {
	if addr := NewEndpointAddress("1.2.3.4", 8080); addr == nil || string(*addr) != "1.2.3.4:8080" {
		t.Fatalf("unexpected ip endpoint address: %+v", addr)
	}
	if addr := NewEndpointAddress("example.com", 0); addr == nil || string(*addr) != "example.com" {
		t.Fatalf("unexpected domain endpoint address: %+v", addr)
	}
	if addr := NewEndpointAddress("not a host", 8080); addr != nil {
		t.Fatalf("expected invalid host to fail, got %+v", addr)
	}
}
