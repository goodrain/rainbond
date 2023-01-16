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
	"strings"
	"sync/atomic"

	"github.com/sirupsen/logrus"
)

// ContextKey context key
type ContextKey string

// RoundRobin round robin loadBalance impl
type RoundRobin struct {
	ops *uint64
}

// LoadBalance LoadBalance
type LoadBalance interface {
	Select(r *http.Request, endpoints EndpointList) Endpoint
}

// Endpoint Endpoint
type Endpoint string

func (e Endpoint) String() string {
	return string(e)
}

// GetName get endpoint name
func (e Endpoint) GetName() string {
	if kv := strings.Split(string(e), "=>"); len(kv) > 1 {
		return kv[0]
	}
	return string(e)
}

// GetAddr get addr
func (e Endpoint) GetAddr() string {
	if kv := strings.Split(string(e), "=>"); len(kv) > 1 {
		return kv[1]
	}
	return string(e)
}

// GetHTTPAddr get http url
func (e Endpoint) GetHTTPAddr() string {
	if kv := strings.Split(string(e), "=>"); len(kv) > 1 {
		return withScheme(kv[1])
	}
	return withScheme(string(e))
}

func withScheme(s string) string {
	if strings.HasPrefix(s, "http") {
		return s
	}
	return "http://" + s
}

// EndpointList EndpointList
type EndpointList []Endpoint

// Len Len
func (e *EndpointList) Len() int {
	return len(*e)
}

// Add Add
func (e *EndpointList) Add(endpoints ...string) {
	for _, end := range endpoints {
		*e = append(*e, Endpoint(end))
	}
}

// Delete Delete
func (e *EndpointList) Delete(endpoints ...string) {
	var new EndpointList
	for _, endpoint := range endpoints {
		for _, old := range *e {
			if string(old) != endpoint {
				new = append(new, old)
			}
		}
	}
	*e = new
}

// Selec Selec
func (e *EndpointList) Selec(i int) Endpoint {
	return (*e)[i]
}

// HaveEndpoint Whether or not there is a endpoint
func (e *EndpointList) HaveEndpoint(endpoint string) bool {
	for _, en := range *e {
		if en.String() == endpoint {
			return true
		}
	}
	return false
}

// CreateEndpoints CreateEndpoints
func CreateEndpoints(endpoints []string) EndpointList {
	var epl EndpointList
	for _, e := range endpoints {
		epl = append(epl, Endpoint(e))
	}
	return epl
}

// NewRoundRobin create a RoundRobin
func NewRoundRobin() LoadBalance {
	var ops uint64
	ops = 0
	return RoundRobin{
		ops: &ops,
	}
}

// Select select a server from servers using RoundRobin
func (rr RoundRobin) Select(r *http.Request, endpoints EndpointList) Endpoint {
	l := uint64(endpoints.Len())
	if 0 >= l {
		return ""
	}
	selec := int(atomic.AddUint64(rr.ops, 1) % l)
	return endpoints.Selec(selec)
}

// SelectBalance 选择性负载均衡
type SelectBalance struct {
	hostIDMap map[string]string
}

// NewSelectBalance  创建选择性负载均衡
func NewSelectBalance() *SelectBalance {
	return &SelectBalance{
		hostIDMap: map[string]string{"local": "rbd-eventlog:6363"},
	}
}

// Select 负载
func (s *SelectBalance) Select(r *http.Request, endpoints EndpointList) Endpoint {
	if r.URL == nil {
		return Endpoint(s.hostIDMap["local"])
	}
	id2ip := map[string]string{"local": "rbd-eventlog:6363"}
	for _, end := range endpoints {
		if kv := strings.Split(string(end), "=>"); len(kv) > 1 {
			id2ip[kv[0]] = kv[1]
		}
	}

	if r.URL != nil {
		hostID := r.URL.Query().Get("host_id")
		if hostID == "" {
			hostIDFromContext := r.Context().Value(ContextKey("host_id"))
			if hostIDFromContext != nil {
				hostID = hostIDFromContext.(string)
			}
		}
		if e, ok := id2ip[hostID]; ok {
			logrus.Infof("[lb selelct] find host %s from name %s success", e, hostID)
			return Endpoint(e)
		}
	}

	if len(endpoints) > 0 {
		logrus.Infof("default endpoint is %s", endpoints[len(endpoints)-1])
		return endpoints[len(endpoints)-1]
	}

	return Endpoint(s.hostIDMap["local"])
}
