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

// GetStorageOverview returns aggregated storage information including PVs and StorageClasses
func GetStorageOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := handler.GetStorageHandler().GetStorageOverview()
	if err != nil {
		logrus.Errorf("failed to get storage overview: %v", err)
		httputil.ReturnError(r, w, 500, "failed to get storage overview: "+err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, overview)
}

// ListStorageClasses returns all StorageClasses in the cluster
func ListStorageClasses(w http.ResponseWriter, r *http.Request) {
	storageClasses, err := handler.GetStorageHandler().ListStorageClasses()
	if err != nil {
		logrus.Errorf("failed to list storage classes: %v", err)
		httputil.ReturnError(r, w, 500, "failed to list storage classes: "+err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, storageClasses)
}

// ListPersistentVolumes returns all PersistentVolumes in the cluster
func ListPersistentVolumes(w http.ResponseWriter, r *http.Request) {
	pvs, err := handler.GetStorageHandler().ListPersistentVolumes()
	if err != nil {
		logrus.Errorf("failed to list persistent volumes: %v", err)
		httputil.ReturnError(r, w, 500, "failed to list persistent volumes: "+err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, pvs)
}
