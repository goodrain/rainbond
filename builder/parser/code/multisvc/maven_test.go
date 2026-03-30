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

package multi

import (
	"os"
	"path/filepath"
	"testing"
)

// capability_id: rainbond.maven.parse-pom
func TestMaven_ParsePom(t *testing.T) {
	tmpDir := t.TempDir()
	pomPath := filepath.Join(tmpDir, "pom.xml")
	content := `<?xml version="1.0" encoding="UTF-8"?>
<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>demo-parent</artifactId>
  <version>1.0.0</version>
  <packaging>pom</packaging>
  <modules>
    <module>rbd-api</module>
    <module>rbd-worker</module>
    <module>rbd-gateway</module>
  </modules>
</project>`
	if err := os.WriteFile(pomPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write pom.xml: %v", err)
	}
	pom, err := parsePom(pomPath)
	if err != nil {
		t.Fatal(err)
	}
	if pom.Packaging != "pom" {
		t.Errorf("Expected pom for pom.Packaging, but returned %s", pom.Packaging)
	}
	if pom.Modules == nil || len(pom.Modules) != 3 {
		t.Error("Modules not found")
	} else {
		if pom.Modules[0] != "rbd-api" {
			t.Errorf("Expected 'rbd-api' for pom.Modules[0], but returned %s", pom.Modules[0])
		}
		if pom.Modules[1] != "rbd-worker" {
			t.Errorf("Expected 'rbd-worker' for pom.Modules[0], but returned %s", pom.Modules[0])
		}
		if pom.Modules[2] != "rbd-gateway" {
			t.Errorf("Expected 'rbd-gateway' for pom.Modules[0], but returned %s", pom.Modules[0])
		}
	}
}

// capability_id: rainbond.maven.list-modules
func TestMaven_ListModules(t *testing.T) {
	root := t.TempDir()
	rootPom := `<?xml version="1.0" encoding="UTF-8"?>
<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>demo-parent</artifactId>
  <version>1.0.0</version>
  <packaging>pom</packaging>
  <modules>
    <module>rbd-api</module>
    <module>rbd-worker</module>
    <module>rbd-gateway</module>
  </modules>
</project>`
	if err := os.WriteFile(filepath.Join(root, "pom.xml"), []byte(rootPom), 0o644); err != nil {
		t.Fatalf("write root pom.xml: %v", err)
	}
	modules := map[string]string{
		"rbd-api": `<?xml version="1.0" encoding="UTF-8"?>
<project><modelVersion>4.0.0</modelVersion><groupId>com.example</groupId><artifactId>rbd-api</artifactId><version>1.0.0</version><packaging>jar</packaging></project>`,
		"rbd-worker": `<?xml version="1.0" encoding="UTF-8"?>
<project><modelVersion>4.0.0</modelVersion><groupId>com.example</groupId><artifactId>rbd-worker</artifactId><version>1.0.0</version><packaging>jar</packaging></project>`,
		"rbd-gateway": `<?xml version="1.0" encoding="UTF-8"?>
<project><modelVersion>4.0.0</modelVersion><groupId>com.example</groupId><artifactId>rbd-gateway</artifactId><version>1.0.0</version><packaging>war</packaging></project>`,
	}
	for name, content := range modules {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(content), 0o644); err != nil {
			t.Fatalf("write module pom.xml: %v", err)
		}
	}

	m := maven{}
	res, err := m.ListModules(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 3 {
		t.Fatalf("Expected 3 modules, but returned %d", len(res))
	}
	names := []string{res[0].Name, res[1].Name, res[2].Name}
	expected := map[string]string{
		"rbd-api":     "jar",
		"rbd-worker":  "jar",
		"rbd-gateway": "war",
	}
	for _, svc := range res {
		packaging, ok := expected[svc.Name]
		if !ok {
			t.Fatalf("unexpected module name %q in %v", svc.Name, names)
		}
		if svc.Packaging != packaging {
			t.Fatalf("module %s packaging = %q, want %q", svc.Name, svc.Packaging, packaging)
		}
		if svc.Envs["BUILD_MAVEN_CUSTOM_GOALS"] == nil {
			t.Fatalf("module %s missing BUILD_MAVEN_CUSTOM_GOALS", svc.Name)
		}
		if svc.Packaging == "war" && svc.Envs["BUILD_PROCFILE"] != nil && svc.Envs["BUILD_PROCFILE"].Value == "" {
			t.Fatalf("module %s missing BUILD_PROCFILE value", svc.Name)
		}
	}
}
