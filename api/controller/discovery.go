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

	"github.com/goodrain/rainbond/api/handler"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
)

// GetClusterScopedResourceTypes returns all cluster-scoped Kubernetes resource types
func GetClusterScopedResourceTypes(w http.ResponseWriter, r *http.Request) {
	resourceTypes, err := handler.GetDiscoveryHandler().GetClusterScopedResourceTypes()
	if err != nil {
		logrus.Errorf("failed to discover cluster resources: %v", err)
		httputil.ReturnError(r, w, 500, "failed to discover cluster resources: "+err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, resourceTypes)
}
