package store

import (
	"os"
	"strings"
)

var (
	// DEBUG_SERVICE_IDS is a comma-separated list of service IDs to debug
	// Set via environment variable: DEBUG_SERVICE_IDS=service-id-1,service-id-2
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

// ShouldDebugService checks if we should output debug logs for this service
func ShouldDebugService(serviceID string) bool {
	// If no debug service IDs configured, debug all services
	if len(debugServiceIDs) == 0 {
		return true
	}
	// Otherwise only debug configured service IDs
	return debugServiceIDs[serviceID]
}

// IsDebugMode returns true if debug mode is enabled (any service IDs configured)
func IsDebugMode() bool {
	return len(debugServiceIDs) > 0
}

// GetDebugServiceIDs returns the list of service IDs being debugged
func GetDebugServiceIDs() []string {
	ids := make([]string, 0, len(debugServiceIDs))
	for id := range debugServiceIDs {
		ids = append(ids, id)
	}
	return ids
}
