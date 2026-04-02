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

package code

import (
	"os"
	"path/filepath"
	"testing"
)

// capability_id: rainbond.runtime.static-empty
func TestCheckRuntime_StaticReturnsEmptyRuntimeInfo(t *testing.T) {
	dir := t.TempDir()

	info, err := CheckRuntime(dir, Static)
	if err != nil {
		t.Fatalf("CheckRuntime() error = %v", err)
	}
	if len(info) != 0 {
		t.Fatalf("expected empty runtime info, got %+v", info)
	}
}

// capability_id: rainbond.runtime.node-defaults
func TestCheckRuntime_NodejsReturnsDefaultRuntimeInfoFromPackageJson(t *testing.T) {
	dir := t.TempDir()
	packageJSON := []byte("{\"name\":\"demo-app\"}\n")
	if err := os.WriteFile(filepath.Join(dir, "package.json"), packageJSON, 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	info, err := CheckRuntime(dir, Nodejs)
	if err != nil {
		t.Fatalf("CheckRuntime() error = %v", err)
	}
	if info["RUNTIMES"] != DefaultNodeVersion {
		t.Fatalf("RUNTIMES = %q, want %q", info["RUNTIMES"], DefaultNodeVersion)
	}
	if info["PACKAGE_TOOL"] != string(PackageManagerNPM) {
		t.Fatalf("PACKAGE_TOOL = %q, want %q", info["PACKAGE_TOOL"], PackageManagerNPM)
	}
	if info["FRAMEWORK"] != "other-static" {
		t.Fatalf("FRAMEWORK = %q, want %q", info["FRAMEWORK"], "other-static")
	}
	if info["RUNTIME_TYPE"] != "static" {
		t.Fatalf("RUNTIME_TYPE = %q, want %q", info["RUNTIME_TYPE"], "static")
	}
}

// capability_id: rainbond.runtime.node-cnb-framework-detection
func TestCheckRuntimeByStrategy_NodejsCNBDetectsFrameworkWithoutEngines(t *testing.T) {
	dir := t.TempDir()
	packageJSON := []byte(`{
		"name": "demo-next-app",
		"dependencies": {
			"next": "14.2.3",
			"react": "18.2.0"
		}
	}` + "\n")
	if err := os.WriteFile(filepath.Join(dir, "package.json"), packageJSON, 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	info, err := CheckRuntimeByStrategy(dir, Nodejs, "cnb")
	if err != nil {
		t.Fatalf("CheckRuntimeByStrategy() error = %v", err)
	}

	if info["RUNTIMES"] != MatchCNBVersion("nodejs", "") {
		t.Fatalf("RUNTIMES = %q, want %q", info["RUNTIMES"], MatchCNBVersion("nodejs", ""))
	}
	if info["PACKAGE_TOOL"] != string(PackageManagerNPM) {
		t.Fatalf("PACKAGE_TOOL = %q, want %q", info["PACKAGE_TOOL"], PackageManagerNPM)
	}
	if info["FRAMEWORK"] != "nextjs" {
		t.Fatalf("FRAMEWORK = %q, want %q", info["FRAMEWORK"], "nextjs")
	}
	if info["FRAMEWORK_DISPLAY_NAME"] != "Next.js" {
		t.Fatalf("FRAMEWORK_DISPLAY_NAME = %q, want %q", info["FRAMEWORK_DISPLAY_NAME"], "Next.js")
	}
	if info["RUNTIME_TYPE"] != "dynamic" {
		t.Fatalf("RUNTIME_TYPE = %q, want %q", info["RUNTIME_TYPE"], "dynamic")
	}
	if info["OUTPUT_DIR"] != ".next" {
		t.Fatalf("OUTPUT_DIR = %q, want %q", info["OUTPUT_DIR"], ".next")
	}
	if info["BUILD_CMD"] != "build" {
		t.Fatalf("BUILD_CMD = %q, want %q", info["BUILD_CMD"], "build")
	}
	if info["START_CMD"] != "start" {
		t.Fatalf("START_CMD = %q, want %q", info["START_CMD"], "start")
	}
}

// capability_id: rainbond.runtime.composite-nodejs
func TestCheckRuntime_CompositeNodejsLanguageUsesNodeRuntime(t *testing.T) {
	dir := t.TempDir()
	packageJSON := []byte("{\"name\":\"demo-app\",\"engines\":{\"node\":\"20.10.0\"}}\n")
	if err := os.WriteFile(filepath.Join(dir, "package.json"), packageJSON, 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	info, err := CheckRuntime(dir, Lang("dockerfile,Node.js"))
	if err != nil {
		t.Fatalf("CheckRuntime() error = %v", err)
	}
	if info["RUNTIMES"] != "20.x" {
		t.Fatalf("RUNTIMES = %q, want %q", info["RUNTIMES"], "20.x")
	}
	if info["NODE_VERSION_SOURCE"] != "engines.node" {
		t.Fatalf("NODE_VERSION_SOURCE = %q, want %q", info["NODE_VERSION_SOURCE"], "engines.node")
	}
}
