// RAINBOND, Application Management Platform
// Copyright (C) 2020-2020 Goodrain Co., Ltd.

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

package model

import (
	"sort"
	"testing"
)

func TestTenantList(t *testing.T) {
	var tenants TenantList
	t1 := &TenantAndResource{
		MemoryRequest: 100,
	}
	t1.LimitMemory = 30
	tenants.Add(t1)

	t2 := &TenantAndResource{
		MemoryRequest: 80,
	}
	t2.LimitMemory = 40
	tenants.Add(t2)

	t3 := &TenantAndResource{
		MemoryRequest: 0,
	}
	t3.LimitMemory = 60
	t4 := &TenantAndResource{
		MemoryRequest: 0,
	}
	t4.LimitMemory = 70

	t5 := &TenantAndResource{
		RunningAppNum: 10,
	}
	t5.LimitMemory = 0

	tenants.Add(t3)
	tenants.Add(t4)
	tenants.Add(t5)
	sort.Sort(tenants)
	for _, ten := range tenants {
		t.Logf("%+v", ten)
	}
}
