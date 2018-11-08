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

package v1

type Location struct {
	Path     string
	PoolName string
}

func (l *Location) Equals(c *Location) bool {
	if l == c {
		return true
	}
	if l == nil || c == nil {
		return false
	}
	if l.Path != c.Path {
		return false
	}
	if l.PoolName != c.PoolName {
		return false
	}
	return true
}

func newFakeLocation() *Location {
	return &Location{
		Path: "foo-path",
		PoolName: "foo-pool-name",
	}
}
