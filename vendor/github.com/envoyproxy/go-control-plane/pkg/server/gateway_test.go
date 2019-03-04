// Copyright 2018 Envoyproxy Authors
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package server_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/envoyproxy/go-control-plane/pkg/cache"
	"github.com/envoyproxy/go-control-plane/pkg/server"
)

type logger struct {
	t *testing.T
}

func (log logger) Infof(format string, args ...interface{})  { log.t.Logf(format, args...) }
func (log logger) Errorf(format string, args ...interface{}) { log.t.Logf(format, args...) }

func TestGateway(t *testing.T) {
	config := makeMockConfigWatcher()
	config.responses = map[string][]cache.Response{
		cache.ClusterType: []cache.Response{{
			Version:   "2",
			Resources: []cache.Resource{cluster},
		}},
		cache.RouteType: []cache.Response{{
			Version:   "3",
			Resources: []cache.Resource{route},
		}},
		cache.ListenerType: []cache.Response{{
			Version:   "4",
			Resources: []cache.Resource{listener},
		}},
	}
	gtw := server.HTTPGateway{Log: logger{t: t}, Server: server.NewServer(config, nil)}

	failCases := []struct {
		path   string
		body   io.Reader
		expect int
	}{
		{
			path:   "/hello/",
			expect: http.StatusNotFound,
		},
		{
			path:   "/v2/discovery:endpoints",
			expect: http.StatusBadRequest,
		},
		{
			path:   "/v2/discovery:endpoints",
			body:   iotest.TimeoutReader(strings.NewReader("hello")),
			expect: http.StatusBadRequest,
		},
		{
			path:   "/v2/discovery:endpoints",
			body:   strings.NewReader("hello"),
			expect: http.StatusBadRequest,
		},
		{
			// missing response
			path:   "/v2/discovery:endpoints",
			body:   strings.NewReader("{\"node\": {\"id\": \"test\"}}"),
			expect: http.StatusInternalServerError,
		},
	}
	for _, cs := range failCases {
		rr := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodPost, cs.path, cs.body)
		if err != nil {
			t.Fatal(err)
		}
		gtw.ServeHTTP(rr, req)
		if status := rr.Code; status != cs.expect {
			t.Errorf("handler returned wrong status: %d, want %d", status, cs.expect)
		}
	}

	for _, path := range []string{"/v2/discovery:clusters", "/v2/discovery:routes", "/v2/discovery:listeners"} {
		rr := httptest.NewRecorder()
		req, err := http.NewRequest(http.MethodPost, path, strings.NewReader("{\"node\": {\"id\": \"test\"}}"))
		if err != nil {
			t.Fatal(err)
		}
		gtw.ServeHTTP(rr, req)
		if status := rr.Code; status != 200 {
			t.Errorf("handler returned wrong status: %d, want %d", status, 200)
		}
	}
}
