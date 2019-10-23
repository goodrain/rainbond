// RAINBOND, Application Management Platform
// Copyright (C) 2014-2019 Goodrain Co., Ltd.

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

package handler

import "testing"

func TestSelectAvailablePort(t *testing.T) {
	t.Log(selectAvailablePort([]int{9000}))         // less than minport
	t.Log(selectAvailablePort([]int{10000}))        // equal to minport
	t.Log(selectAvailablePort([]int{10003, 10001})) // more than minport and less than maxport
	t.Log(selectAvailablePort([]int{65535}))        // equal to maxport
	t.Log(selectAvailablePort([]int{10000, 65536})) // more than maxport
}
