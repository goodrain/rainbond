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

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
)

// ListClusterResources lists all resources of a specific cluster-scoped type
func ListClusterResources(w http.ResponseWriter, r *http.Request) {
	group := chi.URLParam(r, "group")
	version := chi.URLParam(r, "version")
	resource := chi.URLParam(r, "resource")

	if group == "core" {
		group = "" // Core API group is empty string
	}

	resources, err := handler.GetClusterResourceHandler().ListClusterResources(group, version, resource)
	if err != nil {
		logrus.Errorf("failed to list cluster resources: %v", err)
		httputil.ReturnError(r, w, 500, "failed to list cluster resources: "+err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, resources)
}

// GetClusterResource gets a specific cluster-scoped resource by name
func GetClusterResource(w http.ResponseWriter, r *http.Request) {
	group := chi.URLParam(r, "group")
	version := chi.URLParam(r, "version")
	resource := chi.URLParam(r, "resource")
	name := chi.URLParam(r, "name")

	if group == "core" {
		group = "" // Core API group is empty string
	}

	res, err := handler.GetClusterResourceHandler().GetClusterResource(group, version, resource, name)
	if err != nil {
		logrus.Errorf("failed to get cluster resource: %v", err)
		httputil.ReturnError(r, w, 404, "resource not found: "+err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, res)
}
