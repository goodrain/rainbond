package model

import (
	"github.com/goodrain/rainbond/gateway/annotations/proxy"
	"github.com/goodrain/rainbond/gateway/annotations/rewrite"
	"github.com/goodrain/rainbond/gateway/v1"
)

// Server sets configuration for a virtual server...
type Server struct {
	Listen                  string // DefaultType: listen *:80 | *:8000; Sets the address and port for IP, or the path for a UNIX-domain socket on which the server will accept requests
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
	SSLCertificate          string // Specifies a file with the certificate in the PEM format.
	SSLCertificateKey       string // Specifies a file with the secret key in the PEM format.
	ForceSSLRedirect        bool
	Return                  Return
	Rewrites                []Rewrite
	Locations               []*Location
	OptionValue             map[string]string
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
	DisableProxyPass bool
	//PathRewrite if true, path will not passed to the upstream
	PathRewrite   bool
	NameCondition map[string]*v1.Condition

	// Proxy contains information about timeouts and buffer sizes
	// to be used in connections against endpoints
	// +optional
	Proxy proxy.Config `json:"proxy,omitempty"`
}

// ProxySetHeader allows redefining or appending fields to the request header passed to the proxied server.
type ProxySetHeader struct {
	Field string
	Value string
}
