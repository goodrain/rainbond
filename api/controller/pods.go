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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond-operator/util/constants"
	"github.com/goodrain/rainbond/api/handler"
	apimodel "github.com/goodrain/rainbond/api/model"
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
	follow, err := parseFollow(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// previous=true returns logs of the last terminated instance of the
	// container, used for crash diagnosis. Following previous logs is not
	// meaningful, so follow is forced off in that case.
	previous := parsePrevious(r)
	if previous {
		follow = false
	}

	// Get container name from query parameter
	container := r.URL.Query().Get("container")
	logrus.Infof(
		"resource center pod logs request path=%s namespace=%s pod=%s container=%s lines=%d follow=%t previous=%t",
		r.URL.String(), namespace, podName, container, lines, follow, previous,
	)

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
		streamContainerLogs(w, r, podName, namespace, containers[0], tailLines, follow, previous, flusher)
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
				Follow:     follow,
				Timestamps: true,
				TailLines:  &tailLines,
				Container:  cName,
				Previous:   previous,
			})

			stream, err := req.Stream(r.Context())
			if err != nil {
				logrus.Errorf("Error opening log stream for container %s: %v", cName, err)
				return
			}
			defer stream.Close()

			scanner := bufio.NewScanner(stream)
			// Increase buffer size to handle large log lines
			buf := make([]byte, 0, 64*1024)
			scanner.Buffer(buf, 1024*1024)

			for scanner.Scan() {
				select {
				case <-r.Context().Done():
					return
				case <-doneChan:
					return
				default:
					// Prefix log line with container name and sanitize non-UTF8 bytes
					logLine := fmt.Sprintf("[%s] %s", cName, sanitizeLogLine(scanner.Text()))
					logChan <- logLine
				}
			}
			if err := scanner.Err(); err != nil {
				if isExpectedLogStreamClose(r.Context(), err) {
					logrus.Infof("Log stream closed for container %s in pod %s/%s: %v", cName, namespace, podName, err)
					return
				}
				// Log transport errors at warning level since they're often transient
				if isTransportError(err) {
					logrus.Warningf("Transport error in log stream for container %s in pod %s/%s: %v", cName, namespace, podName, err)
					return
				}
				logrus.Errorf("Error scanning log stream for container %s in pod %s/%s: %v", cName, namespace, podName, err)
			} else {
				logrus.Infof("Log stream ended for container %s in pod %s/%s", cName, namespace, podName)
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
		case logLine, ok := <-logChan:
			if !ok {
				// Channel closed, all container streams ended
				logrus.Infof("All container log streams ended for pod %s/%s", namespace, podName)
				return
			}
			msg := "data: " + logLine + "\n\n"
			_, err := fmt.Fprint(w, msg)
			flusher.Flush()
			if err != nil {
				if isTransportError(err) {
					logrus.Warningf("Client disconnected during log stream for pod %s/%s: %v", namespace, podName, err)
					close(doneChan)
					return
				}
				logrus.Errorf("Error writing to response: %v", err)
				close(doneChan)
				return
			}
		}
	}
}

// streamContainerLogs streams logs from a single container
func streamContainerLogs(
	w http.ResponseWriter, r *http.Request, podName, namespace, container string, tailLines int64, follow, previous bool,
	flusher http.Flusher,
) {
	req := k8s.Default().Clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Follow:     follow,
		Timestamps: true,
		TailLines:  &tailLines,
		Container:  container,
		Previous:   previous,
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
	// Increase buffer size to handle large log lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		select {
		case <-r.Context().Done():
			logrus.Warningf("Request context done: %v", r.Context().Err())
			return
		default:
			// Sanitize non-UTF8 bytes to prevent encoding errors
			logLine := sanitizeLogLine(scanner.Text())
			msg := "data: " + logLine + "\n\n"
			_, err := fmt.Fprint(w, msg)
			flusher.Flush()
			if err != nil {
				if isTransportError(err) {
					logrus.Warningf("Client disconnected during log stream for pod %s/%s container %s: %v", namespace, podName, container, err)
					return
				}
				logrus.Errorf("Error writing to response: %v", err)
				return
			}
		}
	}
	if err := scanner.Err(); err != nil {
		if isExpectedLogStreamClose(r.Context(), err) {
			logrus.Infof("Single-container log stream closed for pod %s/%s container %s: %v", namespace, podName, container, err)
			return
		}
		// Log transport errors at warning level since they're often transient
		if isTransportError(err) {
			logrus.Warningf("Transport error in single-container log stream for pod %s/%s container %s: %v", namespace, podName, container, err)
			return
		}
		logrus.Errorf("Error scanning single-container log stream for pod %s/%s container %s: %v", namespace, podName, container, err)
	} else {
		logrus.Infof("Single-container log stream ended for pod %s/%s container %s", namespace, podName, container)
	}
}

func parseFollow(r *http.Request) (bool, error) {
	followValue := r.URL.Query().Get("follow")
	if followValue == "" {
		return true, nil
	}
	follow, err := strconv.ParseBool(followValue)
	if err != nil {
		return false, fmt.Errorf("invalid follow value: %s", followValue)
	}
	return follow, nil
}

// parsePrevious reports whether the caller requested logs of the previously
// terminated container instance via previous=true. Defaults to false; any
// unparseable value is treated as false.
func parsePrevious(r *http.Request) bool {
	previousValue := r.URL.Query().Get("previous")
	if previousValue == "" {
		return false
	}
	previous, err := strconv.ParseBool(previousValue)
	if err != nil {
		return false
	}
	return previous
}

func isExpectedLogStreamClose(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if ctx != nil && errors.Is(ctx.Err(), context.Canceled) {
		return true
	}
	return errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed)
}

// sanitizeLogLine replaces invalid UTF-8 sequences with the Unicode replacement
// character so that log lines can be safely written to SSE responses without
// causing encoding errors on the client side.
func sanitizeLogLine(line string) string {
	if utf8.ValidString(line) {
		return line
	}
	var buf bytes.Buffer
	buf.Grow(len(line))
	for i := 0; i < len(line); {
		r, size := utf8.DecodeRuneInString(line[i:])
		if r == utf8.RuneError && size == 1 {
			buf.WriteRune('�') // Unicode replacement character
			i++
		} else {
			buf.WriteRune(r)
			i += size
		}
	}
	return buf.String()
}

// isTransportError checks if the error is a transport-level error that indicates
// a connection issue (connection reset, broken pipe, etc.)
func isTransportError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	// Check for common transport error strings
	errStr := err.Error()
	return strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "use of closed network connection")
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

// PodExec runs a one-shot, non-interactive command in a pod container and
// returns the captured stdout/stderr, the container exit code, and whether the
// output was truncated. Authorization is enforced by the existing
// InitTenant/InitService middleware; arbitrary commands are allowed, mirroring
// the Web Terminal. Safety rails: context timeout, output cap, audit log.
func (p *PodController) PodExec(w http.ResponseWriter, r *http.Request) {
	podName := chi.URLParam(r, "pod_name")
	tenant := r.Context().Value(ctxutil.ContextKey("tenant")).(*model.Tenants)
	service := r.Context().Value(ctxutil.ContextKey("service")).(*model.TenantServices)

	var req apimodel.PodExecRequest
	if !httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil) {
		return
	}
	if len(req.Command) == 0 {
		httputil.ReturnError(r, w, 400, "command must not be empty")
		return
	}

	// Default to the component's main container when none is specified.
	containerName := req.Container
	if containerName == "" {
		containerName = service.K8sComponentName
	}

	stdout, stderr, exitCode, truncated, err := handler.GetServiceManager().ExecCommand(
		podName, tenant.Namespace, containerName, req.Command, req.TimeoutSeconds,
	)
	if err != nil {
		// Distinguish the not-running case so callers can fall back to
		// previous-container logs.
		if errors.Is(err, handler.ErrContainerNotRunning) {
			logrus.Warningf(
				"pod exec on non-running container: tenant=%s service=%s pod=%s container=%s err=%v",
				tenant.Name, service.ServiceAlias, podName, containerName, err,
			)
			httputil.ReturnError(r, w, 409, fmt.Sprintf("container not running: %v", err))
			return
		}
		logrus.Errorf(
			"pod exec failed: tenant=%s service=%s pod=%s container=%s command=%v err=%v",
			tenant.Name, service.ServiceAlias, podName, containerName, req.Command, err,
		)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("exec failed: %v", err))
		return
	}

	// Audit log: one structured line for traceability.
	logrus.Infof(
		"pod exec audit: tenant=%s service=%s pod=%s container=%s command=%v exit_code=%d truncated=%t",
		tenant.Name, service.ServiceAlias, podName, containerName, req.Command, exitCode, truncated,
	)

	httputil.ReturnSuccess(r, w, &apimodel.PodExecResult{
		Stdout:    stdout,
		Stderr:    stderr,
		ExitCode:  exitCode,
		Truncated: truncated,
	})
}
