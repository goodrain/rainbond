/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package proxy

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/gateway/controller/config"
	extensions "k8s.io/api/extensions/v1beta1"

	"github.com/goodrain/rainbond/gateway/annotations/parser"
	"github.com/goodrain/rainbond/gateway/annotations/resolver"
)

// Config returns the proxy timeout to use in the upstream server/s
type Config struct {
	BodySize          int            `json:"bodySize"`
	ConnectTimeout    int               `json:"connectTimeout"`
	SendTimeout       int               `json:"sendTimeout"`
	ReadTimeout       int               `json:"readTimeout"`
	BuffersNumber     int               `json:"buffersNumber"`
	BufferSize        string            `json:"bufferSize"`
	CookieDomain      string            `json:"cookieDomain"`
	CookiePath        string            `json:"cookiePath"`
	NextUpstream      string            `json:"nextUpstream"`
	NextUpstreamTries int               `json:"nextUpstreamTries"`
	ProxyRedirectFrom string            `json:"proxyRedirectFrom"`
	ProxyRedirectTo   string            `json:"proxyRedirectTo"`
	RequestBuffering  string            `json:"requestBuffering"`
	ProxyBuffering    string            `json:"proxyBuffering"`
	SetHeaders        map[string]string `json:"setHeaders"`
}

func NewProxyConfig() Config {
	defBackend := config.NewDefault()
	return Config{
		BodySize:          defBackend.ProxyBodySize,
		ConnectTimeout:    defBackend.ProxyConnectTimeout,
		SendTimeout:       defBackend.ProxySendTimeout,
		ReadTimeout:       defBackend.ProxyReadTimeout,
		BuffersNumber:     defBackend.ProxyBuffersNumber,
		BufferSize:        defBackend.ProxyBufferSize,
		CookieDomain:      defBackend.ProxyCookieDomain,
		CookiePath:        defBackend.ProxyCookiePath,
		NextUpstream:      defBackend.ProxyNextUpstream,
		NextUpstreamTries: defBackend.ProxyNextUpstreamTries,
		RequestBuffering:  defBackend.ProxyRequestBuffering,
		ProxyRedirectFrom: defBackend.ProxyRedirectFrom,
		ProxyRedirectTo:   defBackend.ProxyRedirectTo,
		ProxyBuffering:    defBackend.ProxyBuffering,
		SetHeaders:        defBackend.ProxySetHeaders,
	}
}

// Equal tests for equality between two Configuration types
func (l1 *Config) Equal(l2 *Config) bool {
	if l1 == l2 {
		return true
	}
	if l1 == nil || l2 == nil {
		return false
	}
	if l1.BodySize != l2.BodySize {
		return false
	}
	if l1.ConnectTimeout != l2.ConnectTimeout {
		return false
	}
	if l1.SendTimeout != l2.SendTimeout {
		return false
	}
	if l1.ReadTimeout != l2.ReadTimeout {
		return false
	}
	if l1.BuffersNumber != l2.BuffersNumber {
		return false
	}
	if l1.BufferSize != l2.BufferSize {
		return false
	}
	if l1.CookieDomain != l2.CookieDomain {
		return false
	}
	if l1.CookiePath != l2.CookiePath {
		return false
	}
	if l1.NextUpstream != l2.NextUpstream {
		return false
	}
	if l1.NextUpstreamTries != l2.NextUpstreamTries {
		return false
	}
	if l1.RequestBuffering != l2.RequestBuffering {
		return false
	}
	if l1.ProxyRedirectFrom != l2.ProxyRedirectFrom {
		return false
	}
	if l1.ProxyRedirectTo != l2.ProxyRedirectTo {
		return false
	}
	if l1.ProxyBuffering != l2.ProxyBuffering {
		return false
	}
	// TODO: ProxySetHeaders

	return true
}

type proxy struct {
	r resolver.Resolver
}

// NewParser creates a new reverse proxy configuration annotation parser
func NewParser(r resolver.Resolver) parser.IngressAnnotation {
	return proxy{r}
}

// ParseAnnotations parses the annotations contained in the ingress
// rule used to configure upstream check parameters
func (a proxy) Parse(ing *extensions.Ingress) (interface{}, error) {
	defBackend := a.r.GetDefaultBackend()
	config := &Config{}

	var err error

	config.ConnectTimeout, err = parser.GetIntAnnotation("proxy-connect-timeout", ing)
	if err != nil {
		config.ConnectTimeout = defBackend.ProxyConnectTimeout
	}

	config.SendTimeout, err = parser.GetIntAnnotation("proxy-send-timeout", ing)
	if err != nil {
		config.SendTimeout = defBackend.ProxySendTimeout
	}

	config.ReadTimeout, err = parser.GetIntAnnotation("proxy-read-timeout", ing)
	if err != nil {
		config.ReadTimeout = defBackend.ProxyReadTimeout
	}

	config.BuffersNumber, err = parser.GetIntAnnotation("proxy-buffers-number", ing)
	if err != nil {
		config.BuffersNumber = defBackend.ProxyBuffersNumber
	}

	config.BufferSize, err = parser.GetStringAnnotation("proxy-buffer-size", ing)
	if err != nil {
		config.BufferSize = defBackend.ProxyBufferSize
	}

	config.CookiePath, err = parser.GetStringAnnotation("proxy-cookie-path", ing)
	if err != nil {
		config.CookiePath = defBackend.ProxyCookiePath
	}

	config.CookieDomain, err = parser.GetStringAnnotation("proxy-cookie-domain", ing)
	if err != nil {
		config.CookieDomain = defBackend.ProxyCookieDomain
	}

	config.BodySize, err = parser.GetIntAnnotation("proxy-body-size", ing)
	if err != nil {
		config.BodySize = defBackend.ProxyBodySize
	}

	config.NextUpstream, err = parser.GetStringAnnotation("proxy-next-upstream", ing)
	if err != nil {
		config.NextUpstream = defBackend.ProxyNextUpstream
	}

	config.NextUpstreamTries, err = parser.GetIntAnnotation("proxy-next-upstream-tries", ing)
	if err != nil {
		config.NextUpstreamTries = defBackend.ProxyNextUpstreamTries
	}

	config.RequestBuffering, err = parser.GetStringAnnotation("proxy-request-buffering", ing)
	if err != nil {
		config.RequestBuffering = defBackend.ProxyRequestBuffering
	}

	config.ProxyRedirectFrom, err = parser.GetStringAnnotation("proxy-redirect-from", ing)
	if err != nil {
		config.ProxyRedirectFrom = defBackend.ProxyRedirectFrom
	}

	config.ProxyRedirectTo, err = parser.GetStringAnnotation("proxy-redirect-to", ing)
	if err != nil {
		config.ProxyRedirectTo = defBackend.ProxyRedirectTo
	}

	config.ProxyBuffering, err = parser.GetStringAnnotation("proxy-buffering", ing)
	if err != nil {
		config.ProxyBuffering = defBackend.ProxyBuffering
	}

	config.SetHeaders = make(map[string]string)
	for k, v := range defBackend.ProxySetHeaders {
		config.SetHeaders[k] = v
	}
	setHeaders, err := parser.GetStringAnnotationWithPrefix("set-header-", ing)
	logrus.Debugf("set headers from anns: %+v", setHeaders)
	if err != nil {
		logrus.Warningf("Ingress Key: %s; error parsing set-header: %v",
			fmt.Sprintf("%s/%s", ing.GetNamespace(), ing.GetName()), err)
	}
	for k, v := range setHeaders {
		config.SetHeaders[k] = v
	}

	return config, nil
}
