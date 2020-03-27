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

package main

import "testing"

func TestHaveChange(t *testing.T) {
	old := make(map[string]string)
	t.Log(haveChange(map[string]string{"console": "127.0.0.1"}, old))
	t.Log(haveChange(map[string]string{"console": "127.0.0.1"}, map[string]string{"console": "127.0.0.1"}))
	t.Log(haveChange(map[string]string{"console": "127.0.0.2"}, map[string]string{"console": "127.0.0.1"}))
	t.Log(haveChange(map[string]string{"console": "127.0.0.2", "console1": "127.0.0.1"}, map[string]string{"console": "127.0.0.1"}))
	t.Log(haveChange(map[string]string{"console": "127.0.0.2"}, map[string]string{"console2": "127.0.0.1", "console1": "127.0.0.1"}))
}
