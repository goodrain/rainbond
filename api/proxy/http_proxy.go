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
	"net/http"
	"net/http/httputil"
	"strings"
)

//HTTPProxy HTTPProxy
type HTTPProxy struct {
	name      string
	endpoints EndpointList
	lb        LoadBalance
}

//Proxy 代理
func (h *HTTPProxy) Proxy(w http.ResponseWriter, r *http.Request) {
	endpoint := h.lb.Select(r, h.endpoints)
	director := func(req *http.Request) {
		req = r
		req.URL.Scheme = "http"
		req.URL.Host = endpoint.String()
	}
	proxy := &httputil.ReverseProxy{Director: director}
	proxy.ServeHTTP(w, r)
}

//UpdateEndpoints 更新端点
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

//Do do proxy
func (h *HTTPProxy) Do(r *http.Request) (*http.Response, error) {
	endpoint := h.lb.Select(r, h.endpoints)
	if strings.HasPrefix(endpoint.String(), "http") {
		r.URL.Host = strings.Replace(endpoint.String(), "http://", "", 1)
	} else {
		r.URL.Host = endpoint.String()
	}
	return http.DefaultClient.Do(r)
}

func createHTTPProxy(name string, endpoints []string) *HTTPProxy {
	ends := []string{}
	for _, end := range endpoints {
		if kv := strings.Split(end, "=>"); len(kv) > 1 {
			ends = append(ends, kv[1])
		} else {
			ends = append(ends, end)
		}
	}
	return &HTTPProxy{name, CreateEndpoints(ends), NewRoundRobin()}
}
