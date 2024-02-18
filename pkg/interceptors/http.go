// RAINBOND, Application Management Platform
// Copyright (C) 2021-2024 Goodrain Co., Ltd.

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

package interceptors

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/goodrain/rainbond/pkg/component/etcd"
	"github.com/goodrain/rainbond/pkg/component/grpc"
	"github.com/goodrain/rainbond/pkg/component/hubregistry"
	"github.com/goodrain/rainbond/pkg/component/mq"
	"github.com/goodrain/rainbond/pkg/component/prom"
	"net/http"
	"strings"
	"time"
)

// Recoverer -
func Recoverer(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil && rvr != http.ErrAbortHandler {
				handleServiceUnavailable(w, r)
			}
		}()

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

// isNilPointerException Check if the panic is a nil pointer exception
func isNilPointerException(p interface{}) bool {
	if p == nil {
		return false
	}

	errMsg := fmt.Sprintf("%v", p)
	return strings.Contains(errMsg, "runtime error: invalid memory address or nil pointer dereference") || strings.Contains(errMsg, "runtime error: slice bounds out of range")
}

// handleServiceUnavailable -
func handleServiceUnavailable(w http.ResponseWriter, r *http.Request) {
	// Additional information about why etcd service is not available
	errorMessage := "部分服务不可用"

	if etcd.Default().EtcdClient == nil {
		errorMessage = "Etcd 服务不可用"
	} else if grpc.Default().StatusClient == nil {
		errorMessage = "worker 服务不可用"
	} else if hubregistry.Default().RegistryCli == nil {
		errorMessage = "私有镜像仓库 服务不可用"
	} else if mq.Default().MqClient == nil {
		errorMessage = "mq 服务不可用"
	} else if prom.Default().PrometheusCli == nil {
		errorMessage = "monitor 服务不可用"
	}

	// Create a response JSON
	response := map[string]interface{}{
		"error": errorMessage,
	}

	// Convert the response to JSON
	responseJSON, err := json.Marshal(response)
	if err != nil {
		// Handle JSON marshaling error
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Set appropriate headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)

	// Write the JSON response to the client
	_, _ = w.Write(responseJSON)
}

// Timeout -
func Timeout(timeout time.Duration) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "pods/logs") {
				timeout = 1 * time.Hour
			}
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer func() {
				cancel()
				if ctx.Err() == context.DeadlineExceeded {
					w.WriteHeader(http.StatusGatewayTimeout)
				}
			}()

			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}
