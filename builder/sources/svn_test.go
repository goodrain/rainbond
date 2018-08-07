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

package sources

import (
	"testing"

	"github.com/goodrain/rainbond/event"
)

func TestSvnCheckout(t *testing.T) {
	client := NewClient("", "", "svn://ali-sh-s1.goodrain.net:21097/testrepo", "/tmp/svn/testrepo", event.GetTestLogger())

	info, err := client.Checkout()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(*info)
}

func TestSvnCheckoutBranch(t *testing.T) {
	client := NewClient("", "", "https://github.com/goodrain/rainbond-install", "/tmp/svn/rainbond-install", event.GetTestLogger())

	info, err := client.Checkout()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(*info)
}
