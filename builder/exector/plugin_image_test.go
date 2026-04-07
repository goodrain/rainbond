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

package exector

import (
	"testing"

	"github.com/goodrain/rainbond/builder"
	buildmodel "github.com/goodrain/rainbond/builder/model"
	"github.com/goodrain/rainbond/event"
)

// capability_id: rainbond.plugin-build.image-tag
func TestCreatePluginImageTag(t *testing.T) {
	origDomain := builder.REGISTRYDOMAIN
	builder.REGISTRYDOMAIN = "goodrain.me"
	defer func() {
		builder.REGISTRYDOMAIN = origDomain
	}()

	tests := []struct {
		image    string
		pluginID string
		version  string
		want     string
	}{
		{
			image:    "busybox:1.36",
			pluginID: "plugin-a",
			version:  "v1",
			want:     "goodrain.me/plugin_busybox_plugin-a:1.36_v1",
		},
		{
			image:    "repo.example.com/custom/plugin-demo:2.0",
			pluginID: "plugin-b",
			version:  "v2",
			want:     "goodrain.me/plugin-demo:plugin-b_v2",
		},
		{
			image:    "plugin-runner",
			pluginID: "plugin-c",
			version:  "v3",
			want:     "goodrain.me/plugin-runner:plugin-c_v3",
		},
	}

	for _, tt := range tests {
		if got := createPluginImageTag(tt.image, tt.pluginID, tt.version); got != tt.want {
			t.Fatalf("createPluginImageTag(%q, %q, %q)=%q, want %q", tt.image, tt.pluginID, tt.version, got, tt.want)
		}
	}
}

// capability_id: rainbond.plugin-build.image-input-validate
func TestPluginImageRunRejectsEmptyImageURL(t *testing.T) {
	manager := &exectorManager{}
	err := manager.run(&buildmodel.BuildPluginTaskBody{
		ImageURL:      "   ",
		PluginID:      "plugin-a",
		DeployVersion: "v1",
	}, event.GetTestLogger())
	if err == nil {
		t.Fatal("expected empty image URL validation error")
	}
}

// capability_id: rainbond.plugin-build.image-input-validate
func TestPluginImageRunRejectsInvalidImageReference(t *testing.T) {
	manager := &exectorManager{}
	err := manager.run(&buildmodel.BuildPluginTaskBody{
		ImageURL:      "%%%invalid%%%image",
		PluginID:      "plugin-a",
		DeployVersion: "v1",
	}, event.GetTestLogger())
	if err == nil {
		t.Fatal("expected invalid image reference error")
	}
}
