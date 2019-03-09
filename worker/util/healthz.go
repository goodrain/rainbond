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
	"fmt"
	"strings"
)

// GenServiceName returns the serviceName consisting of service id and ip address.
func GenServiceName(sid, ip string) string {
	return sid + "/" + strings.Replace(ip, ".", "", -1)
}

// GetServiceID separates the service id from the service name
func GetServiceID(name string) (string, error) {
	slc := strings.Split(name, "/")
	if len(slc) != 2 {
		return "", fmt.Errorf("ServiceName: %s; Invalid serivce name", name)
	}
	return slc[0], nil
}
