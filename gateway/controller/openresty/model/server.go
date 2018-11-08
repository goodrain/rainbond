package model

// Server sets configuration for a virtual server...
type Server struct {
	Listen           string // Default: listen *:80 | *:8000; Sets the address and port for IP, or the path for a UNIX-domain socket on which the server will accept requests
	ServerName       string // Sets names of a virtual server
	KeepaliveTimeout Time   // Default 60s. Sets a timeout during which an idle keepalive connection to an upstream server will stay open.
	DefaultType      string // Defines the default MIME type of a response.
	Charset          string // Adds the specified charset to the “Content-Type” response header field.
	ServerTokens     bool   // Enables or disables emitting nginx version on error pages and in the “Server” response header field.

	ProxyConnectTimeout Time
	ProxyTimeout        Time
	ProxyPass           string

	SSLCertificate    string // Specifies a file with the certificate in the PEM format.
	SSLCertificateKey string // Specifies a file with the secret key in the PEM format.

	Return        Return
	Rewrites      []Rewrite

	Locations []Location
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
	Path string
	Rewrites []Rewrite
	Return Return
	ProxyPass string // Sets the protocol and address of a proxied server and an optional URI to which a location should be mapped
}
