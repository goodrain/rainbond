// RAINBOND, Application Management Platform
// Copyright (C) 2022-2022 Goodrain Co., Ltd.

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
	"fmt"
	"github.com/goodrain/rainbond/api/handler"
	apimodel "github.com/goodrain/rainbond/api/model"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	dbmodel "github.com/goodrain/rainbond/db/model"
	httputil "github.com/goodrain/rainbond/util/http"
	"net/http"
)

// K8sAttributeController -
type K8sAttributeController struct{}

// K8sAttributes -
func (k *K8sAttributeController) K8sAttributes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		k.createK8sAttributes(w, r)
	case "PUT":
		k.updateK8sAttributes(w, r)
	case "DELETE":
		k.deleteK8sAttributes(w, r)
	case "GET":
		k.getK8sAttributes(w, r)
	}
}

func (k *K8sAttributeController) getK8sAttributes(w http.ResponseWriter, r *http.Request) {
	componentID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	var k8sAttrGet apimodel.ComponentK8sAttributeGet
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &k8sAttrGet, nil); !ok {
		httputil.ReturnBcodeError(r, w, fmt.Errorf("k8s attributes is not valid"))
		return
	}
	atts, err := handler.GetServiceManager().GetK8sAttribute(componentID, k8sAttrGet.Name)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	if k8sAttrGet.Arch != "" {
		var attsValue string
		if atts != nil {
			attsValue, err = handler.GetServiceManager().ReviseAttributeAffinityByArch(atts.AttributeValue, k8sAttrGet.Arch)
			atts.AttributeValue = attsValue
		} else {
			attsValue, err = handler.GetServiceManager().ReviseAttributeAffinityByArch("", k8sAttrGet.Arch)
			atts = &dbmodel.ComponentK8sAttributes{
				AttributeValue: attsValue,
			}
		}
		if err != nil {
			httputil.ReturnBcodeError(r, w, err)
			return
		}
	}

	httputil.ReturnSuccess(r, w, atts)
}

func (k *K8sAttributeController) createK8sAttributes(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)
	componentID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	var k8sAttr apimodel.ComponentK8sAttribute
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &k8sAttr, nil); !ok {
		httputil.ReturnBcodeError(r, w, fmt.Errorf("k8s attributes is not valid"))
		return
	}
	if err := handler.GetServiceManager().CreateK8sAttribute(tenantID, componentID, &k8sAttr); err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (k *K8sAttributeController) updateK8sAttributes(w http.ResponseWriter, r *http.Request) {
	componentID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	var k8sAttr apimodel.ComponentK8sAttribute
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &k8sAttr, nil); !ok {
		httputil.ReturnBcodeError(r, w, fmt.Errorf("k8s attributes is not valid"))
		return
	}
	if err := handler.GetServiceManager().UpdateK8sAttribute(componentID, &k8sAttr); err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (k *K8sAttributeController) deleteK8sAttributes(w http.ResponseWriter, r *http.Request) {
	componentID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	var req apimodel.DeleteK8sAttributeReq
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil); !ok {
		httputil.ReturnBcodeError(r, w, fmt.Errorf("k8s attributes is not valid"))
		return
	}
	if err := handler.GetServiceManager().DeleteK8sAttribute(componentID, req.Name); err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
