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

package utils

import (
	"net/url"
	"strconv"
	"strings"

	validation "github.com/goodrain/rainbond/util/endpoint"
)

// FilterEndpointKey filter endpoint key
// key consist of end.name + hostIP, so it's len must be 2
// key := end.Name + "/" + hostIP
func FilterEndpointKey(key string) bool {
	subKeys := strings.Split(key, "/")
	if len(subKeys) != 2 {
		return false
	}

	if strings.TrimSpace(subKeys[1]) == "" {
		return false
	}

	return true
}

// FilterEndpoint filter wrong endpoint
func FilterEndpoint(endpoint string) bool {
	s := strings.Split(endpoint, ":")
	if len(s) == 3 { // contain protocol
		u, err := url.Parse(endpoint)
		if err != nil {
			return false
		}

		if !checkIP(u.Hostname()) {
			return false
		}
		if !checkPort(u.Port()) {
			return false
		}
	} else if len(s) == 2 { // do not contain protocol
		if !checkIP(s[0]) {
			return false
		}
		if !checkPort(s[1]) {
			return false
		}
	} else {
		return false
	}
	return true
}

func checkIP(str string) bool {
	if strings.TrimSpace(str) == "" {
		return false
	}
	if errs := validation.ValidateEndpointIP(str); len(errs) > 0 {
		return false
	}
	return true
}

func checkPort(str string) bool {
	if _, err := strconv.Atoi(str); err != nil {
		return false
	}
	return true
}
