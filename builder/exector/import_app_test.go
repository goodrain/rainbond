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
	"reflect"
	"testing"

	"github.com/goodrain/rainbond-oam/pkg/ram/v1alpha1"
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
