// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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

package exporter

import "fmt"

type matchType string

const (
	matchTypeGlob    matchType = "glob"
	matchTypeRegex   matchType = "regex"
	matchTypeDefault matchType = ""
)

func (t *matchType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var v string
	if err := unmarshal(&v); err != nil {
		return err
	}

	switch matchType(v) {
	case matchTypeRegex:
		*t = matchTypeRegex
	case matchTypeGlob, matchTypeDefault:
		*t = matchTypeGlob
	default:
		return fmt.Errorf("invalid match type %q", v)
	}
	return nil
}
