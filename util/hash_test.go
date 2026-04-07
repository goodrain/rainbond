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

package util

import (
	"os"
	"path/filepath"
	"testing"
)

// capability_id: rainbond.util.core-helpers.hash-ip-string-uuid
func TestCreateFileHash(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "hashtest")
	target := filepath.Join(root, "hashtest.md5")

	if err := os.WriteFile(source, []byte("rainbond-hash-test"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := CreateFileHash(source, target); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 {
		t.Fatal("expected md5 file content")
	}
}

// capability_id: rainbond.util.core-helpers.hash-string
func TestCreateHashString(t *testing.T) {
	hash, err := CreateHashString("rainbond")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "b4db463c7a30007ad5c6e5b42440893d" {
		t.Fatalf("unexpected hash: %q", hash)
	}
}
