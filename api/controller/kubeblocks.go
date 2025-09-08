// Copyright (C) 2014-2025 Goodrain Co., Ltd.
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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/sirupsen/logrus"
)

// KubeBlocksController -
type KubeBlocksController struct{}

// blockMechanicaBaseURL base URL for block-mechanica service
const blockMechanicaBaseURL = "http://block-mechanica.rbd-system.svc:80"

// GetSupportedDatabases get KubeBlocks supported databases list
func (c *KubeBlocksController) GetSupportedDatabases(w http.ResponseWriter, r *http.Request) {
	c.forwardRequest(w, r, "/v1/addons", "GET")
}

// GetStorageClasses get KubeBlocks storage classes list
func (c *KubeBlocksController) GetStorageClasses(w http.ResponseWriter, r *http.Request) {
	c.forwardRequest(w, r, "/v1/storageclasses", "GET")
}

// GetBackupRepos get KubeBlocks backup repositories list
func (c *KubeBlocksController) GetBackupRepos(w http.ResponseWriter, r *http.Request) {
	c.forwardRequest(w, r, "/v1/backuprepos", "GET")
}

// CreateCluster create KubeBlocks database cluster
func (c *KubeBlocksController) CreateCluster(w http.ResponseWriter, r *http.Request) {
	c.forwardRequest(w, r, "/v1/clusters", "POST")
}

// GetClusterConnectInfos get KubeBlocks clusters connection infos
func (c *KubeBlocksController) GetClusterConnectInfos(w http.ResponseWriter, r *http.Request) {
	c.forwardRequest(w, r, "/v1/clusters/connect-infos", "GET")
}

// GetClusterByID get KubeBlocks cluster by service ID
func (c *KubeBlocksController) GetClusterByID(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "service_id")
	c.forwardRequest(w, r, fmt.Sprintf("/v1/clusters/%s", serviceID), "GET")
}

// ExpansionCluster update KubeBlocks cluster
func (c *KubeBlocksController) ExpansionCluster(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "service_id")
	c.forwardRequest(w, r, fmt.Sprintf("/v1/clusters/%s", serviceID), "PUT")
}

// GetKubeBlocksComponentInfo get KubeBlocks component info by service ID
func (c *KubeBlocksController) GetKubeBlocksComponentInfo(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "service_id")
	c.forwardRequest(w, r, fmt.Sprintf("/v1/kubeblocks-component/%s", serviceID), "GET")
}

// UpdateClusterBackupSchedules update KubeBlocks cluster backup schedules
func (c *KubeBlocksController) UpdateClusterBackupSchedules(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "service_id")
	c.forwardRequest(w, r, fmt.Sprintf("/v1/clusters/%s/backup-schedules", serviceID), "PUT")
}

// CreateClusterBackup create KubeBlocks cluster backup
func (c *KubeBlocksController) CreateClusterBackup(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "service_id")
	c.forwardRequest(w, r, fmt.Sprintf("/v1/clusters/%s/backups", serviceID), "POST")
}

// GetClusterBackups get KubeBlocks cluster backups
func (c *KubeBlocksController) GetClusterBackups(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "service_id")
	c.forwardRequest(w, r, fmt.Sprintf("/v1/clusters/%s/backups", serviceID), "GET")
}

// DeleteClusters delete KubeBlocks clusters
func (c *KubeBlocksController) DeleteClusters(w http.ResponseWriter, r *http.Request) {
	c.forwardRequest(w, r, "/v1/clusters", "DELETE")
}

// DeleteClusterBackup delete KubeBlocks cluster backup
func (c *KubeBlocksController) DeleteClusterBackup(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "service_id")
	c.forwardRequest(w, r, fmt.Sprintf("/v1/clusters/%s/backups", serviceID), "DELETE")
}

// ManageCluster forwards to block-mechanica to manage cluster lifecycle
func (c *KubeBlocksController) ManageCluster(w http.ResponseWriter, r *http.Request) {
	logrus.Infof("ManageCluster request: %v", r.Body)
	c.forwardRequest(w, r, "/v1/clusters/actions", "POST")
}

// GetClusterPodDetail forwards to block-mechanica to get pod details managed by Cluster
func (c *KubeBlocksController) GetClusterPodDetail(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "service_id")
	podName := chi.URLParam(r, "pod_name")
	c.forwardRequest(w, r, fmt.Sprintf("/v1/clusters/%s/pods/%s/details", serviceID, podName), "GET")
}

// GetClusterEvents forwards to block-mechanica to get cluster's events
func (c *KubeBlocksController) GetClusterEvents(w http.ResponseWriter, r *http.Request) {
	serviceID := chi.URLParam(r, "service_id")
	c.forwardRequest(w, r, fmt.Sprintf("/v1/clusters/%s/events", serviceID), "GET")
}

// forwardRequest helper function to forward requests to Block Mechanica
func (c *KubeBlocksController) forwardRequest(w http.ResponseWriter, r *http.Request, api, method string) {
	// 构建URL
	targetURL := blockMechanicaBaseURL + api
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}
	logrus.Debugf("request block-mechanica service: %s %s", method, targetURL)

	var req *http.Request
	var err error

	// 读取请求体
	var body io.Reader
	requestBody, readErr := io.ReadAll(r.Body)
	if readErr != nil {
		logrus.Errorf("read request body failed: %v", readErr)
		httputil.ReturnError(r, w, 400, "read request body failed: "+readErr.Error())
		return
	}
	if len(requestBody) > 0 {
		body = strings.NewReader(string(requestBody))
	}
	req, err = http.NewRequest(method, targetURL, body)

	if err != nil {
		logrus.Errorf("create request to block-mechanica failed: %v", err)
		httputil.ReturnError(r, w, 500, "create request to block-mechanica failed: "+err.Error())
		return
	}

	// Set headers
	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Set(key, value)
		}
	}

	if req.Header.Get("Content-Type") == "" && (method == "POST" || method == "PUT" || method == "DELETE") {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logrus.Errorf("request block-mechanica service failed: %v", err)
		httputil.ReturnError(r, w, 500, "request block-mechanica service failed: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logrus.Errorf("block-mechanica service returned error status code %d: %s", resp.StatusCode, string(body))
		httputil.ReturnError(r, w, resp.StatusCode, "block-mechanica service returned error: "+string(body))
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logrus.Errorf("parse block-mechanica response failed: %v", err)
		httputil.ReturnError(r, w, 500, "parse block-mechanica response failed: "+err.Error())
		return
	}

	if field, exists := result["list"]; exists {
		httputil.ReturnSuccess(r, w, field)
	} else if field, exists := result["bean"]; exists {
		httputil.ReturnSuccess(r, w, field)
	} else {
		httputil.ReturnSuccess(r, w, result)
	}
}
