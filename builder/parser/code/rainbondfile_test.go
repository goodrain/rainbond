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

// capability_id: rainbond.rainbondfile.read-project-root
func TestReadRainbondFile(t *testing.T) {
	rbdfile, err := ReadRainbondFile("./")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(rbdfile)
}

// capability_id: rainbond.rainbondfile.missing
func TestReadRainbondFile_ReturnsNotFoundWhenMissing(t *testing.T) {
	dir := t.TempDir()

	_, err := ReadRainbondFile(dir)

	if err != ErrRainbondFileNotFound {
		t.Fatalf("expected ErrRainbondFileNotFound, got %v", err)
	}
}

// capability_id: rainbond.rainbondfile.parse
func TestReadRainbondFile_ParsesYamlConfig(t *testing.T) {
	dir := t.TempDir()
	content := []byte("language: Node.js\nbuildpath: web\ncmd: npm start\nports:\n- port: 80\n  protocol: http\n")
	if err := os.WriteFile(filepath.Join(dir, "rainbondfile"), content, 0o644); err != nil {
		t.Fatalf("write rainbondfile: %v", err)
	}

	rbdfile, err := ReadRainbondFile(dir)
	if err != nil {
		t.Fatalf("ReadRainbondFile() error = %v", err)
	}
	if rbdfile.Language != "Node.js" {
		t.Fatalf("Language = %q, want %q", rbdfile.Language, "Node.js")
	}
	if rbdfile.BuildPath != "web" {
		t.Fatalf("BuildPath = %q, want %q", rbdfile.BuildPath, "web")
	}
	if rbdfile.Cmd != "npm start" {
		t.Fatalf("Cmd = %q, want %q", rbdfile.Cmd, "npm start")
	}
	if len(rbdfile.Ports) != 1 || rbdfile.Ports[0].Port != 80 || rbdfile.Ports[0].Protocol != "http" {
		t.Fatalf("Ports parsed incorrectly: %+v", rbdfile.Ports)
	}
}
