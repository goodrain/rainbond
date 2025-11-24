package conversion

import (
	"os"
	"strings"
)

var (
	debugServiceIDs map[string]bool
)

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
	}
}

// shouldDebugServiceID checks if we should output debug logs for this service
func shouldDebugServiceID(serviceID string) bool {
	// If no debug service IDs configured, debug all services
	if len(debugServiceIDs) == 0 {
		return true
	}
	// Otherwise only debug configured service IDs
	return debugServiceIDs[serviceID]
}
