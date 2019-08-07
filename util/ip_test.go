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

package util

import "testing"

func TestCheckIP(t *testing.T) {
	t.Logf("ip %s %t", "1829.123", CheckIP("1829.123"))
	t.Logf("ip %s %t", "1829.123.1", CheckIP("1829.123.1"))
	t.Logf("ip %s %t", "1829.123.2.1", CheckIP("1829.123.2.1"))
	t.Logf("ip %s %t", "y.123", CheckIP("y.123"))
	t.Logf("ip %s %t", "0.0.0.0", CheckIP("0.0.0.0"))
	t.Logf("ip %s %t", "127.0.0.1", CheckIP("127.0.0.1"))
	t.Logf("ip %s %t", "localhost", CheckIP("localhost"))
	t.Logf("ip %s %t", "192.168.0.1", CheckIP("192.168.0.1"))
}
