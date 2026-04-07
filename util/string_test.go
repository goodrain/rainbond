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

import "testing"

// capability_id: rainbond.util.core-helpers.string-contains
func TestStringArrayContains(t *testing.T) {
	if !StringArrayContains([]string{"a", "b", "c"}, "b") {
		t.Fatal("expected list to contain b")
	}
	if StringArrayContains([]string{"a", "b", "c"}, "z") {
		t.Fatal("did not expect list to contain z")
	}
	if StringArrayContains(nil, "a") {
		t.Fatal("did not expect nil list to contain anything")
	}
}

// capability_id: rainbond.util.core-helpers.hash-ip-string-uuid
func TestReverse(t *testing.T) {
	tests := []struct {
		input []string
		want  []string
	}{
		{input: []string{"1", "2", "3"}, want: []string{"3", "2", "1"}},
		{input: []string{"1", "2", "3", "4"}, want: []string{"4", "3", "2", "1"}},
	}

	for _, tt := range tests {
		got := Reverse(tt.input)
		if len(got) != len(tt.want) {
			t.Fatalf("unexpected result length: got %d want %d", len(got), len(tt.want))
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Fatalf("Reverse(%v)=%v, want %v", tt.input, got, tt.want)
			}
		}
	}
}
