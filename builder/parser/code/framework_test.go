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

func TestDetectFramework_NextJS_NoConfigFile(t *testing.T) {
	// Next.js project with only package.json, no next.config.* file
	// Should still be detected as dynamic (SSR)
	tmpDir, err := os.MkdirTemp("", "test-nextjs-noconfig-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packageJSON := `{
		"name": "test-nextjs-noconfig",
		"dependencies": {
			"next": "14.2.3",
			"react": "18.2.0"
		}
	}`
	if err := os.WriteFile(path.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	framework := DetectFramework(tmpDir)
	if framework == nil {
		t.Fatal("Expected to detect Next.js framework without config file, got nil")
	}

	if framework.Name != "nextjs" {
		t.Errorf("Expected framework name 'nextjs', got '%s'", framework.Name)
	}
	if framework.RuntimeType != "dynamic" {
		t.Errorf("Expected runtime type 'dynamic', got '%s'", framework.RuntimeType)
	}
	if framework.OutputDir != ".next" {
		t.Errorf("Expected output dir '.next', got '%s'", framework.OutputDir)
	}
}

func TestDetectFramework_NextJS_StaticExport(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-nextjs-static-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packageJSON := `{
		"name": "test-nextjs-static",
		"dependencies": {
			"next": "14.2.3",
			"react": "18.2.0"
		}
	}`
	if err := os.WriteFile(path.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	// next.config.mjs with output: 'export'
	nextConfig := `const nextConfig = {
  output: 'export',
  images: { unoptimized: true },
}
export default nextConfig`
	if err := os.WriteFile(path.Join(tmpDir, "next.config.mjs"), []byte(nextConfig), 0644); err != nil {
		t.Fatalf("Failed to write next.config.mjs: %v", err)
	}

	framework := DetectFramework(tmpDir)
	if framework == nil {
		t.Fatal("Expected to detect Next.js framework, got nil")
	}

	if framework.Name != "nextjs-static" {
		t.Errorf("Expected framework name 'nextjs-static', got '%s'", framework.Name)
	}
	if framework.RuntimeType != "static" {
		t.Errorf("Expected runtime type 'static' for output: 'export', got '%s'", framework.RuntimeType)
	}
	if framework.OutputDir != "out" {
		t.Errorf("Expected output dir 'out' for static export, got '%s'", framework.OutputDir)
	}
	if framework.StartCmd != "" {
		t.Errorf("Expected empty start cmd for static export, got '%s'", framework.StartCmd)
	}
}

func TestDetectFramework_NextJS_SSR(t *testing.T) {
	// NextJS without output: 'export' should remain dynamic
	tmpDir, err := os.MkdirTemp("", "test-nextjs-ssr-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packageJSON := `{
		"name": "test-nextjs-ssr",
		"dependencies": {
			"next": "14.2.3",
			"react": "18.2.0"
		}
	}`
	if err := os.WriteFile(path.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	nextConfig := `const nextConfig = {
  reactStrictMode: true,
}
module.exports = nextConfig`
	if err := os.WriteFile(path.Join(tmpDir, "next.config.js"), []byte(nextConfig), 0644); err != nil {
		t.Fatalf("Failed to write next.config.js: %v", err)
	}

	framework := DetectFramework(tmpDir)
	if framework == nil {
		t.Fatal("Expected to detect Next.js framework, got nil")
	}

	if framework.RuntimeType != "dynamic" {
		t.Errorf("Expected runtime type 'dynamic' for SSR, got '%s'", framework.RuntimeType)
	}
	if framework.OutputDir != ".next" {
		t.Errorf("Expected output dir '.next' for SSR, got '%s'", framework.OutputDir)
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

func TestDetectFramework_Nuxt_StaticTarget(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-nuxt-static-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packageJSON := `{
		"name": "test-nuxt-static",
		"dependencies": {
			"nuxt": "2.17.0"
		}
	}`
	if err := os.WriteFile(path.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	nuxtConfig := `export default {
  target: 'static',
  head: { title: 'My App' },
}`
	if err := os.WriteFile(path.Join(tmpDir, "nuxt.config.js"), []byte(nuxtConfig), 0644); err != nil {
		t.Fatalf("Failed to write nuxt.config.js: %v", err)
	}

	framework := DetectFramework(tmpDir)
	if framework == nil {
		t.Fatal("Expected to detect Nuxt framework, got nil")
	}
	if framework.RuntimeType != "static" {
		t.Errorf("Expected runtime type 'static' for target: 'static', got '%s'", framework.RuntimeType)
	}
	if framework.Name != "nuxt-static" {
		t.Errorf("Expected framework name 'nuxt-static', got '%s'", framework.Name)
	}
	if framework.OutputDir != "dist" {
		t.Errorf("Expected output dir 'dist', got '%s'", framework.OutputDir)
	}
	if framework.StartCmd != "" {
		t.Errorf("Expected empty start cmd, got '%s'", framework.StartCmd)
	}
}

func TestDetectFramework_Nuxt3_SSRFalse(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-nuxt3-spa-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packageJSON := `{
		"name": "test-nuxt3-spa",
		"dependencies": {
			"nuxt": "3.8.0"
		}
	}`
	if err := os.WriteFile(path.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	nuxtConfig := `export default defineNuxtConfig({
  ssr: false,
  app: { head: { title: 'SPA App' } },
})`
	if err := os.WriteFile(path.Join(tmpDir, "nuxt.config.ts"), []byte(nuxtConfig), 0644); err != nil {
		t.Fatalf("Failed to write nuxt.config.ts: %v", err)
	}

	framework := DetectFramework(tmpDir)
	if framework == nil {
		t.Fatal("Expected to detect Nuxt framework, got nil")
	}
	if framework.RuntimeType != "static" {
		t.Errorf("Expected runtime type 'static' for ssr: false, got '%s'", framework.RuntimeType)
	}
	if framework.Name != "nuxt-static" {
		t.Errorf("Expected framework name 'nuxt-static', got '%s'", framework.Name)
	}
}

func TestDetectFramework_Nuxt3_NitroStatic(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-nuxt3-nitro-static-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packageJSON := `{
		"name": "test-nuxt3-nitro-static",
		"dependencies": {
			"nuxt": "3.16.0"
		}
	}`
	if err := os.WriteFile(path.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	nuxtConfig := `export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },
  ssr: true,
  nitro: {
    static: true,
  },
})`
	if err := os.WriteFile(path.Join(tmpDir, "nuxt.config.ts"), []byte(nuxtConfig), 0644); err != nil {
		t.Fatalf("Failed to write nuxt.config.ts: %v", err)
	}

	framework := DetectFramework(tmpDir)
	if framework == nil {
		t.Fatal("Expected to detect Nuxt framework, got nil")
	}
	if framework.Name != "nuxt-static" {
		t.Errorf("Expected framework name 'nuxt-static' for nitro.static:true, got '%s'", framework.Name)
	}
	if framework.RuntimeType != "static" {
		t.Errorf("Expected runtime type 'static' for nitro.static:true, got '%s'", framework.RuntimeType)
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

func TestDetectFramework_Angular_SPA(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-angular-spa-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	packageJSON := `{
		"name": "test-angular-spa",
		"dependencies": {
			"@angular/core": "^19.2.0",
			"@angular/router": "^19.2.0"
		}
	}`
	if err := os.WriteFile(path.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}
	if err := os.WriteFile(path.Join(tmpDir, "angular.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to write angular.json: %v", err)
	}

	framework := DetectFramework(tmpDir)
	if framework == nil {
		t.Fatal("Expected to detect Angular framework, got nil")
	}
	if framework.Name != "angular" {
		t.Errorf("Expected 'angular', got '%s'", framework.Name)
	}
	if framework.RuntimeType != "static" {
		t.Errorf("Expected 'static' for SPA, got '%s'", framework.RuntimeType)
	}
}

func TestDetectFramework_Angular_SSR(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-angular-ssr-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Angular SSR project has @angular/ssr and express, but should still detect as Angular
	packageJSON := `{
		"name": "test-angular-ssr",
		"dependencies": {
			"@angular/core": "^19.2.0",
			"@angular/ssr": "^19.2.19",
			"express": "^4.18.2"
		}
	}`
	if err := os.WriteFile(path.Join(tmpDir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}
	if err := os.WriteFile(path.Join(tmpDir, "angular.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to write angular.json: %v", err)
	}

	framework := DetectFramework(tmpDir)
	if framework == nil {
		t.Fatal("Expected to detect Angular framework, got nil")
	}
	if framework.Name != "angular" {
		t.Errorf("Expected 'angular', got '%s' (should not detect as express)", framework.Name)
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
	if len(frameworks) != 13 {
		t.Errorf("Expected 13 supported frameworks, got %d", len(frameworks))
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
		{"angular", "Angular"},
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
