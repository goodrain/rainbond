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

package sources

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/registry"
	"github.com/goodrain/rainbond/event"
	corev1 "k8s.io/api/core/v1"
)

// capability_id: rainbond.source-image.parse-name
func TestImageName(t *testing.T) {
	tests := []struct {
		input string
		host  string
		name  string
		tag   string
	}{
		{input: "hub.goodrain.com/nginx:v1", host: "hub.goodrain.com", name: "nginx", tag: "v1"},
		{input: "hub.goodrain.cn/nginx", host: "hub.goodrain.cn", name: "nginx", tag: "latest"},
		{input: "nginx:v2", host: "", name: "nginx", tag: "v2"},
		{input: "tomcat", host: "", name: "tomcat", tag: "latest"},
	}
	for _, tt := range tests {
		got := ImageNameHandle(tt.input)
		if got.Host != tt.host || got.Name != tt.name || got.Tag != tt.tag {
			t.Fatalf("ImageNameHandle(%q)=%+v, want host=%q name=%q tag=%q", tt.input, got, tt.host, tt.name, tt.tag)
		}
	}
}

// capability_id: rainbond.source-image.parse-name-with-namespace
func TestImageNameWithNamespace(t *testing.T) {
	got := ImageNameWithNamespaceHandle("registry.example.com/team/demo:v1")
	if got.Host != "registry.example.com" || got.Namespace != "team" || got.Name != "demo" || got.Tag != "v1" {
		t.Fatalf("unexpected parsed image: %+v", got)
	}

	got = ImageNameWithNamespaceHandle("demo")
	if got.Host != "" || got.Namespace != "" || got.Name != "demo" || got.Tag != "latest" {
		t.Fatalf("unexpected parsed image without namespace: %+v", got)
	}
}

// capability_id: rainbond.source-image.auth-base64-encode
func TestEncodeAuthToBase64(t *testing.T) {
	encoded, err := EncodeAuthToBase64(registry.AuthConfig{
		Username: "demo",
		Password: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var auth registry.AuthConfig
	if err := json.Unmarshal(raw, &auth); err != nil {
		t.Fatal(err)
	}
	if auth.Username != "demo" || auth.Password != "secret" {
		t.Fatalf("unexpected decoded auth config: %+v", auth)
	}
}

// capability_id: rainbond.source-image.trusted-registry-check
func TestCheckTrustedRepositories(t *testing.T) {
	t.Skip("requires remote registry access")
	err := CheckTrustedRepositories("hub.goodrain.com/zengqg-test/etcd2:v2.2.0", "zengqg-test", "zengqg-test")
	if err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.source-image.save
func TestImageSave(t *testing.T) {
	t.Skip("requires local docker daemon")
	/*
		dc, _ := client.NewEnvClient()
		if err := ImageSave(dc, "hub.goodrain.com/zengqg-test/etcd:v2.2.0", "/tmp/testsaveimage.tar", nil); err != nil {
			t.Fatal(err)
		}
	*/
}

// capability_id: rainbond.source-image.multi-save
func TestMulitImageSave(t *testing.T) {
	t.Skip("requires local docker daemon")
	/*
		dc, _ := client.NewEnvClient()
		if err := MultiImageSave(context.Background(), dc, "/tmp/testsaveimage.tar", nil,
			"registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-node:V5.3.0-cloud",
			"registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-resource-proxy:V5.3.0-cloud"); err != nil {
			t.Fatal(err)
		}
	*/
}

// capability_id: rainbond.source-image.import
func TestImageImport(t *testing.T) {
	t.Skip("requires local docker daemon")
	/*
		dc, _ := client.NewEnvClient()
		if err := ImageImport(dc, "hub.goodrain.com/zengqg-test/etcd:v2.2.0", "/tmp/testsaveimage.tar", nil); err != nil {
			t.Fatal(err)
		}
	*/
}

// capability_id: rainbond.source-image.vm-build-host-taint-toleration
func TestNewBuildKitPodSpecAddsTolerationForHostScheduling(t *testing.T) {
	hostAliases := []corev1.HostAlias{{IP: "10.0.0.2", Hostnames: []string{"registry.local"}}}

	podSpec := newBuildKitPodSpec("amd64", "node-1", hostAliases)

	if podSpec.NodeSelector["kubernetes.io/hostname"] != "node-1" {
		t.Fatalf("expected node selector for host scheduling, got %#v", podSpec.NodeSelector)
	}
	if len(podSpec.Tolerations) != 1 || podSpec.Tolerations[0].Operator != corev1.TolerationOpExists {
		t.Fatalf("expected broad host taint toleration, got %#v", podSpec.Tolerations)
	}
	if podSpec.Affinity == nil || podSpec.Affinity.NodeAffinity == nil {
		t.Fatal("expected node affinity to be preserved")
	}
	if len(podSpec.HostAliases) != 1 || podSpec.HostAliases[0].IP != "10.0.0.2" {
		t.Fatalf("expected host aliases to be preserved, got %#v", podSpec.HostAliases)
	}
}

// capability_id: rainbond.vm-publish.stage-timing-logs
func TestRecordImageBuildStageLogsFields(t *testing.T) {
	logger := &stageRecordingLogger{}
	start := time.Unix(100, 0)

	err := recordImageBuildStage(logger, "buildkit_job_wait", start, nil, map[string]interface{}{
		"service_id": "svc-a",
		"job_name":   "svc-a-1-dockerfile",
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(logger.infos) != 1 {
		t.Fatalf("expected one info log, got %d", len(logger.infos))
	}
	got := logger.infos[0]
	for _, want := range []string{
		"stage=buildkit_job_wait",
		"status=success",
		"duration_ms=",
		"service_id=svc-a",
		"job_name=svc-a-1-dockerfile",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected log to contain %q, got %q", want, got)
		}
	}
}

func TestRecordImageBuildStageLogsError(t *testing.T) {
	logger := &stageRecordingLogger{}
	start := time.Unix(100, 0)
	stageErr := errors.New("job timeout")

	err := recordImageBuildStage(logger, "buildkit_job_wait", start, stageErr, map[string]interface{}{
		"service_id": "svc-a",
	})

	if !errors.Is(err, stageErr) {
		t.Fatalf("expected original error, got %v", err)
	}
	if len(logger.errors) != 1 {
		t.Fatalf("expected one error log, got %d", len(logger.errors))
	}
	got := logger.errors[0]
	for _, want := range []string{
		"stage=buildkit_job_wait",
		"status=failure",
		"duration_ms=",
		"service_id=svc-a",
		"error=job timeout",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected log to contain %q, got %q", want, got)
		}
	}
}

// capability_id: rainbond.vm-publish.http-artifact-image-build
func TestBuildKitImageOutputUsesUncompressedLayersForVMBuild(t *testing.T) {
	vmOutput := buildKitImageOutput("vm-build", "goodrain.me/demo:latest")
	if vmOutput != "type=image,name=goodrain.me/demo:latest,push=true,compression=uncompressed" {
		t.Fatalf("unexpected vm build output: %q", vmOutput)
	}

	defaultOutput := buildKitImageOutput("run-build", "goodrain.me/demo:latest")
	if defaultOutput != "type=image,name=goodrain.me/demo:latest,push=true" {
		t.Fatalf("unexpected default build output: %q", defaultOutput)
	}
}

type stageRecordingLogger struct {
	infos  []string
	errors []string
}

func (l *stageRecordingLogger) Info(message string, info map[string]string) {
	l.infos = append(l.infos, message)
}

func (l *stageRecordingLogger) Error(message string, info map[string]string) {
	l.errors = append(l.errors, message)
}

func (l *stageRecordingLogger) Debug(message string, info map[string]string) {}

func (l *stageRecordingLogger) Event() string { return "test" }

func (l *stageRecordingLogger) CreateTime() time.Time { return time.Unix(0, 0) }

func (l *stageRecordingLogger) GetChan() chan []byte { return nil }

func (l *stageRecordingLogger) SetChan(ch chan []byte) {}

func (l *stageRecordingLogger) GetWriter(step, level string) event.LoggerWriter {
	return stageDiscardLoggerWriter{}
}

type stageDiscardLoggerWriter struct{}

func (stageDiscardLoggerWriter) SetFormat(format map[string]interface{}) {}

func (stageDiscardLoggerWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
