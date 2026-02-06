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

package code

import (
	"os"
	"path"
	"testing"
)

func TestResolveNodeVersion_Empty(t *testing.T) {
	info := ResolveNodeVersion("")
	if info.Resolved != DefaultNodeVersion {
		t.Errorf("Expected %s for empty version, got %s", DefaultNodeVersion, info.Resolved)
	}
	if info.Source != "default" {
		t.Errorf("Expected source 'default', got %s", info.Source)
	}
}

func TestResolveNodeVersion_Wildcard(t *testing.T) {
	tests := []string{"*", "latest"}
	for _, v := range tests {
		info := ResolveNodeVersion(v)
		if info.Resolved != DefaultNodeVersion {
			t.Errorf("Expected %s for '%s', got %s", DefaultNodeVersion, v, info.Resolved)
		}
	}
}

func TestResolveNodeVersion_GreaterThanOrEqual(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		major    int
	}{
		{">=18.0.0", "18.x", 18},
		{">=20.0.0", "20.x", 20},
		{">=22.0.0", "22.x", 22},
	}

	for _, tt := range tests {
		info := ResolveNodeVersion(tt.input)
		if info.Resolved != tt.expected {
			t.Errorf("ResolveNodeVersion(%q) = %q, want %q", tt.input, info.Resolved, tt.expected)
		}
		if info.Major != tt.major {
			t.Errorf("ResolveNodeVersion(%q) major = %d, want %d", tt.input, info.Major, tt.major)
		}
	}
}

func TestResolveNodeVersion_Caret(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		major    int
	}{
		{"^18.0.0", "18.x", 18},
		{"^20.0.0", "20.x", 20},
		{"^20.10.0", "20.x", 20},
	}

	for _, tt := range tests {
		info := ResolveNodeVersion(tt.input)
		if info.Resolved != tt.expected {
			t.Errorf("ResolveNodeVersion(%q) = %q, want %q", tt.input, info.Resolved, tt.expected)
		}
		if info.Major != tt.major {
			t.Errorf("ResolveNodeVersion(%q) major = %d, want %d", tt.input, info.Major, tt.major)
		}
	}
}

func TestResolveNodeVersion_Tilde(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		major    int
	}{
		{"~18.0.0", "18.x", 18},
		{"~20.10.0", "20.x", 20},
	}

	for _, tt := range tests {
		info := ResolveNodeVersion(tt.input)
		if info.Resolved != tt.expected {
			t.Errorf("ResolveNodeVersion(%q) = %q, want %q", tt.input, info.Resolved, tt.expected)
		}
	}
}

func TestResolveNodeVersion_XNotation(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		major    int
	}{
		{"20.x", "20.x", 20},
		{"18.x", "18.x", 18},
		{"22.x", "22.x", 22},
		{"20.X", "20.x", 20},
		{"20.*", "20.x", 20},
	}

	for _, tt := range tests {
		info := ResolveNodeVersion(tt.input)
		if info.Resolved != tt.expected {
			t.Errorf("ResolveNodeVersion(%q) = %q, want %q", tt.input, info.Resolved, tt.expected)
		}
		if info.Major != tt.major {
			t.Errorf("ResolveNodeVersion(%q) major = %d, want %d", tt.input, info.Major, tt.major)
		}
	}
}

func TestResolveNodeVersion_MajorOnly(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		major    int
	}{
		{"20", "20.x", 20},
		{"18", "18.x", 18},
		{"22", "22.x", 22},
	}

	for _, tt := range tests {
		info := ResolveNodeVersion(tt.input)
		if info.Resolved != tt.expected {
			t.Errorf("ResolveNodeVersion(%q) = %q, want %q", tt.input, info.Resolved, tt.expected)
		}
	}
}

func TestResolveNodeVersion_ExactVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		major    int
		minor    int
		patch    int
	}{
		{"20.10.0", "20.x", 20, 10, 0},
		{"18.19.1", "18.x", 18, 19, 1},
		{"22.0.0", "22.x", 22, 0, 0},
	}

	for _, tt := range tests {
		info := ResolveNodeVersion(tt.input)
		if info.Resolved != tt.expected {
			t.Errorf("ResolveNodeVersion(%q) = %q, want %q", tt.input, info.Resolved, tt.expected)
		}
		if info.Minor != tt.minor {
			t.Errorf("ResolveNodeVersion(%q) minor = %d, want %d", tt.input, info.Minor, tt.minor)
		}
		if info.Patch != tt.patch {
			t.Errorf("ResolveNodeVersion(%q) patch = %d, want %d", tt.input, info.Patch, tt.patch)
		}
	}
}

func TestResolveNodeVersion_UnsupportedVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected int // expected major version after fallback
	}{
		{"14.0.0", 18},  // too old, should use oldest supported
		{"16.0.0", 18},  // too old, should use oldest supported
		{"24.0.0", 22},  // too new, should use newest supported
		{"100.0.0", 22}, // way too new
	}

	for _, tt := range tests {
		info := ResolveNodeVersion(tt.input)
		if info.Major != tt.expected {
			t.Errorf("ResolveNodeVersion(%q) major = %d, want %d (fallback)", tt.input, info.Major, tt.expected)
		}
	}
}

func TestResolveNodeVersion_Range(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		isRange  bool
	}{
		{">=18.0.0 <20.0.0", "18.x", true},
		{"18.x || 20.x", "18.x", true},
		{"^20.0.0", "20.x", true},
		{"20.10.0", "20.x", false},
	}

	for _, tt := range tests {
		info := ResolveNodeVersion(tt.input)
		if info.Resolved != tt.expected {
			t.Errorf("ResolveNodeVersion(%q) = %q, want %q", tt.input, info.Resolved, tt.expected)
		}
		if info.IsRange != tt.isRange {
			t.Errorf("ResolveNodeVersion(%q) isRange = %v, want %v", tt.input, info.IsRange, tt.isRange)
		}
	}
}

func TestResolveNodeVersion_WithVPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"v20.10.0", "20.x"},
		{"v18.0.0", "18.x"},
	}

	for _, tt := range tests {
		info := ResolveNodeVersion(tt.input)
		if info.Resolved != tt.expected {
			t.Errorf("ResolveNodeVersion(%q) = %q, want %q", tt.input, info.Resolved, tt.expected)
		}
	}
}

func TestCleanVersionSpec(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{">=18.0.0", "18.0.0"},
		{"^20.0.0", "20.0.0"},
		{"~20.10.0", "20.10.0"},
		{"v20.10.0", "20.10.0"},
		{"=20.10.0", "20.10.0"},
		{"20.10.0", "20.10.0"},
		{">=18.0.0 <20.0.0", "18.0.0"},
		{"18.x || 20.x", "18.x"},
	}

	for _, tt := range tests {
		result := cleanVersionSpec(tt.input)
		if result != tt.expected {
			t.Errorf("cleanVersionSpec(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestExtractMajorVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"20.10.0", 20},
		{"18.x", 18},
		{"22", 22},
		{"20.X", 20},
		{"20.*", 20},
		{"invalid", 0},
		{"", 0},
	}

	for _, tt := range tests {
		result := extractMajorVersion(tt.input)
		if result != tt.expected {
			t.Errorf("extractMajorVersion(%q) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

func TestExtractMinorPatch(t *testing.T) {
	tests := []struct {
		input         string
		expectedMinor int
		expectedPatch int
	}{
		{"20.10.5", 10, 5},
		{"20.10", 10, 0},
		{"20", 0, 0},
		{"20.x", 0, 0},
	}

	for _, tt := range tests {
		minor, patch := extractMinorPatch(tt.input)
		if minor != tt.expectedMinor {
			t.Errorf("extractMinorPatch(%q) minor = %d, want %d", tt.input, minor, tt.expectedMinor)
		}
		if patch != tt.expectedPatch {
			t.Errorf("extractMinorPatch(%q) patch = %d, want %d", tt.input, patch, tt.expectedPatch)
		}
	}
}

func TestNodeVersionInfo_IsLTS(t *testing.T) {
	tests := []struct {
		major    int
		expected bool
	}{
		{18, true},
		{19, false},
		{20, true},
		{21, false},
		{22, true},
	}

	for _, tt := range tests {
		info := NodeVersionInfo{Major: tt.major}
		if result := info.IsLTS(); result != tt.expected {
			t.Errorf("NodeVersionInfo{Major: %d}.IsLTS() = %v, want %v", tt.major, result, tt.expected)
		}
	}
}

func TestNodeVersionInfo_GetNodeVersionDisplay(t *testing.T) {
	tests := []struct {
		info     NodeVersionInfo
		expected string
	}{
		{NodeVersionInfo{Original: ">=20.0.0", Resolved: "20.x"}, "20.x (from >=20.0.0)"},
		{NodeVersionInfo{Original: "20.x", Resolved: "20.x"}, "20.x"},
		{NodeVersionInfo{Original: "", Resolved: "20.x"}, "20.x"},
	}

	for _, tt := range tests {
		result := tt.info.GetNodeVersionDisplay()
		if result != tt.expected {
			t.Errorf("GetNodeVersionDisplay() = %q, want %q", result, tt.expected)
		}
	}
}

func TestParseNodeVersionFromPackageJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-node-version-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create package.json with engines.node
	packageJSON := `{
		"name": "test-app",
		"engines": {
			"node": ">=20.0.0"
		}
	}`
	if err := os.WriteFile(path.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	info := ParseNodeVersionFromPackageJSON(tmpDir)

	if info.Resolved != "20.x" {
		t.Errorf("Expected resolved version '20.x', got %s", info.Resolved)
	}
	if info.Original != ">=20.0.0" {
		t.Errorf("Expected original version '>=20.0.0', got %s", info.Original)
	}
	if info.Source != "engines.node" {
		t.Errorf("Expected source 'engines.node', got %s", info.Source)
	}
}

func TestParseNodeVersionFromPackageJSON_NoEngines(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-node-version-no-engines-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create package.json without engines
	packageJSON := `{
		"name": "test-app"
	}`
	if err := os.WriteFile(path.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	info := ParseNodeVersionFromPackageJSON(tmpDir)

	if info.Resolved != DefaultNodeVersion {
		t.Errorf("Expected default version %s, got %s", DefaultNodeVersion, info.Resolved)
	}
	if info.Source != "default" {
		t.Errorf("Expected source 'default', got %s", info.Source)
	}
}

func TestParseNodeVersionFromPackageJSON_NoPackageJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-node-version-no-pkg-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	info := ParseNodeVersionFromPackageJSON(tmpDir)

	if info.Resolved != DefaultNodeVersion {
		t.Errorf("Expected default version %s, got %s", DefaultNodeVersion, info.Resolved)
	}
	if info.Source != "default" {
		t.Errorf("Expected source 'default', got %s", info.Source)
	}
}
