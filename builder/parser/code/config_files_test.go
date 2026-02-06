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

func TestDetectConfigFiles_Npmrc(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-npmrc-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .npmrc
	npmrcContent := "registry=https://registry.npmmirror.com"
	if err := os.WriteFile(path.Join(tmpDir, ".npmrc"), []byte(npmrcContent), 0644); err != nil {
		t.Fatalf("Failed to write .npmrc: %v", err)
	}

	config := DetectConfigFiles(tmpDir)

	if !config.HasNpmrc {
		t.Error("Expected HasNpmrc to be true")
	}
	if config.HasYarnrc {
		t.Error("Expected HasYarnrc to be false")
	}
	if config.HasPnpmrc {
		t.Error("Expected HasPnpmrc to be false")
	}
	if config.NpmrcPath != path.Join(tmpDir, ".npmrc") {
		t.Errorf("Expected NpmrcPath to be %s, got %s", path.Join(tmpDir, ".npmrc"), config.NpmrcPath)
	}
}

func TestDetectConfigFiles_YarnrcClassic(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-yarnrc-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .yarnrc (classic)
	if err := os.WriteFile(path.Join(tmpDir, ".yarnrc"), []byte("registry \"https://registry.npmmirror.com\""), 0644); err != nil {
		t.Fatalf("Failed to write .yarnrc: %v", err)
	}

	config := DetectConfigFiles(tmpDir)

	if config.HasNpmrc {
		t.Error("Expected HasNpmrc to be false")
	}
	if !config.HasYarnrc {
		t.Error("Expected HasYarnrc to be true")
	}
	if config.YarnrcPath != path.Join(tmpDir, ".yarnrc") {
		t.Errorf("Expected YarnrcPath to be %s, got %s", path.Join(tmpDir, ".yarnrc"), config.YarnrcPath)
	}
}

func TestDetectConfigFiles_YarnrcYml(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-yarnrc-yml-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .yarnrc.yml (berry/modern)
	yarnrcContent := `nodeLinker: node-modules
npmRegistryServer: "https://registry.npmmirror.com"`
	if err := os.WriteFile(path.Join(tmpDir, ".yarnrc.yml"), []byte(yarnrcContent), 0644); err != nil {
		t.Fatalf("Failed to write .yarnrc.yml: %v", err)
	}

	config := DetectConfigFiles(tmpDir)

	if !config.HasYarnrc {
		t.Error("Expected HasYarnrc to be true")
	}
	if config.YarnrcPath != path.Join(tmpDir, ".yarnrc.yml") {
		t.Errorf("Expected YarnrcPath to be %s, got %s", path.Join(tmpDir, ".yarnrc.yml"), config.YarnrcPath)
	}
}

func TestDetectConfigFiles_Pnpmrc(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-pnpmrc-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .pnpmrc
	if err := os.WriteFile(path.Join(tmpDir, ".pnpmrc"), []byte("shamefully-hoist=true"), 0644); err != nil {
		t.Fatalf("Failed to write .pnpmrc: %v", err)
	}

	config := DetectConfigFiles(tmpDir)

	if !config.HasPnpmrc {
		t.Error("Expected HasPnpmrc to be true")
	}
	if config.PnpmrcPath != path.Join(tmpDir, ".pnpmrc") {
		t.Errorf("Expected PnpmrcPath to be %s, got %s", path.Join(tmpDir, ".pnpmrc"), config.PnpmrcPath)
	}
}

func TestDetectConfigFiles_Multiple(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-multi-config-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create multiple config files
	os.WriteFile(path.Join(tmpDir, ".npmrc"), []byte("registry=https://registry.npmmirror.com"), 0644)
	os.WriteFile(path.Join(tmpDir, ".yarnrc.yml"), []byte("nodeLinker: node-modules"), 0644)
	os.WriteFile(path.Join(tmpDir, ".pnpmrc"), []byte("shamefully-hoist=true"), 0644)

	config := DetectConfigFiles(tmpDir)

	if !config.HasNpmrc {
		t.Error("Expected HasNpmrc to be true")
	}
	if !config.HasYarnrc {
		t.Error("Expected HasYarnrc to be true")
	}
	if !config.HasPnpmrc {
		t.Error("Expected HasPnpmrc to be true")
	}
}

func TestDetectConfigFiles_None(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-no-config-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := DetectConfigFiles(tmpDir)

	if config.HasNpmrc {
		t.Error("Expected HasNpmrc to be false")
	}
	if config.HasYarnrc {
		t.Error("Expected HasYarnrc to be false")
	}
	if config.HasPnpmrc {
		t.Error("Expected HasPnpmrc to be false")
	}
	if config.HasAnyConfigFile() {
		t.Error("Expected HasAnyConfigFile to be false")
	}
}

func TestConfigFiles_GetNpmrcContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-npmrc-content-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	expectedContent := "registry=https://registry.npmmirror.com\n@private:registry=https://npm.company.com"
	if err := os.WriteFile(path.Join(tmpDir, ".npmrc"), []byte(expectedContent), 0644); err != nil {
		t.Fatalf("Failed to write .npmrc: %v", err)
	}

	config := DetectConfigFiles(tmpDir)
	content, err := config.GetNpmrcContent()

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if content != expectedContent {
		t.Errorf("Expected content %q, got %q", expectedContent, content)
	}
}

func TestConfigFiles_GetYarnrcContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-yarnrc-content-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	expectedContent := "nodeLinker: node-modules"
	if err := os.WriteFile(path.Join(tmpDir, ".yarnrc.yml"), []byte(expectedContent), 0644); err != nil {
		t.Fatalf("Failed to write .yarnrc.yml: %v", err)
	}

	config := DetectConfigFiles(tmpDir)
	content, err := config.GetYarnrcContent()

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if content != expectedContent {
		t.Errorf("Expected content %q, got %q", expectedContent, content)
	}
}

func TestConfigFiles_HasAnyConfigFile(t *testing.T) {
	tests := []struct {
		name     string
		config   ConfigFiles
		expected bool
	}{
		{"no config", ConfigFiles{}, false},
		{"npmrc only", ConfigFiles{HasNpmrc: true}, true},
		{"yarnrc only", ConfigFiles{HasYarnrc: true}, true},
		{"pnpmrc only", ConfigFiles{HasPnpmrc: true}, true},
		{"all configs", ConfigFiles{HasNpmrc: true, HasYarnrc: true, HasPnpmrc: true}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := tt.config.HasAnyConfigFile(); result != tt.expected {
				t.Errorf("HasAnyConfigFile() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestConfigFiles_GetRelevantConfigFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-relevant-config-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create all config files
	npmrcPath := path.Join(tmpDir, ".npmrc")
	yarnrcPath := path.Join(tmpDir, ".yarnrc.yml")
	pnpmrcPath := path.Join(tmpDir, ".pnpmrc")

	os.WriteFile(npmrcPath, []byte(""), 0644)
	os.WriteFile(yarnrcPath, []byte(""), 0644)
	os.WriteFile(pnpmrcPath, []byte(""), 0644)

	config := DetectConfigFiles(tmpDir)

	tests := []struct {
		pm       PackageManager
		expected string
	}{
		{PackageManagerNPM, npmrcPath},
		{PackageManagerYarn, yarnrcPath},
		{PackageManagerPNPM, pnpmrcPath}, // pnpm prefers .pnpmrc if exists
	}

	for _, tt := range tests {
		result := config.GetRelevantConfigFile(tt.pm)
		if result != tt.expected {
			t.Errorf("GetRelevantConfigFile(%s) = %q, want %q", tt.pm, result, tt.expected)
		}
	}
}

func TestConfigFiles_GetRelevantConfigFile_PnpmFallback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-pnpm-fallback-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create only .npmrc (pnpm should fall back to it)
	npmrcPath := path.Join(tmpDir, ".npmrc")
	os.WriteFile(npmrcPath, []byte(""), 0644)

	config := DetectConfigFiles(tmpDir)

	result := config.GetRelevantConfigFile(PackageManagerPNPM)
	if result != npmrcPath {
		t.Errorf("GetRelevantConfigFile(pnpm) should fall back to .npmrc, got %q", result)
	}
}
