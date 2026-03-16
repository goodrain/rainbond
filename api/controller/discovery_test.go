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

package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetClusterScopedResourceTypes(t *testing.T) {
	// Initialize k8s component with fake clientset for testing
	testComponent := &k8s.Component{
		TestClientset: fake.NewSimpleClientset(),
	}
	// Set as default for handler to use
	originalComponent := k8s.Default()
	k8s.SetDefaultForTest(testComponent)
	defer k8s.SetDefaultForTest(originalComponent)

	r := chi.NewRouter()
	r.Get("/v2/platform/discovery/cluster-resources", GetClusterScopedResourceTypes)

	req := httptest.NewRequest("GET", "/v2/platform/discovery/cluster-resources", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
