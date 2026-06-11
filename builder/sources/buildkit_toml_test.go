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
	"fmt"
	"testing"
)

// capability_id: rainbond.dockerfile-build.registry-mirror-toml
func TestBuildKitTomlContent(t *testing.T) {
	imageDomain := "goodrain.me"

	tests := []struct {
		name    string
		mirrors []string
		want    string
	}{
		{
			// zero-regression guard: empty mirrors MUST byte-for-byte match
			// the historical inline content.
			name:    "empty mirrors keeps legacy content unchanged",
			mirrors: nil,
			want:    "debug = true\n[registry.\"goodrain.me\"]\n  http = true",
		},
		{
			name:    "empty slice behaves like nil",
			mirrors: []string{},
			want:    "debug = true\n[registry.\"goodrain.me\"]\n  http = true",
		},
		{
			name:    "single https mirror",
			mirrors: []string{"docker.1ms.run"},
			want: "debug = true\n[registry.\"goodrain.me\"]\n  http = true\n" +
				"[registry.\"docker.io\"]\n  mirrors = [\"docker.1ms.run\"]",
		},
		{
			name:    "multiple mirrors",
			mirrors: []string{"docker.1ms.run", "mirror.example.com"},
			want: "debug = true\n[registry.\"goodrain.me\"]\n  http = true\n" +
				"[registry.\"docker.io\"]\n  mirrors = [\"docker.1ms.run\", \"mirror.example.com\"]",
		},
		{
			name:    "http mirror emits http=true section and strips scheme in mirrors array",
			mirrors: []string{"http://mirror.internal:5000"},
			want: "debug = true\n[registry.\"goodrain.me\"]\n  http = true\n" +
				"[registry.\"docker.io\"]\n  mirrors = [\"mirror.internal:5000\"]\n" +
				"[registry.\"mirror.internal:5000\"]\n  http = true",
		},
		{
			name:    "mixed http and https mirrors",
			mirrors: []string{"docker.1ms.run", "http://mirror.internal:5000"},
			want: "debug = true\n[registry.\"goodrain.me\"]\n  http = true\n" +
				"[registry.\"docker.io\"]\n  mirrors = [\"docker.1ms.run\", \"mirror.internal:5000\"]\n" +
				"[registry.\"mirror.internal:5000\"]\n  http = true",
		},
		{
			name:    "whitespace and empty entries are ignored",
			mirrors: []string{" docker.1ms.run ", "", "   "},
			want: "debug = true\n[registry.\"goodrain.me\"]\n  http = true\n" +
				"[registry.\"docker.io\"]\n  mirrors = [\"docker.1ms.run\"]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildKitTomlContent(imageDomain, tt.mirrors)
			if got != tt.want {
				t.Fatalf("buildKitTomlContent() mismatch:\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

// Guard: the empty-mirror output must equal the exact legacy expression so a
// refactor can never silently change the existing ConfigMap payload.
func TestBuildKitTomlContentLegacyEquivalence(t *testing.T) {
	for _, imageDomain := range []string{"goodrain.me", "registry.cn-hangzhou.aliyuncs.com"} {
		legacy := fmt.Sprintf("debug = true\n[registry.\"%v\"]\n  http = true", imageDomain)
		got := buildKitTomlContent(imageDomain, nil)
		if got != legacy {
			t.Fatalf("legacy equivalence broken for %q:\n got: %q\nwant: %q", imageDomain, got, legacy)
		}
	}
}
