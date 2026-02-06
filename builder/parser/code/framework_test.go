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

func TestDetectFramework_NextJS(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "test-nextjs-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create package.json with next dependency
	packageJSON := `{
		"name": "test-nextjs-app",
		"dependencies": {
			"next": "14.2.3",
			"react": "18.2.0"
		}
	}`
	if err := os.WriteFile(path.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	// Create next.config.js
	if err := os.WriteFile(path.Join(tmpDir, "next.config.js"), []byte("module.exports = {}"), 0644); err != nil {
		t.Fatalf("Failed to write next.config.js: %v", err)
	}

	// Test detection
	framework := DetectFramework(tmpDir)
	if framework == nil {
		t.Fatal("Expected to detect Next.js framework, got nil")
	}

	if framework.Name != "nextjs" {
		t.Errorf("Expected framework name 'nextjs', got '%s'", framework.Name)
	}
	if framework.DisplayName != "Next.js" {
		t.Errorf("Expected display name 'Next.js', got '%s'", framework.DisplayName)
	}
	if framework.RuntimeType != "dynamic" {
		t.Errorf("Expected runtime type 'dynamic', got '%s'", framework.RuntimeType)
	}
	if framework.OutputDir != ".next" {
		t.Errorf("Expected output dir '.next', got '%s'", framework.OutputDir)
	}
	if framework.Version != "14.2.3" {
		t.Errorf("Expected version '14.2.3', got '%s'", framework.Version)
	}
}

func TestDetectFramework_Nuxt(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-nuxt-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packageJSON := `{
		"name": "test-nuxt-app",
		"dependencies": {
			"nuxt": "3.8.0"
		}
	}`
	if err := os.WriteFile(path.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	if err := os.WriteFile(path.Join(tmpDir, "nuxt.config.ts"), []byte("export default {}"), 0644); err != nil {
		t.Fatalf("Failed to write nuxt.config.ts: %v", err)
	}

	framework := DetectFramework(tmpDir)
	if framework == nil {
		t.Fatal("Expected to detect Nuxt framework, got nil")
	}

	if framework.Name != "nuxt" {
		t.Errorf("Expected framework name 'nuxt', got '%s'", framework.Name)
	}
	if framework.RuntimeType != "dynamic" {
		t.Errorf("Expected runtime type 'dynamic', got '%s'", framework.RuntimeType)
	}
}

func TestDetectFramework_Umi(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-umi-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packageJSON := `{
		"name": "test-umi-app",
		"dependencies": {
			"umi": "4.0.0"
		}
	}`
	if err := os.WriteFile(path.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	if err := os.WriteFile(path.Join(tmpDir, ".umirc.ts"), []byte("export default {}"), 0644); err != nil {
		t.Fatalf("Failed to write .umirc.ts: %v", err)
	}

	framework := DetectFramework(tmpDir)
	if framework == nil {
		t.Fatal("Expected to detect Umi framework, got nil")
	}

	if framework.Name != "umi" {
		t.Errorf("Expected framework name 'umi', got '%s'", framework.Name)
	}
	if framework.RuntimeType != "static" {
		t.Errorf("Expected runtime type 'static', got '%s'", framework.RuntimeType)
	}
	if framework.OutputDir != "dist" {
		t.Errorf("Expected output dir 'dist', got '%s'", framework.OutputDir)
	}
}

func TestDetectFramework_Vite(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-vite-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packageJSON := `{
		"name": "test-vite-app",
		"devDependencies": {
			"vite": "^5.0.0"
		}
	}`
	if err := os.WriteFile(path.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	if err := os.WriteFile(path.Join(tmpDir, "vite.config.ts"), []byte("export default {}"), 0644); err != nil {
		t.Fatalf("Failed to write vite.config.ts: %v", err)
	}

	framework := DetectFramework(tmpDir)
	if framework == nil {
		t.Fatal("Expected to detect Vite framework, got nil")
	}

	if framework.Name != "vite" {
		t.Errorf("Expected framework name 'vite', got '%s'", framework.Name)
	}
	if framework.RuntimeType != "static" {
		t.Errorf("Expected runtime type 'static', got '%s'", framework.RuntimeType)
	}
	if framework.Version != "5.0.0" {
		t.Errorf("Expected version '5.0.0', got '%s'", framework.Version)
	}
}

func TestDetectFramework_CRA(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-cra-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packageJSON := `{
		"name": "test-cra-app",
		"dependencies": {
			"react-scripts": "5.0.1"
		}
	}`
	if err := os.WriteFile(path.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	framework := DetectFramework(tmpDir)
	if framework == nil {
		t.Fatal("Expected to detect CRA framework, got nil")
	}

	if framework.Name != "cra" {
		t.Errorf("Expected framework name 'cra', got '%s'", framework.Name)
	}
	if framework.DisplayName != "Create React App" {
		t.Errorf("Expected display name 'Create React App', got '%s'", framework.DisplayName)
	}
	if framework.OutputDir != "build" {
		t.Errorf("Expected output dir 'build', got '%s'", framework.OutputDir)
	}
}

func TestDetectFramework_Express(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-express-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packageJSON := `{
		"name": "test-express-app",
		"dependencies": {
			"express": "4.18.2"
		}
	}`
	if err := os.WriteFile(path.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	framework := DetectFramework(tmpDir)
	if framework == nil {
		t.Fatal("Expected to detect Express framework, got nil")
	}

	if framework.Name != "express" {
		t.Errorf("Expected framework name 'express', got '%s'", framework.Name)
	}
	if framework.RuntimeType != "dynamic" {
		t.Errorf("Expected runtime type 'dynamic', got '%s'", framework.RuntimeType)
	}
}

func TestDetectFramework_NestJS(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-nestjs-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packageJSON := `{
		"name": "test-nestjs-app",
		"dependencies": {
			"@nestjs/core": "10.0.0"
		}
	}`
	if err := os.WriteFile(path.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	if err := os.WriteFile(path.Join(tmpDir, "nest-cli.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to write nest-cli.json: %v", err)
	}

	framework := DetectFramework(tmpDir)
	if framework == nil {
		t.Fatal("Expected to detect NestJS framework, got nil")
	}

	if framework.Name != "nestjs" {
		t.Errorf("Expected framework name 'nestjs', got '%s'", framework.Name)
	}
	if framework.RuntimeType != "dynamic" {
		t.Errorf("Expected runtime type 'dynamic', got '%s'", framework.RuntimeType)
	}
}

func TestDetectFramework_NoPackageJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-no-pkg-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	framework := DetectFramework(tmpDir)
	if framework != nil {
		t.Errorf("Expected nil for directory without package.json, got %+v", framework)
	}
}

func TestDetectFramework_NoFramework(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-no-framework-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packageJSON := `{
		"name": "test-plain-app",
		"dependencies": {
			"lodash": "4.17.21"
		}
	}`
	if err := os.WriteFile(path.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	framework := DetectFramework(tmpDir)
	if framework != nil {
		t.Errorf("Expected nil for plain Node.js project, got %+v", framework)
	}
}

func TestCleanVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"^14.2.3", "14.2.3"},
		{"~5.0.0", "5.0.0"},
		{">=18.0.0", "18.0.0"},
		{">10.0.0", "10.0.0"},
		{"<=20.0.0", "20.0.0"},
		{"<15.0.0", "15.0.0"},
		{"=12.0.0", "12.0.0"},
		{"14.2.3", "14.2.3"},
		{"", ""},
	}

	for _, tt := range tests {
		result := cleanVersion(tt.input)
		if result != tt.expected {
			t.Errorf("cleanVersion(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGetSupportedFrameworks(t *testing.T) {
	frameworks := GetSupportedFrameworks()
	if len(frameworks) != 11 {
		t.Errorf("Expected 11 supported frameworks, got %d", len(frameworks))
	}

	// Check that all frameworks have required fields
	for _, f := range frameworks {
		if f.Name == "" {
			t.Error("Framework name should not be empty")
		}
		if f.DisplayName == "" {
			t.Error("Framework display name should not be empty")
		}
		if f.RuntimeType != "static" && f.RuntimeType != "dynamic" {
			t.Errorf("Framework runtime type should be 'static' or 'dynamic', got '%s'", f.RuntimeType)
		}
	}
}

func TestGetDisplayName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"nextjs", "Next.js"},
		{"nuxt", "Nuxt"},
		{"umi", "Umi"},
		{"vite", "Vite"},
		{"cra", "Create React App"},
		{"vue-cli", "Vue CLI"},
		{"gatsby", "Gatsby"},
		{"docusaurus", "Docusaurus"},
		{"remix", "Remix"},
		{"express", "Express"},
		{"nestjs", "Nest.js"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		result := GetDisplayName(tt.input)
		if result != tt.expected {
			t.Errorf("GetDisplayName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
