package code

import (
	"strconv"
	"strings"
)

// CNBVersion represents a CNB build supported version
type CNBVersion struct {
	Version string `json:"version"`
	Default bool   `json:"default"`
}

// cnbNodeVersions defines the supported Node.js versions for CNB builds
var cnbNodeVersions = []CNBVersion{
	{Version: "18.20.7", Default: false},
	{Version: "18.20.8", Default: false},
	{Version: "20.19.6", Default: false},
	{Version: "20.20.0", Default: false},
	{Version: "22.21.1", Default: false},
	{Version: "22.22.0", Default: false},
	{Version: "24.12.0", Default: false},
	{Version: "24.13.0", Default: true},
}

// GetCNBVersions returns the supported CNB versions for a given language.
func GetCNBVersions(lang string) []CNBVersion {
	switch strings.ToLower(lang) {
	case "nodejs", "node", "node.js":
		return cnbNodeVersions
	default:
		return []CNBVersion{}
	}
}

// MatchCNBVersion resolves a fuzzy version spec (e.g. "20.x", ">=20.0", "20")
// to the best matching exact version from the CNB version list.
// Returns empty string if no match found.
func MatchCNBVersion(lang, versionSpec string) string {
	versions := GetCNBVersions(lang)
	if len(versions) == 0 {
		return ""
	}

	// Find default version as fallback
	defaultVer := ""
	for _, v := range versions {
		if v.Default {
			defaultVer = v.Version
			break
		}
	}
	if defaultVer == "" {
		defaultVer = versions[len(versions)-1].Version
	}

	if versionSpec == "" {
		return defaultVer
	}

	// Exact match
	for _, v := range versions {
		if v.Version == versionSpec {
			return v.Version
		}
	}

	// Extract major version from spec like "20.x", ">=20.0", "^20", "~20.10"
	major := extractMajorFromSpec(versionSpec)
	if major <= 0 {
		return defaultVer
	}

	// Find the latest version matching this major
	var matched string
	for _, v := range versions {
		parts := strings.SplitN(v.Version, ".", 2)
		if len(parts) > 0 {
			if m, err := strconv.Atoi(parts[0]); err == nil && m == major {
				matched = v.Version // keep updating, last one is the latest
			}
		}
	}
	if matched != "" {
		return matched
	}

	return defaultVer
}

// extractMajorFromSpec extracts the major version number from a version spec.
func extractMajorFromSpec(spec string) int {
	// Strip common prefixes
	s := strings.TrimSpace(spec)
	for _, prefix := range []string{">=", "<=", ">", "<", "^", "~", "=", "v"} {
		s = strings.TrimPrefix(s, prefix)
	}
	// Take first segment before "."
	if idx := strings.Index(s, "."); idx > 0 {
		s = s[:idx]
	}
	m, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return m
}
