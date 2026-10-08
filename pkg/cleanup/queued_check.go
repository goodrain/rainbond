package cleanup

import (
	"encoding/json"
	"strings"
)

// PackageCheckIdentity selects only source checks which may consume uploaded data.
// API and builder share this classifier so a queue reservation is never orphaned
// merely because its consumer took an unprotected dispatch branch.
func PackageCheckIdentity(body []byte) (string, bool, error) {
	var input struct {
		ID      string `json:"uuid"`
		Type    string `json:"source_type"`
		Source  string `json:"source_body"`
		EventID string `json:"event_id"`
	}
	if len(body) > 1<<20 || json.Unmarshal(body, &input) != nil || !coordinationIdentity.MatchString(input.ID) {
		return "", false, ErrCoordinationChanged
	}
	needs := false
	switch input.Type {
	case "package_build":
		needs = true
	case "sourcecode":
		var source struct {
			ServerType string `json:"server_type"`
		}
		if json.Unmarshal([]byte(input.Source), &source) == nil {
			needs = source.ServerType == "pkg"
		}
	case "docker-compose":
		needs = input.EventID != ""
	case "docker-run":
		parts := strings.Fields(input.Source)
		if len(parts) == 2 && parts[0] == "event" {
			needs = true
		}
	}
	return input.ID, needs, nil
}
