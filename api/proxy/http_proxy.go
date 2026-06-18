// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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

package proxy

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// HTTPProxy HTTPProxy
type HTTPProxy struct {
	name      string
	endpoints EndpointList
	lb        LoadBalance
	client    *http.Client
}

// proxyErrorResponse is the JSON body returned when the proxy cannot reach
// the upstream endpoint.  Keeping it small avoids information leaks while
// still giving callers a structured response instead of a bare status code.
type proxyErrorResponse struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Detail  string `json:"detail,omitempty"`
}

// writeProxyError sends a structured JSON error response for proxy failures.
func writeProxyError(w http.ResponseWriter, statusCode int, msg, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	resp := proxyErrorResponse{Code: statusCode, Msg: msg, Detail: detail}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logrus.Errorf("failed to encode proxy error response: %v", err)
	}
}

// Proxy http proxy
func (h *HTTPProxy) Proxy(w http.ResponseWriter, r *http.Request) {
	endpoint := h.lb.Select(r, h.endpoints)
	endURL, err := url.Parse(endpoint.GetHTTPAddr())
	if err != nil {
		logrus.Errorf("parse endpoint url error,%s", err.Error())
		writeProxyError(w, http.StatusBadGateway, "upstream endpoint unavailable", "failed to parse endpoint address")
		return
	}
	if endURL.Scheme == "" {
		endURL.Scheme = "http"
	}
	proxy := httputil.NewSingleHostReverseProxy(endURL)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logrus.Errorf("proxy request to %s failed: %v", endURL.String(), err)
		writeProxyError(w, http.StatusBadGateway, "upstream service unreachable", err.Error())
	}
	proxy.ServeHTTP(w, r)
}

// UpdateEndpoints 更新端点
func (h *HTTPProxy) UpdateEndpoints(endpoints ...string) {
	ends := []string{}
	for _, end := range endpoints {
		if kv := strings.Split(end, "=>"); len(kv) > 1 {
			ends = append(ends, kv[1])
		} else {
			ends = append(ends, end)
		}
	}
	h.endpoints = CreateEndpoints(ends)
}

// Do do proxy
func (h *HTTPProxy) Do(r *http.Request) (*http.Response, error) {
	endpoint := h.lb.Select(r, h.endpoints)
	if strings.HasPrefix(endpoint.String(), "http") {
		r.URL.Host = strings.Replace(endpoint.String(), "http://", "", 1)
	} else {
		r.URL.Host = endpoint.String()
	}
	//default is http
	r.URL.Scheme = "http"
	return h.client.Do(r)
}

func createHTTPProxy(name string, endpoints []string, lb LoadBalance) *HTTPProxy {
	ends := []string{}
	for _, end := range endpoints {
		if kv := strings.Split(end, "=>"); len(kv) > 1 {
			ends = append(ends, kv[1])
		} else {
			ends = append(ends, end)
		}
	}
	if lb == nil {
		lb = NewRoundRobin()
	}
	timeout, _ := strconv.Atoi(os.Getenv("PROXY_TIMEOUT"))
	if timeout == 0 {
		timeout = 10
	}
	var netTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	client := &http.Client{
		Transport: netTransport,
		Timeout:   time.Second * time.Duration(timeout),
	}
	return &HTTPProxy{name, CreateEndpoints(ends), lb, client}
}
