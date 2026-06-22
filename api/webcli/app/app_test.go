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
	ktesting "k8s.io/client-go/testing"
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

func TestDebugToolboxImageUsesDefaultAndEnvOverride(t *testing.T) {
	t.Setenv(EnvDebugToolboxImage, "")
	if got := DebugToolboxImage(); got != DefaultDebugToolboxImage {
		t.Fatalf("expected default toolbox image %q, got %q", DefaultDebugToolboxImage, got)
	}

	t.Setenv(EnvDebugToolboxImage, "example.com/rainbond/toolbox:test")
	if got := DebugToolboxImage(); got != "example.com/rainbond/toolbox:test" {
		t.Fatalf("expected env toolbox image override, got %q", got)
	}
}

func TestEnsureDebugContainerCreatesEphemeralContainerWithTargetContext(t *testing.T) {
	client := fake.NewSimpleClientset(debugTestPod(nil, nil))
	app := &App{coreClient: client}

	_, _, _, err := app.GetDebugContainerArgs("demo-ns", "demo-pod", "main")
	if err == nil {
		t.Fatal("expected newly-created debug container to report not running yet")
	}

	actions := client.Actions()
	if len(actions) < 2 {
		t.Fatalf("expected get and update actions, got %#v", actions)
	}
	update, ok := actions[1].(ktesting.UpdateAction)
	if !ok {
		t.Fatalf("expected update action, got %T", actions[1])
	}
	if update.GetSubresource() != "ephemeralcontainers" {
		t.Fatalf("expected ephemeralcontainers subresource update, got %q", update.GetSubresource())
	}
	updatedPod, ok := update.GetObject().(*api.Pod)
	if !ok {
		t.Fatalf("expected pod update object, got %T", update.GetObject())
	}
	if len(updatedPod.Spec.EphemeralContainers) != 1 {
		t.Fatalf("expected one debug ephemeral container, got %d", len(updatedPod.Spec.EphemeralContainers))
	}
	debug := updatedPod.Spec.EphemeralContainers[0]
	if debug.Image != DefaultDebugToolboxImage {
		t.Fatalf("unexpected toolbox image: %q", debug.Image)
	}
	if debug.TargetContainerName != "main" {
		t.Fatalf("unexpected target container: %q", debug.TargetContainerName)
	}
	if len(debug.Command) == 0 {
		t.Fatal("expected long-running debug command")
	}
	if len(debug.VolumeMounts) != 1 || debug.VolumeMounts[0].Name != "data" || debug.VolumeMounts[0].MountPath != "/data" {
		t.Fatalf("expected non-subPath target volume mount to be copied, got %#v", debug.VolumeMounts)
	}
}

func TestGetDebugContainerArgsReusesRunningEphemeralContainer(t *testing.T) {
	debugContainer := api.EphemeralContainer{
		EphemeralContainerCommon: api.EphemeralContainerCommon{
			Name:  "rb-debug-main",
			Image: DefaultDebugToolboxImage,
		},
		TargetContainerName: "main",
	}
	debugStatus := api.ContainerStatus{
		Name:  "rb-debug-main",
		Ready: true,
		State: api.ContainerState{Running: &api.ContainerStateRunning{}},
	}
	client := fake.NewSimpleClientset(debugTestPod([]api.EphemeralContainer{debugContainer}, []api.ContainerStatus{debugStatus}))
	app := &App{coreClient: client}

	container, ip, args, err := app.GetDebugContainerArgs("demo-ns", "demo-pod", "main")
	if err != nil {
		t.Fatal(err)
	}
	if container != "rb-debug-main" || ip != "10.0.0.3" {
		t.Fatalf("unexpected debug container/ip: %q %q", container, ip)
	}
	if len(args) != 1 || args[0] != "/bin/sh" {
		t.Fatalf("unexpected debug exec args: %#v", args)
	}
	for _, action := range client.Actions() {
		if action.Matches("update", "pods") {
			t.Fatalf("did not expect update when reusing running debug container: %#v", client.Actions())
		}
	}
}

func TestEnsureDebugContainerUsesNewNameWhenExistingDebugContainerTerminated(t *testing.T) {
	debugContainer := api.EphemeralContainer{
		EphemeralContainerCommon: api.EphemeralContainerCommon{
			Name:  "rb-debug-main",
			Image: DefaultDebugToolboxImage,
		},
		TargetContainerName: "main",
	}
	debugStatus := api.ContainerStatus{
		Name:  "rb-debug-main",
		Ready: false,
		State: api.ContainerState{Terminated: &api.ContainerStateTerminated{}},
	}
	client := fake.NewSimpleClientset(debugTestPod([]api.EphemeralContainer{debugContainer}, []api.ContainerStatus{debugStatus}))
	app := &App{coreClient: client}

	_, _, _, err := app.GetDebugContainerArgs("demo-ns", "demo-pod", "main")
	if err == nil {
		t.Fatal("expected newly-created replacement debug container to report not running yet")
	}

	var updatedPod *api.Pod
	for _, action := range client.Actions() {
		update, ok := action.(ktesting.UpdateAction)
		if ok && update.GetSubresource() == "ephemeralcontainers" {
			updatedPod = update.GetObject().(*api.Pod)
			break
		}
	}
	if updatedPod == nil {
		t.Fatal("expected ephemeralcontainers update")
	}
	if len(updatedPod.Spec.EphemeralContainers) != 2 {
		t.Fatalf("expected replacement debug container, got %#v", updatedPod.Spec.EphemeralContainers)
	}
	if updatedPod.Spec.EphemeralContainers[1].Name != "rb-debug-main-1" {
		t.Fatalf("unexpected replacement debug container name: %q", updatedPod.Spec.EphemeralContainers[1].Name)
	}
}

func TestGetDebugContainerArgsPrefersRunningDebugContainerOverTerminatedOne(t *testing.T) {
	terminatedDebugContainer := api.EphemeralContainer{
		EphemeralContainerCommon: api.EphemeralContainerCommon{
			Name:  "rb-debug-main",
			Image: DefaultDebugToolboxImage,
		},
		TargetContainerName: "main",
	}
	runningDebugContainer := api.EphemeralContainer{
		EphemeralContainerCommon: api.EphemeralContainerCommon{
			Name:  "rb-debug-main-1",
			Image: DefaultDebugToolboxImage,
		},
		TargetContainerName: "main",
	}
	terminatedStatus := api.ContainerStatus{
		Name:  "rb-debug-main",
		Ready: false,
		State: api.ContainerState{Terminated: &api.ContainerStateTerminated{}},
	}
	runningStatus := api.ContainerStatus{
		Name:  "rb-debug-main-1",
		Ready: true,
		State: api.ContainerState{Running: &api.ContainerStateRunning{}},
	}
	client := fake.NewSimpleClientset(debugTestPod(
		[]api.EphemeralContainer{terminatedDebugContainer, runningDebugContainer},
		[]api.ContainerStatus{terminatedStatus, runningStatus},
	))
	app := &App{coreClient: client}

	container, _, _, err := app.GetDebugContainerArgs("demo-ns", "demo-pod", "main")
	if err != nil {
		t.Fatal(err)
	}
	if container != "rb-debug-main-1" {
		t.Fatalf("expected running debug container to be reused, got %q", container)
	}
	for _, action := range client.Actions() {
		if action.Matches("update", "pods") {
			t.Fatalf("did not expect update when a running debug container exists: %#v", client.Actions())
		}
	}
}

func debugTestPod(ephemeralContainers []api.EphemeralContainer, ephemeralStatuses []api.ContainerStatus) *api.Pod {
	return &api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-pod",
			Namespace: "demo-ns",
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Name: "main",
					VolumeMounts: []api.VolumeMount{
						{Name: "data", MountPath: "/data"},
						{Name: "config", MountPath: "/etc/config", SubPath: "app.yaml"},
					},
				},
			},
			EphemeralContainers: ephemeralContainers,
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
			EphemeralContainerStatuses: ephemeralStatuses,
		},
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
