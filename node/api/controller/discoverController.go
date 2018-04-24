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

	"github.com/Sirupsen/logrus"
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/util"
	"github.com/pquerna/ffjson/ffjson"
)

//ServiceDiscover service discover service
func ServiceDiscover(w http.ResponseWriter, r *http.Request) {
	serviceInfo := chi.URLParam(r, "service_name")
	//eg: serviceInfo := test_gr123456_201711031246
	sds, err := discoverService.DiscoverService(serviceInfo)
	if err != nil {
		err.Handle(r, w)
		return
	}
	sdsJ, errJ := ffjson.Marshal(sds)
	if errJ != nil {
		util.CreateAPIHandleError(500, errJ).Handle(r, w)
		return
	}
	w.WriteHeader(200)
	w.Write([]byte(sdsJ))
}

//ListenerDiscover ListenerDiscover
func ListenerDiscover(w http.ResponseWriter, r *http.Request) {
	tenantService := chi.URLParam(r, "tenant_service")
	serviceNodes := chi.URLParam(r, "service_nodes")
	lds, err := discoverService.DiscoverListeners(tenantService, serviceNodes)
	if err != nil {
		err.Handle(r, w)
		return
	}
	ldsJ, errJ := ffjson.Marshal(lds)
	if errJ != nil {
		util.CreateAPIHandleError(500, errJ).Handle(r, w)
		return
	}
	w.WriteHeader(200)
	w.Write(ldsJ)
}

//ClusterDiscover ClusterDiscover
func ClusterDiscover(w http.ResponseWriter, r *http.Request) {
	tenantService := chi.URLParam(r, "tenant_service")
	serviceNodes := chi.URLParam(r, "service_nodes")
	cds, err := discoverService.DiscoverClusters(tenantService, serviceNodes)
	if err != nil {
		err.Handle(r, w)
		return
	}
	cdsJ, errJ := ffjson.Marshal(cds)
	if errJ != nil {
		util.CreateAPIHandleError(500, errJ).Handle(r, w)
		return
	}
	w.WriteHeader(200)
	w.Write(cdsJ)
}

//RoutesDiscover RoutesDiscover
func RoutesDiscover(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "tenant_id")
	serviceNodes := chi.URLParam(r, "service_nodes")
	routeConfig := chi.URLParam(r, "route_config")
	logrus.Debugf("route_config is %s, namespace %s, serviceNodes %s", routeConfig, namespace, serviceNodes)
	w.WriteHeader(200)
}

//ResourcesEnv ResourcesEnv
func ResourcesEnv(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "tenant_id")
	sourceAlias := chi.URLParam(r, "source_alias")
	envName := chi.URLParam(r, "env_name")
	ss, err := discoverService.ToolsGetSourcesEnv(namespace, sourceAlias, envName)
	if err != nil {
		err.Handle(r, w)
		return
	}
	w.WriteHeader(200)
	w.Write(ss)
}

//UserDefineResources UserDefineResources
func UserDefineResources(w http.ResponseWriter, r *http.Request) {
	return
}
