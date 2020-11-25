// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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
	"encoding/json"
	"net/http"
	"os"
	"path"
	"runtime"

	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"

	httputil "github.com/goodrain/rainbond/util/http"
)

//CreateLocalVolume crete local volume dir
func CreateLocalVolume(w http.ResponseWriter, r *http.Request) {
	var requestopt = make(map[string]string)
	if err := json.NewDecoder(r.Body).Decode(&requestopt); err != nil {
		w.WriteHeader(400)
		return
	}
	tenantID := requestopt["tenant_id"]
	serviceID := requestopt["service_id"]
	pvcName := requestopt["pvcname"]
	var volumeHostPath = ""
	localPath := os.Getenv("LOCAL_DATA_PATH")
	if runtime.GOOS == "windows" {
		if localPath == "" {
			localPath = `c:\`
		}
	} else {
		if localPath == "" {
			localPath = "/grlocaldata"
		}
	}
	volumeHostPath = path.Join(localPath, "tenant", tenantID, "service", serviceID, pvcName)
	volumePath, volumeok := requestopt["volume_path"]
	podName, podok := requestopt["pod_name"]
	if volumeok && podok {
		volumeHostPath = path.Join(localPath, "tenant", tenantID, "service", serviceID, volumePath, podName)
	}
	if err := util.CheckAndCreateDirByMode(volumeHostPath, 0777); err != nil {
		logrus.Errorf("check and create dir %s error %s", volumeHostPath, err.Error())
		w.WriteHeader(500)
		return
	}
	httputil.ReturnSuccess(r, w, map[string]string{"path": volumeHostPath})
}

// DeleteLocalVolume delete local volume dir
func DeleteLocalVolume(w http.ResponseWriter, r *http.Request) {
	var requestopt = make(map[string]string)
	if err := json.NewDecoder(r.Body).Decode(&requestopt); err != nil {
		w.WriteHeader(400)
		return
	}
	path := requestopt["path"]

	if err := os.RemoveAll(path); err != nil {
		logrus.Errorf("path: %s; remove pv path: %v", path, err)
		w.WriteHeader(500)
	}

	httputil.ReturnSuccess(r, w, nil)
}
