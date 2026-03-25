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

func writePomFile(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pom.xml"), []byte(content), 0644); err != nil {
		t.Fatalf("write pom.xml in %s: %v", dir, err)
	}
}

func createMavenFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writePomFile(t, root, `<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>rainbond-root</artifactId>
  <packaging>pom</packaging>
  <modules>
    <module>rbd-api</module>
    <module>rbd-worker</module>
    <module>rbd-gateway</module>
  </modules>
</project>`)
	writePomFile(t, filepath.Join(root, "rbd-api"), `<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>rbd-api</artifactId>
  <packaging>jar</packaging>
</project>`)
	writePomFile(t, filepath.Join(root, "rbd-worker"), `<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>rbd-worker</artifactId>
  <packaging>jar</packaging>
</project>`)
	writePomFile(t, filepath.Join(root, "rbd-gateway"), `<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>rbd-gateway</artifactId>
  <packaging>war</packaging>
</project>`)
	return root
}

func TestMaven_ParsePom(t *testing.T) {
	root := createMavenFixture(t)
	pomPath := filepath.Join(root, "pom.xml")
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

func TestMaven_ListModules(t *testing.T) {
	path := createMavenFixture(t)
	m := maven{}
	res, err := m.ListModules(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 3 {
		t.Errorf("Expected 3 for the length of mudules, but returned %d", len(res))
	}
	for _, svc := range res {
		for _, env := range svc.Envs {
			t.Logf("Name: %s; Value: %s", env.Name, env.Value)
		}
	}
}
