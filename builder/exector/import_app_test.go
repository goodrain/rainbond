// RAINBOND, Application Management Platform
// Copyright (C) 2020-2020 Goodrain Co., Ltd.

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

package exector

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/containerd/containerd"
	dockercli "github.com/docker/docker/client"
	"github.com/goodrain/rainbond-oam/pkg/ram/v1alpha1"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/event"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// capability_id: rainbond.app-import.package-name-normalize
func TestBuildFromLinuxFileName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "", want: ""},
		{input: "nginx--latest", want: "nginx:latest"},
		{input: "/tmp/cache/demo--v1", want: "demo:v1"},
		{input: "  nginx--latest  ", want: "nginx:latest"},
	}

	for _, tt := range tests {
		if got := buildFromLinuxFileName(tt.input); got != tt.want {
			t.Fatalf("buildFromLinuxFileName(%q)=%q, want %q", tt.input, got, tt.want)
		}
	}
}

// capability_id: rainbond.app-import.status-serialization
func TestAppStatusMapRoundTrip(t *testing.T) {
	input := "app-a:importing,app-b:success"
	got := str2map(input)
	want := map[string]string{
		"app-a": "importing",
		"app-b": "success",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("str2map(%q)=%v, want %v", input, got, want)
	}

	serialized := map2str(want)
	roundTrip := str2map(serialized)
	if !reflect.DeepEqual(roundTrip, want) {
		t.Fatalf("round trip mismatch: got %v, want %v", roundTrip, want)
	}
}

// capability_id: rainbond.app-import.scaling-rule-compat
func TestNormalizeImportedRAMPreservesLegacyScalingRule(t *testing.T) {
	rawMetadata := []byte(`{
		"group_key": "demo-app",
		"group_name": "Demo App",
		"group_version": "1.0.0",
		"apps": [{
			"service_key": "svc-key",
			"cpu": 250,
			"extend_method_map": {
				"min_node": 1,
				"min_memory": 64
			},
			"service_extend_method": {
				"min_node": 2,
				"max_node": 7,
				"step_node": 2,
				"min_memory": 512,
				"max_memory": 4096,
				"step_memory": 128,
				"is_restart": false,
				"container_cpu": 600
			}
		}]
	}`)
	var ram v1alpha1.RainbondApplicationConfig
	if err := json.Unmarshal(rawMetadata, &ram); err != nil {
		t.Fatal(err)
	}

	normalizeImportedRAM(rawMetadata, &ram)

	rule := ram.Components[0].ExtendMethodRule
	if rule.MinNode != 1 {
		t.Fatalf("expected existing min_node to be kept, got %d", rule.MinNode)
	}
	if rule.MaxNode != 7 {
		t.Fatalf("expected legacy max_node to be restored, got %d", rule.MaxNode)
	}
	if rule.StepNode != 2 {
		t.Fatalf("expected legacy step_node to be restored, got %d", rule.StepNode)
	}
	if rule.InitMemory != 64 {
		t.Fatalf("expected init_memory to fall back to min_memory, got %d", rule.InitMemory)
	}
	if ram.Components[0].CPU != 600 {
		t.Fatalf("expected legacy container_cpu to be restored, got %d", ram.Components[0].CPU)
	}
}

func TestNormalizeImportedRAMRestoresMemoryFromLegacyScalingRule(t *testing.T) {
	rawMetadata := []byte(`{
		"group_key": "demo-app",
		"group_name": "Demo App",
		"group_version": "1.0.0",
		"apps": [{
			"service_key": "svc-key",
			"extend_method_map": {
				"min_memory": 64
			},
			"service_extend_method": {
				"init_memory": 1024,
				"container_cpu": 600
			}
		}]
	}`)
	var ram v1alpha1.RainbondApplicationConfig
	if err := json.Unmarshal(rawMetadata, &ram); err != nil {
		t.Fatal(err)
	}

	normalizeImportedRAM(rawMetadata, &ram)

	if ram.Components[0].Memory != 1024 {
		t.Fatalf("expected legacy init_memory to restore component memory, got %d", ram.Components[0].Memory)
	}
	if ram.Components[0].CPU != 600 {
		t.Fatalf("expected legacy container_cpu to restore component CPU, got %d", ram.Components[0].CPU)
	}
}

func TestNormalizeImportedRAMClearsDaemonSetNodeScaling(t *testing.T) {
	rawMetadata := []byte(`{
		"group_key": "demo-app",
		"group_name": "Demo App",
		"group_version": "1.0.0",
		"apps": [{
			"service_key": "svc-key",
			"extend_method": "daemonset",
			"extend_method_map": {
				"min_node": 2,
				"max_node": 7,
				"step_node": 2,
				"min_memory": 64,
				"init_memory": 1024,
				"container_cpu": 600
			}
		}]
	}`)
	var ram v1alpha1.RainbondApplicationConfig
	if err := json.Unmarshal(rawMetadata, &ram); err != nil {
		t.Fatal(err)
	}

	normalizeImportedRAM(rawMetadata, &ram)

	rule := ram.Components[0].ExtendMethodRule
	if rule.MinNode != 0 || rule.MaxNode != 0 || rule.StepNode != 0 {
		t.Fatalf("expected daemonset node scaling fields to be cleared, got min=%d max=%d step=%d", rule.MinNode, rule.MaxNode, rule.StepNode)
	}
	if rule.InitMemory != 1024 || ram.Components[0].CPU != 600 {
		t.Fatalf("expected daemonset resource settings to be preserved, got init_memory=%d cpu=%d", rule.InitMemory, ram.Components[0].CPU)
	}
}

// capability_id: rainbond.app-import.wait-image-push
func TestEnsureImportedImagesPushedPushesComponentsAndPluginsOnce(t *testing.T) {
	ram := &v1alpha1.RainbondApplicationConfig{
		Components: []*v1alpha1.Component{
			{ShareImage: "14.103.42.22/rainbond/demo:v1"},
			nil,
			{ShareImage: "14.103.42.22/rainbond/demo:v1"},
		},
		Plugins: []*v1alpha1.Plugin{
			{ShareImage: "14.103.42.22/rainbond/plugin:v1"},
		},
	}
	client := &recordingImageClient{}

	err := ensureImportedImagesPushed(client, ram, api_model.ServiceImage{
		HubUser:     "hub-user",
		HubPassword: "hub-pass",
	}, nil)

	if err != nil {
		t.Fatalf("ensureImportedImagesPushed returned error: %v", err)
	}
	want := []string{"14.103.42.22/rainbond/demo:v1", "14.103.42.22/rainbond/plugin:v1"}
	if !reflect.DeepEqual(client.pushed, want) {
		t.Fatalf("pushed images=%v, want %v", client.pushed, want)
	}
}

// capability_id: rainbond.app-import.propagate-image-push-error
func TestEnsureImportedImagesPushedReturnsPushError(t *testing.T) {
	client := &recordingImageClient{pushErr: fmt.Errorf("registry unavailable")}

	err := ensureImportedImagesPushed(client, &v1alpha1.RainbondApplicationConfig{
		Components: []*v1alpha1.Component{{ShareImage: "14.103.42.22/rainbond/demo:v1"}},
	}, api_model.ServiceImage{}, nil)

	if err == nil {
		t.Fatalf("expected push error")
	}
	if !strings.Contains(err.Error(), "14.103.42.22/rainbond/demo:v1") {
		t.Fatalf("error should include image name, got %v", err)
	}
}

// capability_id: rainbond.app-import.propagate-task-error
func TestRunImportAppTasksReturnsTaskError(t *testing.T) {
	_, err := runImportAppTasks([]string{"ok-app", "bad-app"}, func(app string) (*v1alpha1.RainbondApplicationConfig, error) {
		if app == "bad-app" {
			return nil, fmt.Errorf("import failed")
		}
		return &v1alpha1.RainbondApplicationConfig{AppName: app}, nil
	})

	if err == nil {
		t.Fatalf("expected task error")
	}
	if !strings.Contains(err.Error(), "bad-app") {
		t.Fatalf("error should include app name, got %v", err)
	}
}

type recordingImageClient struct {
	pushed  []string
	pushErr error
}

func (r *recordingImageClient) GetContainerdClient() *containerd.Client { return nil }
func (r *recordingImageClient) GetDockerClient() *dockercli.Client      { return nil }
func (r *recordingImageClient) CheckIfImageExists(imageName string) (string, bool, error) {
	return imageName, false, nil
}
func (r *recordingImageClient) ImagePull(string, string, string, event.Logger, int) (*ocispec.ImageConfig, error) {
	return nil, nil
}
func (r *recordingImageClient) ImageTag(string, string, event.Logger, int) error { return nil }
func (r *recordingImageClient) ImagePush(image, user, pass string, logger event.Logger, timeout int) error {
	r.pushed = append(r.pushed, image)
	return r.pushErr
}
func (r *recordingImageClient) ImagesPullAndPush(string, string, string, string, event.Logger) error {
	return nil
}
func (r *recordingImageClient) ImageRemove(string) error { return nil }
func (r *recordingImageClient) ImageSave(string, string) error {
	return nil
}
func (r *recordingImageClient) ImageLoad(string, event.Logger) ([]string, error) {
	return nil, nil
}
func (r *recordingImageClient) TrustedImagePush(string, string, string, event.Logger, int) error {
	return nil
}
func (r *recordingImageClient) GetImageMetadata(string, string, string, event.Logger) (*ocispec.ImageConfig, error) {
	return nil, nil
}
