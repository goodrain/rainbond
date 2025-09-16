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

package util

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

//NewUUID 创建无-的32位uuid
func NewUUID() string {
	uid := uuid.New().String()
	return strings.Replace(uid, "-", "", -1)
}

//TimeVersion time version
type TimeVersion string

//NewTimeVersion return version string by time
func NewTimeVersion() TimeVersion {
	now := time.Now()
	return TimeVersion(fmt.Sprintf("%d", now.UnixNano()))
}
