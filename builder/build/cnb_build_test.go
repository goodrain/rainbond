// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

package build

import (
	"os"
	"testing"
)

func TestBuildEnvVars_BasicEnvs(t *testing.T) {
	c := &cnbBuild{}
	re := &Request{
		BuildEnvs: map[string]string{},
	}

	envs := c.buildEnvVars(re)

	// Check that basic envs are always present
	envMap := make(map[string]string)
	for _, env := range envs {
		envMap[env.Name] = env.Value
	}

	if _, ok := envMap["CNB_PLATFORM_API"]; !ok {
		t.Error("Expected CNB_PLATFORM_API to be set")
	}
	if _, ok := envMap["DOCKER_CONFIG"]; !ok {
		t.Error("Expected DOCKER_CONFIG to be set")
	}
	if envMap["DOCKER_CONFIG"] != "/home/cnb/.docker" {
		t.Errorf("Expected DOCKER_CONFIG to be '/home/cnb/.docker', got '%s'", envMap["DOCKER_CONFIG"])
	}
}

func TestBuildEnvVars_NodeVersion(t *testing.T) {
	tests := []struct {
		name      string
		buildEnvs map[string]string
		wantKey   string
		wantValue string
	}{
		{
			name:      "CNB_NODE_VERSION set",
			buildEnvs: map[string]string{"CNB_NODE_VERSION": "20.10.0"},
			wantKey:   "BP_NODE_VERSION",
			wantValue: "20.10.0",
		},
		{
			name:      "RUNTIMES fallback",
			buildEnvs: map[string]string{"RUNTIMES": "18.17.0"},
			wantKey:   "BP_NODE_VERSION",
			wantValue: "18.17.0",
		},
		{
			name:      "CNB_NODE_VERSION takes priority over RUNTIMES",
			buildEnvs: map[string]string{"CNB_NODE_VERSION": "20.10.0", "RUNTIMES": "18.17.0"},
			wantKey:   "BP_NODE_VERSION",
			wantValue: "20.10.0",
		},
		{
			name:      "No node version set",
			buildEnvs: map[string]string{},
			wantKey:   "",
			wantValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &cnbBuild{}
			re := &Request{
				BuildEnvs: tt.buildEnvs,
			}

			envs := c.buildEnvVars(re)
			envMap := make(map[string]string)
			for _, env := range envs {
				envMap[env.Name] = env.Value
			}

			if tt.wantKey == "" {
				if _, ok := envMap["BP_NODE_VERSION"]; ok {
					t.Errorf("Expected BP_NODE_VERSION to not be set, but got '%s'", envMap["BP_NODE_VERSION"])
				}
			} else {
				if envMap[tt.wantKey] != tt.wantValue {
					t.Errorf("Expected %s='%s', got '%s'", tt.wantKey, tt.wantValue, envMap[tt.wantKey])
				}
			}
		})
	}
}

func TestBuildEnvVars_StaticBuild(t *testing.T) {
	c := &cnbBuild{}
	re := &Request{
		BuildEnvs: map[string]string{
			"CNB_OUTPUT_DIR": "dist",
		},
	}

	envs := c.buildEnvVars(re)
	envMap := make(map[string]string)
	for _, env := range envs {
		envMap[env.Name] = env.Value
	}

	// Check nginx configuration for static builds
	if envMap["BP_WEB_SERVER"] != "nginx" {
		t.Errorf("Expected BP_WEB_SERVER='nginx', got '%s'", envMap["BP_WEB_SERVER"])
	}
	if envMap["BP_WEB_SERVER_ROOT"] != "dist" {
		t.Errorf("Expected BP_WEB_SERVER_ROOT='dist', got '%s'", envMap["BP_WEB_SERVER_ROOT"])
	}
	if envMap["BP_WEB_SERVER_ENABLE_PUSH_STATE"] != "true" {
		t.Errorf("Expected BP_WEB_SERVER_ENABLE_PUSH_STATE='true', got '%s'", envMap["BP_WEB_SERVER_ENABLE_PUSH_STATE"])
	}
}

func TestBuildEnvVars_BuildScript(t *testing.T) {
	c := &cnbBuild{}
	re := &Request{
		BuildEnvs: map[string]string{
			"CNB_BUILD_SCRIPT": "build",
		},
	}

	envs := c.buildEnvVars(re)
	envMap := make(map[string]string)
	for _, env := range envs {
		envMap[env.Name] = env.Value
	}

	// Check that build script is correctly passed
	if envMap["BP_NODE_RUN_SCRIPTS"] != "build" {
		t.Errorf("Expected BP_NODE_RUN_SCRIPTS='build', got '%s'", envMap["BP_NODE_RUN_SCRIPTS"])
	}
}

func TestBuildEnvVars_BPPrefix(t *testing.T) {
	c := &cnbBuild{}
	re := &Request{
		BuildEnvs: map[string]string{
			"BP_CUSTOM_VAR":     "custom_value",
			"BP_ANOTHER":        "another_value",
			"NOT_BP_VAR":        "should_not_appear",
			"SOME_OTHER_VAR":    "also_not",
			"BP_":               "edge_case_empty_suffix",
		},
	}

	envs := c.buildEnvVars(re)
	envMap := make(map[string]string)
	for _, env := range envs {
		envMap[env.Name] = env.Value
	}

	// Check BP_ prefixed vars are included
	if envMap["BP_CUSTOM_VAR"] != "custom_value" {
		t.Errorf("Expected BP_CUSTOM_VAR='custom_value', got '%s'", envMap["BP_CUSTOM_VAR"])
	}
	if envMap["BP_ANOTHER"] != "another_value" {
		t.Errorf("Expected BP_ANOTHER='another_value', got '%s'", envMap["BP_ANOTHER"])
	}
	// Check edge case: BP_ with empty suffix should still be included
	if envMap["BP_"] != "edge_case_empty_suffix" {
		t.Errorf("Expected BP_='edge_case_empty_suffix', got '%s'", envMap["BP_"])
	}

	// Check non-BP_ vars are NOT included
	if _, ok := envMap["NOT_BP_VAR"]; ok {
		t.Error("NOT_BP_VAR should not be included in env vars")
	}
	if _, ok := envMap["SOME_OTHER_VAR"]; ok {
		t.Error("SOME_OTHER_VAR should not be included in env vars")
	}
}

func TestBuildEnvVars_ProxySettings(t *testing.T) {
	// Set proxy environment variables
	os.Setenv("HTTP_PROXY", "http://proxy.example.com:8080")
	os.Setenv("HTTPS_PROXY", "https://proxy.example.com:8443")
	os.Setenv("NO_PROXY", "localhost,127.0.0.1")
	defer func() {
		os.Unsetenv("HTTP_PROXY")
		os.Unsetenv("HTTPS_PROXY")
		os.Unsetenv("NO_PROXY")
	}()

	c := &cnbBuild{}
	re := &Request{
		BuildEnvs: map[string]string{},
	}

	envs := c.buildEnvVars(re)
	envMap := make(map[string]string)
	for _, env := range envs {
		envMap[env.Name] = env.Value
	}

	if envMap["HTTP_PROXY"] != "http://proxy.example.com:8080" {
		t.Errorf("Expected HTTP_PROXY to be set from env")
	}
	if envMap["HTTPS_PROXY"] != "https://proxy.example.com:8443" {
		t.Errorf("Expected HTTPS_PROXY to be set from env")
	}
	if envMap["NO_PROXY"] != "localhost,127.0.0.1" {
		t.Errorf("Expected NO_PROXY to be set from env")
	}
}

func TestBuildCreatorArgs(t *testing.T) {
	c := &cnbBuild{}
	re := &Request{}

	args := c.buildCreatorArgs(re, "my-image:latest", "run-image:latest")

	// Check required arguments
	expectedArgs := map[string]bool{
		"-app=/workspace":       false,
		"-layers=/layers":       false,
		"-platform=/platform":   false,
		"-cache-dir=/cache":     false,
		"-log-level=info":       false,
	}

	for _, arg := range args {
		if _, ok := expectedArgs[arg]; ok {
			expectedArgs[arg] = true
		}
	}

	for arg, found := range expectedArgs {
		if !found {
			t.Errorf("Expected argument %s not found in creator args", arg)
		}
	}

	// Check run image argument
	runImageFound := false
	for _, arg := range args {
		if arg == "-run-image=run-image:latest" {
			runImageFound = true
			break
		}
	}
	if !runImageFound {
		t.Error("Expected -run-image argument not found")
	}

	// Check output image is last argument
	lastArg := args[len(args)-1]
	if lastArg != "my-image:latest" {
		t.Errorf("Expected last argument to be image name 'my-image:latest', got '%s'", lastArg)
	}
}

func TestBuildPlatformAnnotations(t *testing.T) {
	tests := []struct {
		name        string
		buildEnvs   map[string]string
		wantKeys    []string
		notWantKeys []string
	}{
		{
			name:        "No CNB_OUTPUT_DIR - no annotations",
			buildEnvs:   map[string]string{},
			wantKeys:    []string{},
			notWantKeys: []string{"cnb-bp-web-server", "cnb-bp-node-version"},
		},
		{
			name:        "With CNB_NODE_VERSION - node version annotation",
			buildEnvs:   map[string]string{"CNB_NODE_VERSION": "20.20.0"},
			wantKeys:    []string{"cnb-bp-node-version"},
			notWantKeys: []string{"cnb-bp-web-server"},
		},
		{
			name:        "With RUNTIMES fallback - node version annotation",
			buildEnvs:   map[string]string{"RUNTIMES": "18.17.0"},
			wantKeys:    []string{"cnb-bp-node-version"},
			notWantKeys: []string{"cnb-bp-web-server"},
		},
		{
			name:        "With CNB_OUTPUT_DIR - web server annotations",
			buildEnvs:   map[string]string{"CNB_OUTPUT_DIR": "dist"},
			wantKeys:    []string{"cnb-bp-web-server", "cnb-bp-web-server-root", "cnb-bp-web-server-enable-push-state"},
			notWantKeys: []string{},
		},
		{
			name:        "With CNB_BUILD_SCRIPT - node run scripts annotation",
			buildEnvs:   map[string]string{"CNB_BUILD_SCRIPT": "build:prod"},
			wantKeys:    []string{"cnb-bp-node-run-scripts"},
			notWantKeys: []string{"cnb-bp-web-server"},
		},
		{
			name:        "Both static build and build script",
			buildEnvs:   map[string]string{"CNB_OUTPUT_DIR": "build", "CNB_BUILD_SCRIPT": "build", "CNB_NODE_VERSION": "20.20.0"},
			wantKeys:    []string{"cnb-bp-web-server", "cnb-bp-node-run-scripts", "cnb-bp-node-version"},
			notWantKeys: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &cnbBuild{}
			re := &Request{
				BuildEnvs: tt.buildEnvs,
			}

			annotations := c.buildPlatformAnnotations(re)

			for _, key := range tt.wantKeys {
				if _, ok := annotations[key]; !ok {
					t.Errorf("Expected annotation key '%s' to be present", key)
				}
			}

			for _, key := range tt.notWantKeys {
				if _, ok := annotations[key]; ok {
					t.Errorf("Did not expect annotation key '%s' to be present", key)
				}
			}
		})
	}
}

func TestBuildPlatformAnnotations_Values(t *testing.T) {
	c := &cnbBuild{}
	re := &Request{
		BuildEnvs: map[string]string{
			"CNB_OUTPUT_DIR":   "dist",
			"CNB_BUILD_SCRIPT": "build:prod",
			"CNB_NODE_VERSION": "20.20.0",
		},
	}

	annotations := c.buildPlatformAnnotations(re)

	if annotations["cnb-bp-web-server"] != "nginx" {
		t.Errorf("Expected cnb-bp-web-server='nginx', got '%s'", annotations["cnb-bp-web-server"])
	}
	if annotations["cnb-bp-web-server-root"] != "dist" {
		t.Errorf("Expected cnb-bp-web-server-root='dist', got '%s'", annotations["cnb-bp-web-server-root"])
	}
	if annotations["cnb-bp-web-server-enable-push-state"] != "true" {
		t.Errorf("Expected cnb-bp-web-server-enable-push-state='true', got '%s'", annotations["cnb-bp-web-server-enable-push-state"])
	}
	if annotations["cnb-bp-node-run-scripts"] != "build:prod" {
		t.Errorf("Expected cnb-bp-node-run-scripts='build:prod', got '%s'", annotations["cnb-bp-node-run-scripts"])
	}
	if annotations["cnb-bp-node-version"] != "20.20.0" {
		t.Errorf("Expected cnb-bp-node-version='20.20.0', got '%s'", annotations["cnb-bp-node-version"])
	}
}

func TestInjectConfigFile_ProjectConfigExists(t *testing.T) {
	c := &cnbBuild{}

	// Create temp directory with existing .npmrc
	tmpDir := t.TempDir()
	existingContent := "registry=https://custom.registry.com"
	if err := os.WriteFile(tmpDir+"/.npmrc", []byte(existingContent), 0644); err != nil {
		t.Fatalf("Failed to create .npmrc: %v", err)
	}

	re := &Request{
		SourceDir: tmpDir,
		BuildEnvs: map[string]string{},
	}

	// Should not overwrite existing file
	err := c.injectConfigFile(re, ".npmrc", "CNB_MIRROR_NPMRC")
	if err != nil {
		t.Errorf("injectConfigFile should not error: %v", err)
	}

	// Verify file content unchanged
	content, _ := os.ReadFile(tmpDir + "/.npmrc")
	if string(content) != existingContent {
		t.Errorf("Expected file content to remain unchanged, got '%s'", string(content))
	}
}

func TestInjectConfigFile_UserProvidedContent(t *testing.T) {
	c := &cnbBuild{}

	tmpDir := t.TempDir()
	customContent := "registry=https://user-custom.registry.com"

	re := &Request{
		SourceDir: tmpDir,
		BuildEnvs: map[string]string{
			"CNB_MIRROR_NPMRC": customContent,
		},
	}

	err := c.injectConfigFile(re, ".npmrc", "CNB_MIRROR_NPMRC")
	if err != nil {
		t.Errorf("injectConfigFile should not error: %v", err)
	}

	// Verify file was created with user content
	content, err := os.ReadFile(tmpDir + "/.npmrc")
	if err != nil {
		t.Fatalf("Failed to read .npmrc: %v", err)
	}
	if string(content) != customContent {
		t.Errorf("Expected file content '%s', got '%s'", customContent, string(content))
	}
}

func TestSetSourceDirPermissions(t *testing.T) {
	c := &cnbBuild{}

	tmpDir := t.TempDir()

	// Create a .git directory
	gitDir := tmpDir + "/.git"
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	// Create a file inside .git
	if err := os.WriteFile(gitDir+"/config", []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create file in .git: %v", err)
	}

	re := &Request{
		SourceDir: tmpDir,
	}

	err := c.setSourceDirPermissions(re)
	if err != nil {
		t.Errorf("setSourceDirPermissions should not error: %v", err)
	}

	// Verify .git directory was removed
	if _, err := os.Stat(gitDir); !os.IsNotExist(err) {
		t.Error("Expected .git directory to be removed")
	}

	// Verify source directory permissions are set
	info, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatalf("Failed to stat source dir: %v", err)
	}
	if info.Mode().Perm() != 0777 {
		t.Errorf("Expected permissions 0777, got %o", info.Mode().Perm())
	}
}
