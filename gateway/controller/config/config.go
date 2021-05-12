// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

package config

import (
	"github.com/goodrain/rainbond/gateway/defaults"
)

const (
	// http://nginx.org/en/docs/http/ngx_http_core_module.html#client_max_body_size
	// Sets the maximum allowed size of the client request body
	bodySize = 0

	// http://nginx.org/en/docs/ngx_core_module.html#error_log
	// Configures logging level [debug | info | notice | warn | error | crit | alert | emerg]
	// Log levels above are listed in the order of increasing severity
	errorLevel = "notice"

	// HTTP Strict Transport Security (often abbreviated as HSTS) is a security feature (HTTP header)
	// that tell browsers that it should only be communicated with using HTTPS, instead of using HTTP.
	// https://developer.mozilla.org/en-US/docs/Web/Security/HTTP_strict_transport_security
	// max-age is the time, in seconds, that the browser should remember that this site is only to be accessed using HTTPS.
	hstsMaxAge = "15724800"

	gzipTypes = "application/atom+xml application/javascript application/x-javascript application/json application/rss+xml application/vnd.ms-fontobject application/x-font-ttf application/x-web-app-manifest+json application/xhtml+xml application/xml font/opentype image/svg+xml image/x-icon text/css text/plain text/x-component"

	brotliTypes = "application/xml+rss application/atom+xml application/javascript application/x-javascript application/json application/rss+xml application/vnd.ms-fontobject application/x-font-ttf application/x-web-app-manifest+json application/xhtml+xml application/xml font/opentype image/svg+xml image/x-icon text/css text/plain text/x-component"

	logFormatStream = `[$time_local] $protocol $status $bytes_sent $bytes_received $session_time`

	// http://nginx.org/en/docs/http/ngx_http_ssl_module.html#ssl_buffer_size
	// Sets the size of the buffer used for sending data.
	// 4k helps NGINX to improve TLS Time To First Byte (TTTFB)
	// https://www.igvita.com/2013/12/16/optimizing-nginx-tls-time-to-first-byte/
	sslBufferSize = "4k"

	// Enabled ciphers list to enabled. The ciphers are specified in the format understood by the OpenSSL library
	// http://nginx.org/en/docs/http/ngx_http_ssl_module.html#ssl_ciphers
	sslCiphers = "ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-SHA384:ECDHE-RSA-AES256-SHA384:ECDHE-ECDSA-AES128-SHA256:ECDHE-RSA-AES128-SHA256"

	// SSL enabled protocols to use
	// http://nginx.org/en/docs/http/ngx_http_ssl_module.html#ssl_protocols
	sslProtocols = "TLSv1.2"

	// Time during which a client may reuse the session parameters stored in a cache.
	// http://nginx.org/en/docs/http/ngx_http_ssl_module.html#ssl_session_timeout
	sslSessionTimeout = "10m"

	// Size of the SSL shared cache between all worker processes.
	// http://nginx.org/en/docs/http/ngx_http_ssl_module.html#ssl_session_cache
	sslSessionCacheSize = "10m"

	// Parameters for a shared memory zone that will keep states for various keys.
	// http://nginx.org/en/docs/http/ngx_http_limit_conn_module.html#limit_conn_zone
	defaultLimitConnZoneVariable = "$binary_remote_addr"
)

// Configuration represents the content of nginx.conf file
type Configuration struct {
	defaults.Backend `json:",squash"`
}

// NewDefault returns the default nginx configuration
func NewDefault() Configuration {
	cfg := Configuration{
		Backend: defaults.Backend{
			ProxyBodySize:            bodySize,
			ProxyConnectTimeout:      5,
			ProxyReadTimeout:         60,
			ProxySendTimeout:         60,
			ProxyBuffersNumber:       4,
			ProxyBufferSize:          "4k",
			ProxyCookieDomain:        "off",
			ProxyCookiePath:          "off",
			ProxyNextUpstream:        "error timeout",
			ProxyNextUpstreamTries:   3,
			ProxyNextUpstreamTimeout: 0,
			ProxyRequestBuffering:    "on",
			ProxyRedirectFrom:        "off",
			ProxyRedirectTo:          "off",
			SSLRedirect:              true,
			CustomHTTPErrors:         []int{},
			WhitelistSourceRange:     []string{},
			SkipAccessLogURLs:        []string{},
			LimitRate:                0,
			LimitRateAfter:           0,
			ProxyBuffering:           "off",
			//defaut set header
			ProxySetHeaders: map[string]string{
				"Host":              "$best_http_host",
				"X-Real-IP":         "$remote_addr",
				"X-Forwarded-For":   "$remote_addr",
				"X-Forwarded-Host":  "$best_http_host",
				"X-Forwarded-Port":  "$pass_port",
				"X-Forwarded-Proto": "$pass_access_scheme",
				"X-Scheme":          "$pass_access_scheme",
				// mitigate HTTPoxy Vulnerability
				// https://www.nginx.com/blog/mitigating-the-httpoxy-vulnerability-with-nginx/
				"Proxy": "\"\"",
			},
		},
	}
	return cfg
}
