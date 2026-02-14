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

func TestDetectPackageManager_PNPM(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-pnpm-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create pnpm-lock.yaml
	if err := os.WriteFile(path.Join(tmpDir, "pnpm-lock.yaml"), []byte("lockfileVersion: 6.0"), 0644); err != nil {
		t.Fatalf("Failed to write pnpm-lock.yaml: %v", err)
	}

	pm := DetectPackageManager(tmpDir)

	if pm.Manager != PackageManagerPNPM {
		t.Errorf("Expected manager 'pnpm', got '%s'", pm.Manager)
	}
	if pm.LockFile != "pnpm-lock.yaml" {
		t.Errorf("Expected lock file 'pnpm-lock.yaml', got '%s'", pm.LockFile)
	}
}

func TestDetectPackageManager_Yarn(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-yarn-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create yarn.lock
	if err := os.WriteFile(path.Join(tmpDir, "yarn.lock"), []byte("# yarn lockfile v1"), 0644); err != nil {
		t.Fatalf("Failed to write yarn.lock: %v", err)
	}

	pm := DetectPackageManager(tmpDir)

	if pm.Manager != PackageManagerYarn {
		t.Errorf("Expected manager 'yarn', got '%s'", pm.Manager)
	}
	if pm.LockFile != "yarn.lock" {
		t.Errorf("Expected lock file 'yarn.lock', got '%s'", pm.LockFile)
	}
}

func TestDetectPackageManager_NPM(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-npm-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create package-lock.json
	if err := os.WriteFile(path.Join(tmpDir, "package-lock.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to write package-lock.json: %v", err)
	}

	pm := DetectPackageManager(tmpDir)

	if pm.Manager != PackageManagerNPM {
		t.Errorf("Expected manager 'npm', got '%s'", pm.Manager)
	}
	if pm.LockFile != "package-lock.json" {
		t.Errorf("Expected lock file 'package-lock.json', got '%s'", pm.LockFile)
	}
}

func TestDetectPackageManager_Priority(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-priority-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create all lock files - pnpm should win
	os.WriteFile(path.Join(tmpDir, "pnpm-lock.yaml"), []byte(""), 0644)
	os.WriteFile(path.Join(tmpDir, "yarn.lock"), []byte(""), 0644)
	os.WriteFile(path.Join(tmpDir, "package-lock.json"), []byte(""), 0644)

	pm := DetectPackageManager(tmpDir)

	if pm.Manager != PackageManagerPNPM {
		t.Errorf("Expected pnpm to have highest priority, got '%s'", pm.Manager)
	}
}

func TestDetectPackageManager_YarnOverNPM(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-yarn-npm-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create yarn.lock and package-lock.json - yarn should win
	os.WriteFile(path.Join(tmpDir, "yarn.lock"), []byte(""), 0644)
	os.WriteFile(path.Join(tmpDir, "package-lock.json"), []byte(""), 0644)

	pm := DetectPackageManager(tmpDir)

	if pm.Manager != PackageManagerYarn {
		t.Errorf("Expected yarn to have priority over npm, got '%s'", pm.Manager)
	}
}

func TestDetectPackageManager_Default(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-default-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// No lock files - should default to npm
	pm := DetectPackageManager(tmpDir)

	if pm.Manager != PackageManagerNPM {
		t.Errorf("Expected default manager 'npm', got '%s'", pm.Manager)
	}
}

func TestDetectPackageManager_PackageManagerField(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-pm-field-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create package.json with packageManager field
	packageJSON := `{
		"name": "test-app",
		"packageManager": "pnpm@8.15.0"
	}`
	if err := os.WriteFile(path.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	pm := DetectPackageManager(tmpDir)

	if pm.Manager != PackageManagerPNPM {
		t.Errorf("Expected manager 'pnpm', got '%s'", pm.Manager)
	}
	if pm.Version != "8.15.0" {
		t.Errorf("Expected version '8.15.0', got '%s'", pm.Version)
	}
}

func TestDetectPackageManager_PackageManagerFieldYarn(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-pm-field-yarn-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packageJSON := `{
		"name": "test-app",
		"packageManager": "yarn@4.0.0"
	}`
	if err := os.WriteFile(path.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	pm := DetectPackageManager(tmpDir)

	if pm.Manager != PackageManagerYarn {
		t.Errorf("Expected manager 'yarn', got '%s'", pm.Manager)
	}
	if pm.Version != "4.0.0" {
		t.Errorf("Expected version '4.0.0', got '%s'", pm.Version)
	}
}

func TestParsePackageManagerField(t *testing.T) {
	tests := []struct {
		input           string
		expectedManager PackageManager
		expectedVersion string
		expectNil       bool
	}{
		{"pnpm@8.15.0", PackageManagerPNPM, "8.15.0", false},
		{"yarn@4.0.0", PackageManagerYarn, "4.0.0", false},
		{"npm@10.0.0", PackageManagerNPM, "10.0.0", false},
		{"pnpm", PackageManagerPNPM, "", false},
		{"PNPM@8.0.0", PackageManagerPNPM, "8.0.0", false},
		{"unknown@1.0.0", "", "", true},
		{"", "", "", true},
	}

	for _, tt := range tests {
		result := parsePackageManagerField(tt.input)
		if tt.expectNil {
			if result != nil {
				t.Errorf("parsePackageManagerField(%q) expected nil, got %+v", tt.input, result)
			}
			continue
		}
		if result == nil {
			t.Errorf("parsePackageManagerField(%q) expected non-nil result", tt.input)
			continue
		}
		if result.Manager != tt.expectedManager {
			t.Errorf("parsePackageManagerField(%q) manager = %q, want %q", tt.input, result.Manager, tt.expectedManager)
		}
		if result.Version != tt.expectedVersion {
			t.Errorf("parsePackageManagerField(%q) version = %q, want %q", tt.input, result.Version, tt.expectedVersion)
		}
	}
}

func TestPackageManagerInfo_GetCommands(t *testing.T) {
	tests := []struct {
		manager        PackageManager
		installCmd     string
		buildCmd       string
		startCmd       string
	}{
		{PackageManagerPNPM, "pnpm install --frozen-lockfile", "pnpm run build", "pnpm start"},
		{PackageManagerYarn, "yarn install --frozen-lockfile", "yarn build", "yarn start"},
		{PackageManagerNPM, "npm ci", "npm run build", "npm start"},
	}

	for _, tt := range tests {
		pm := PackageManagerInfo{Manager: tt.manager}

		if cmd := pm.GetInstallCommand(); cmd != tt.installCmd {
			t.Errorf("GetInstallCommand() for %s = %q, want %q", tt.manager, cmd, tt.installCmd)
		}
		if cmd := pm.GetBuildCommand(); cmd != tt.buildCmd {
			t.Errorf("GetBuildCommand() for %s = %q, want %q", tt.manager, cmd, tt.buildCmd)
		}
		if cmd := pm.GetStartCommand(); cmd != tt.startCmd {
			t.Errorf("GetStartCommand() for %s = %q, want %q", tt.manager, cmd, tt.startCmd)
		}
	}
}

func TestPackageManager_String(t *testing.T) {
	tests := []struct {
		pm       PackageManager
		expected string
	}{
		{PackageManagerNPM, "npm"},
		{PackageManagerYarn, "yarn"},
		{PackageManagerPNPM, "pnpm"},
	}

	for _, tt := range tests {
		if result := tt.pm.String(); result != tt.expected {
			t.Errorf("PackageManager.String() = %q, want %q", result, tt.expected)
		}
	}
}
