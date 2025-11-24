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

package store

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

var debugServiceIDs map[string]bool

func init() {
	debugServiceIDs = make(map[string]bool)
	if envIDs := os.Getenv("DEBUG_SERVICE_IDS"); envIDs != "" {
		ids := strings.Split(envIDs, ",")
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id != "" {
				debugServiceIDs[id] = true
			}
		}
		logrus.Infof("[DebugHelper] Debug mode enabled for service IDs: %v", getDebugServiceIDList())
	}
}

func getDebugServiceIDList() []string {
	ids := make([]string, 0, len(debugServiceIDs))
	for id := range debugServiceIDs {
		ids = append(ids, id)
	}
	return ids
}

// ShouldDebugService returns true if this service should output debug logs
// If DEBUG_SERVICE_IDS is not set, returns true for all services (default behavior)
// If DEBUG_SERVICE_IDS is set, only returns true for services in the list
func ShouldDebugService(serviceID string) bool {
	if len(debugServiceIDs) == 0 {
		return true
	}
	return debugServiceIDs[serviceID]
}
