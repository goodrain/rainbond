// RAINBOND, Application Management Platform
// Copyright (C) 2021-2024 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package interceptors

import (
	"net/http"
	"testing"
	"time"
)

// capability_id: rainbond.k8s-resource.deletion-request-timeout
func TestRequestTimeout(t *testing.T) {
	defaultTimeout := 5 * time.Second
	tests := []struct {
		name   string
		method string
		path   string
		want   time.Duration
	}{
		{
			name:   "pod logs use the long timeout",
			method: http.MethodGet,
			path:   "/v2/tenants/team/services/service/pods/pod/logs",
			want:   time.Hour,
		},
		{
			name:   "plugin streams use the long timeout",
			method: http.MethodGet,
			path:   "/v2/platform/backend/plugins/example/events",
			want:   time.Hour,
		},
		{
			name:   "event streams use the long timeout",
			method: http.MethodGet,
			path:   "/v2/events/event-1/stream",
			want:   time.Hour,
		},
		{
			name:   "event log history keeps the default timeout",
			method: http.MethodGet,
			path:   "/v2/events/event-1/log",
			want:   defaultTimeout,
		},
		{
			name:   "event stream suffix is matched precisely",
			method: http.MethodGet,
			path:   "/v2/events/event-1/stream/archive",
			want:   defaultTimeout,
		},
		{
			name:   "single Kubernetes resource deletion outlives the global timeout",
			method: http.MethodDelete,
			path:   "/v2/cluster/k8s-resource",
			want:   75 * time.Second,
		},
		{
			name:   "batch Kubernetes resource deletion outlives the global timeout",
			method: http.MethodDelete,
			path:   "/v2/cluster/batch-k8s-resource",
			want:   75 * time.Second,
		},
		{
			name:   "Kubernetes resource creation keeps the default timeout",
			method: http.MethodPost,
			path:   "/v2/cluster/k8s-resource",
			want:   defaultTimeout,
		},
		{
			name:   "Kubernetes resource deletion preview keeps the default timeout",
			method: http.MethodPost,
			path:   "/v2/cluster/k8s-resource-deletions/preview",
			want:   defaultTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := requestTimeout(tt.method, tt.path, defaultTimeout); got != tt.want {
				t.Fatalf("requestTimeout(%q, %q) = %s, want %s", tt.method, tt.path, got, tt.want)
			}
		})
	}
}
