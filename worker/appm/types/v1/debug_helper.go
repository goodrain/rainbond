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

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

var debugTargetServiceID string

func init() {
	// DEBUG_TARGET_SERVICE_ID 用于目标组件的详细调试日志
	if targetID := os.Getenv("DEBUG_TARGET_SERVICE_ID"); targetID != "" {
		debugTargetServiceID = strings.TrimSpace(targetID)
		logrus.Infof("[DebugHelper-v1] Target service debug mode enabled for: %s", debugTargetServiceID)
	}
}

// IsTargetService returns true if this is the target service for detailed debugging
// Controlled by DEBUG_TARGET_SERVICE_ID environment variable
func IsTargetService(serviceID string) bool {
	return debugTargetServiceID != "" && serviceID == debugTargetServiceID
}
