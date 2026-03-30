// RAINBOND, Application Management Platform
// Copyright (C) 2014-2020 Goodrain Co., Ltd.

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

package app

import (
	"net/http"
	"testing"

	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"
)

// capability_id: rainbond.webcli.config-defaults
func TestSetConfigDefaults(t *testing.T) {
	config := &restclient.Config{}

	if err := SetConfigDefaults(config); err != nil {
		t.Fatal(err)
	}
	if config.APIPath != "/api" {
		t.Fatalf("unexpected APIPath: %q", config.APIPath)
	}
	if config.GroupVersion == nil || config.GroupVersion.Version != "v1" {
		t.Fatalf("unexpected GroupVersion: %+v", config.GroupVersion)
	}
	if config.NegotiatedSerializer == nil {
		t.Fatal("expected negotiated serializer")
	}
	if config.UserAgent == "" {
		t.Fatal("expected default user agent")
	}
}

// capability_id: rainbond.webcli.auth-signature
func TestMD5Func(t *testing.T) {
	got := md5Func("tenant_service_pod")
	if got != "97b13ff70a9e35034ba3c555bb46ad59" {
		t.Fatalf("unexpected md5: %q", got)
	}
}

// capability_id: rainbond.webcli.config-defaults
func TestHandleWSRejectsNonGET(t *testing.T) {
	app := &App{}
	req, err := http.NewRequest(http.MethodPost, "/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := &responseRecorder{}
	app.HandleWS(rr, req)
	if rr.status != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rr.status)
	}
}

// capability_id: rainbond.webcli.container-args
func TestGetContainerArgsSelectsContainerAndExecArgs(t *testing.T) {
	app := &App{
		coreClient: fake.NewSimpleClientset(&api.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-pod",
				Namespace: "demo-ns",
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					{
						Name: "main",
						Env: []api.EnvVar{
							{Name: "ES_DEFAULT_EXEC_ARGS", Value: "/bin/bash -lc"},
						},
					},
				},
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				PodIP: "10.0.0.3",
				ContainerStatuses: []api.ContainerStatus{
					{
						Name:  "main",
						Ready: true,
						State: api.ContainerState{Running: &api.ContainerStateRunning{}},
					},
				},
			},
		}),
	}

	container, ip, args, err := app.GetContainerArgs("demo-ns", "demo-pod", "")
	if err != nil {
		t.Fatal(err)
	}
	if container != "main" || ip != "10.0.0.3" {
		t.Fatalf("unexpected container/ip: %q %q", container, ip)
	}
	if len(args) != 2 || args[0] != "/bin/bash" || args[1] != "-lc" {
		t.Fatalf("unexpected exec args: %#v", args)
	}
}

// capability_id: rainbond.webcli.container-args
func TestGetContainerArgsRejectsNonRunningPod(t *testing.T) {
	app := &App{
		coreClient: fake.NewSimpleClientset(&api.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-pod",
				Namespace: "demo-ns",
			},
			Status: api.PodStatus{Phase: api.PodPending},
		}),
	}

	_, _, _, err := app.GetContainerArgs("demo-ns", "demo-pod", "")
	if err == nil {
		t.Fatal("expected non-running pod error")
	}
}

// capability_id: rainbond.webcli.container-args
func TestGetContainerArgsRejectsNotReadyContainer(t *testing.T) {
	app := &App{
		coreClient: fake.NewSimpleClientset(&api.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-pod",
				Namespace: "demo-ns",
			},
			Spec: api.PodSpec{
				Containers: []api.Container{{Name: "main"}},
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				ContainerStatuses: []api.ContainerStatus{
					{
						Name:  "main",
						Ready: false,
						State: api.ContainerState{Waiting: &api.ContainerStateWaiting{Reason: "ContainerCreating"}},
					},
				},
			},
		}),
	}

	_, _, _, err := app.GetContainerArgs("demo-ns", "demo-pod", "main")
	if err == nil {
		t.Fatal("expected not-ready container error")
	}
}

// capability_id: rainbond.webcli.completed-pod-guard
func TestGetContainerArgsRejectsCompletedPod(t *testing.T) {
	app := &App{
		coreClient: fake.NewSimpleClientset(&api.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-pod",
				Namespace: "demo-ns",
			},
			Status: api.PodStatus{Phase: api.PodSucceeded},
		}),
	}

	_, _, _, err := app.GetContainerArgs("demo-ns", "demo-pod", "")
	if err == nil {
		t.Fatal("expected completed pod error")
	}
}

// capability_id: rainbond.webcli.missing-container-guard
func TestGetContainerArgsRejectsMissingContainer(t *testing.T) {
	app := &App{
		coreClient: fake.NewSimpleClientset(&api.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "demo-pod",
				Namespace: "demo-ns",
			},
			Spec: api.PodSpec{
				Containers: []api.Container{{Name: "main"}},
			},
			Status: api.PodStatus{
				Phase: api.PodRunning,
				ContainerStatuses: []api.ContainerStatus{{
					Name:  "main",
					Ready: true,
					State: api.ContainerState{Running: &api.ContainerStateRunning{}},
				}},
			},
		}),
	}

	_, _, _, err := app.GetContainerArgs("demo-ns", "demo-pod", "sidecar")
	if err == nil {
		t.Fatal("expected missing container error")
	}
}

type responseRecorder struct {
	header http.Header
	body   []byte
	status int
}

func (r *responseRecorder) Header() http.Header {
	if r.header == nil {
		r.header = make(http.Header)
	}
	return r.header
}

func (r *responseRecorder) Write(p []byte) (int, error) {
	r.body = append(r.body, p...)
	return len(p), nil
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
}
