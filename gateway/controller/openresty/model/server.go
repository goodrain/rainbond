package model

import (
	"fmt"
	"strings"

	"github.com/goodrain/rainbond/gateway/annotations/proxy"
	"github.com/goodrain/rainbond/gateway/annotations/rewrite"
	v1 "github.com/goodrain/rainbond/gateway/v1"
)

// Server sets configuration for a virtual server...
type Server struct {
	Listen                  string // DefaultType: listen *:80 | *:8000; Sets the address and port for IP, or the path for a UNIX-domain socket on which the server will accept requests
	Protocol                string
	Root                    string // Sets the root directory for requests.
	ServerName              string // Sets names of a virtual server
	KeepaliveTimeout        Time   // DefaultType 60s. Sets a timeout during which an idle keepalive connection to an upstream server will stay open.
	DefaultType             string // Defines the default MIME type of a response.
	Charset                 string // Adds the specified charset to the “Content-Type” response header field.
	ServerTokens            bool   // Enables or disables emitting nginx version on error pages and in the “Server” response header field.
	ClientMaxBodySize       Size   // Sets the maximum allowed size of the client request body
	ChunkedTransferEncoding bool   // Allows disabling chunked transfer encoding in HTTP/1.1
	ProxyConnectTimeout     Time
	ProxyTimeout            Time
	ProxyPass               string
	SSLProtocols            string
	SSLCertificate          string // Specifies a file with the certificate in the PEM format.
	SSLCertificateKey       string // Specifies a file with the secret key in the PEM format.
	EnableSSLStapling       bool
	ForceSSLRedirect        bool
	Return                  Return
	Rewrites                []Rewrite
	Locations               []*Location
	OptionValue             map[string]string
	UpstreamName            string //used for tcp and udp server

	// Sets the number of datagrams expected from the proxied server in response
	// to the client request if the UDP protocol is used.
	// http://nginx.org/en/docs/stream/ngx_stream_proxy_module.html#proxy_responses
	// Default: 1
	ProxyStreamResponses int

	ProxyStreamTimeout             string
	ProxyStreamNextUpstream        bool   `json:"proxyStreamNextUpstream"`
	ProxyStreamNextUpstreamTimeout string `json:"proxyStreamNextUpstreamTimeout"`
	ProxyStreamNextUpstreamTries   int    `json:"proxyStreamNextUpstreamTries"`
	//proxy protocol for tcp real ip
	ProxyProtocol ProxyProtocol
}

// ProxyProtocol describes the proxy protocol configuration
type ProxyProtocol struct {
	Decode bool `json:"decode"`
	Encode bool `json:"encode"`
}

//Validation validation nginx parameters
func (s *Server) Validation() error {
	if s.ServerName != "" && strings.Contains(s.ServerName, " ") {
		return fmt.Errorf("servername %s is valid", s.ServerName)
	}
	if s.ProxyStreamTimeout == "" {
		s.ProxyStreamTimeout = "600s"
	}
	if s.Protocol == "" {
		if s.ServerName != "" {
			s.Protocol = "HTTP"
		} else {
			s.Protocol = "TCP"
		}
	}
	for _, l := range s.Locations {
		if err := l.Validation(); err != nil {
			return fmt.Errorf("servername %s location is valid:%s", s.ServerName, err.Error())
		}
	}
	return nil
}

// FastCGIParam sets a parameter that should be passed to the FastCGI server.
type FastCGIParam struct {
	Param string
	Value string
}

// Rewrite matching request URI to replacement.
type Rewrite struct {
	Regex       string
	Replacement string
	Flag        string
}

// Return stops processing and returns the specified code to a client.
type Return struct {
	Code int
	Text string
	URL  string
}

// Location sets configuration depending on a request URI.
type Location struct {
	Path    string
	Rewrite rewrite.Config
	Return  Return
	// Sets the protocol and address of a proxied server and an optional URI to which a location should be mapped
	ProxyPass string
	// Sets the text that should be changed in the “Location” and “Refresh” header fields of a proxied server response
	// TODO: mv ProxyRedirect to Proxy
	ProxyRedirect string

	EnableMetrics    bool //Enables or disables monitor
	DisableAccessLog bool //disable or enables access log
	AccessLogPath    string
	ErrorLogPath     string
	DisableProxyPass bool
	//PathRewrite if true, path will not passed to the upstream
	PathRewrite   bool
	NameCondition map[string]*v1.Condition

	// Proxy contains information about timeouts and buffer sizes
	// to be used in connections against endpoints
	// +optional
	Proxy proxy.Config `json:"proxy,omitempty"`
}

//Validation validation nginx parameters
func (s *Location) Validation() error {
	if s.Path == "" {
		return fmt.Errorf("location path can not be empty")
	}
	if err := (&s.Proxy).Validation(); err != nil {
		return fmt.Errorf("location %s proxy config is valid %s", s.Path, err.Error())
	}
	return nil
}

// ProxySetHeader allows redefining or appending fields to the request header passed to the proxied server.
type ProxySetHeader struct {
	Field string
	Value string
}
