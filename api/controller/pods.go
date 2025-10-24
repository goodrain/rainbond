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
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond-operator/util/constants"
	"github.com/goodrain/rainbond/api/handler"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/util"
	utils "github.com/goodrain/rainbond/util"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/goodrain/rainbond/worker/server"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// SystemPodDetail -
func (p *PodController) SystemPodDetail(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("ns")
	name := r.URL.Query().Get("name")
	list, err := k8s.Default().Clientset.CoreV1().Pods(ns).List(r.Context(), metav1.ListOptions{
		LabelSelector: "name=" + name,
	})
	if err != nil {
		logrus.Errorf("error getting pod detail: %v", err)
		return
	}
	httputil.ReturnSuccess(r, w, list)
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

func logs(w http.ResponseWriter, r *http.Request, podName string, namespace string) {
	lines, err := strconv.Atoi(r.URL.Query().Get("lines"))
	if err != nil {
		lines = 100
	}
	tailLines := int64(lines)

	// Get container name from query parameter
	container := r.URL.Query().Get("container")

	// Get pod to check how many containers it has
	pod, err := k8s.Default().Clientset.CoreV1().Pods(namespace).Get(r.Context(), podName, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error getting pod %s: %v", podName, err)
		http.Error(w, fmt.Sprintf("Error getting pod: %v", err), http.StatusInternalServerError)
		return
	}

	// Determine which containers to stream logs from
	var containers []string
	if container != "" {
		// Use specified container
		containers = append(containers, container)
		logrus.Infof("Streaming logs from specified container: %s", container)
	} else {
		// Special handling for rbd-gateway pods
		if strings.HasPrefix(podName, "rbd-gateway") {
			containers = append(containers, "apisix")
			logrus.Infof("rbd-gateway pod detected, using container: ingress-apisix")
		} else {
			// Default behavior: stream all containers
			for _, c := range pod.Spec.Containers {
				containers = append(containers, c.Name)
			}
			logrus.Infof("No container specified, streaming logs from all containers: %v", containers)
		}
	}

	if len(containers) == 0 {
		http.Error(w, "No containers found in pod", http.StatusNotFound)
		return
	}

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

	// If single container, use original logic
	if len(containers) == 1 {
		streamContainerLogs(w, r, podName, namespace, containers[0], tailLines, flusher)
		return
	}

	// For multiple containers, merge streams
	logrus.Infof("Opening log streams for pod %s with %d containers", podName, len(containers))

	// Create a channel to merge logs from all containers
	logChan := make(chan string, 100)
	doneChan := make(chan struct{})

	// Start goroutine for each container
	for _, containerName := range containers {
		go func(cName string) {
			req := k8s.Default().Clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
				Follow:     true,
				Timestamps: true,
				TailLines:  &tailLines,
				Container:  cName,
			})

			stream, err := req.Stream(r.Context())
			if err != nil {
				logrus.Errorf("Error opening log stream for container %s: %v", cName, err)
				return
			}
			defer stream.Close()

			scanner := bufio.NewScanner(stream)
			for scanner.Scan() {
				select {
				case <-r.Context().Done():
					return
				case <-doneChan:
					return
				default:
					// Prefix log line with container name
					logLine := fmt.Sprintf("[%s] %s", cName, scanner.Text())
					logChan <- logLine
				}
			}
		}(containerName)
	}

	// Stream merged logs to client
	for {
		select {
		case <-r.Context().Done():
			close(doneChan)
			logrus.Warningf("Request context done: %v", r.Context().Err())
			return
		case logLine := <-logChan:
			msg := "data: " + logLine + "\n\n"
			_, err := fmt.Fprintf(w, msg)
			flusher.Flush()
			if err != nil {
				logrus.Errorf("Error writing to response: %v", err)
				close(doneChan)
				return
			}
		}
	}
}

// streamContainerLogs streams logs from a single container
func streamContainerLogs(w http.ResponseWriter, r *http.Request, podName, namespace, container string, tailLines int64, flusher http.Flusher) {
	req := k8s.Default().Clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Follow:     true,
		Timestamps: true,
		TailLines:  &tailLines,
		Container:  container,
	})
	logrus.Infof("Opening log stream for pod %s, container %s", podName, container)

	stream, err := req.Stream(r.Context())
	if err != nil {
		logrus.Errorf("Error opening log stream: %v", err)
		http.Error(w, "Error opening log stream", http.StatusInternalServerError)
		return
	}
	defer stream.Close()

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

// SystemPodLogs -
func (p *PodController) SystemPodLogs(w http.ResponseWriter, r *http.Request) {
	ns, err := util.GetMyNamespace()
	if err != nil {
		httputil.ReturnError(r, w, 500, fmt.Sprintf("error getting namespace: %v", err))
		return
	}
	if ns == "" {
		ns = utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace)
	}
	name := r.URL.Query().Get("name")
	logs(w, r, name, ns)
}

// PodLogs -
func (p *PodController) PodLogs(w http.ResponseWriter, r *http.Request) {
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*model.Tenants)
	logs(w, r, chi.URLParam(r, "pod_name"), tenant.Namespace)
}
