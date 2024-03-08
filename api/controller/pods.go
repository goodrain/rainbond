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
	"bufio"
	"fmt"
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/goodrain/rainbond/worker/server"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"net/http"
	"strconv"
	"strings"
)

// PodController is an implementation of PodInterface
type PodController struct{}

// Pods get some service pods
// swagger:operation GET /v2/tenants/{tenant_name}/pods v2/tenants pods
//
// 获取一些应用的Pod信息
//
// get some service pods
//
// ---
// consumes:
// - application/json
// - application/x-protobuf
//
// produces:
// - application/json
// - application/xml
//
// responses:
//
//	default:
//	  schema:
//	    "$ref": "#/responses/commandResponse"
//	  description: get some service pods
func Pods(w http.ResponseWriter, r *http.Request) {
	serviceIDs := strings.Split(r.FormValue("service_ids"), ",")
	if serviceIDs == nil || len(serviceIDs) == 0 {
		tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*model.Tenants)
		services, _ := db.GetManager().TenantServiceDao().GetServicesByTenantID(tenant.UUID)
		for _, s := range services {
			serviceIDs = append(serviceIDs, s.ServiceID)
		}
	}
	var allpods []*handler.K8sPodInfo
	podinfo, err := handler.GetServiceManager().GetMultiServicePods(serviceIDs)
	if err != nil {
		logrus.Errorf("get service pod failure %s", err.Error())
	}
	if podinfo != nil {
		var pods []*handler.K8sPodInfo
		if podinfo.OldPods != nil {
			pods = append(podinfo.NewPods, podinfo.OldPods...)
		} else {
			pods = podinfo.NewPods
		}
		for _, pod := range pods {
			allpods = append(allpods, pod)
		}
	}
	httputil.ReturnSuccess(r, w, allpods)
}

// PodNums reutrns the number of pods for components.
func PodNums(w http.ResponseWriter, r *http.Request) {
	componentIDs := strings.Split(r.FormValue("service_ids"), ",")
	podNums, err := handler.GetServiceManager().GetComponentPodNums(r.Context(), componentIDs)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, podNums)
}

// PodDetail -
func (p *PodController) PodDetail(w http.ResponseWriter, r *http.Request) {
	podName := chi.URLParam(r, "pod_name")

	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*model.Tenants)
	pd, err := handler.GetPodHandler().PodDetail(tenant.Namespace, podName)
	if err != nil {
		logrus.Errorf("error getting pod detail: %v", err)
		if err == server.ErrPodNotFound {
			httputil.ReturnError(r, w, 404, fmt.Sprintf("error getting pod detail: %v", err))
			return
		}
		httputil.ReturnError(r, w, 500, fmt.Sprintf("error getting pod detail: %v", err))
		return
	}
	httputil.ReturnSuccess(r, w, pd)
}

// PodLogs -
func (p *PodController) PodLogs(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*model.Tenants)
	podName := chi.URLParam(r, "pod_name")
	lines, err := strconv.Atoi(r.URL.Query().Get("lines"))
	if err != nil {
		lines = 100
	}
	tailLines := int64(lines)

	req := k8s.Default().Clientset.CoreV1().Pods(tenant.Namespace).GetLogs(podName, &corev1.PodLogOptions{
		Follow:     true,
		Timestamps: true,
		TailLines:  &tailLines,
	})
	logrus.Infof("Opening log stream for pod %s", podName)

	stream, err := req.Stream(r.Context())
	if err != nil {
		logrus.Errorf("Error opening log stream: %v", err)
		http.Error(w, "Error opening log stream", http.StatusInternalServerError)
		return
	}
	defer stream.Close()
	// Use Flusher to send headers to the client
	flusher, ok := w.(http.Flusher)
	if !ok {
		logrus.Errorf("Streaming not supported")
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	scanner := bufio.NewScanner(stream)

	for scanner.Scan() {
		select {
		case <-r.Context().Done():
			logrus.Warningf("Request context done: %v", r.Context().Err())
			return
		default:
			msg := "data: " + scanner.Text() + "\n\n"
			_, err := fmt.Fprintf(w, msg)
			flusher.Flush()
			if err != nil {
				logrus.Errorf("Error writing to response: %v", err)
			}
		}
	}
}
