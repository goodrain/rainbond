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

// cnbJavaVersions defines the supported Java major versions for CNB builds.
var cnbJavaVersions = []CNBVersion{
	{Version: "8", Default: false},
	{Version: "11", Default: false},
	{Version: "17", Default: true},
	{Version: "21", Default: false},
	{Version: "25", Default: false},
}

// cnbGolangVersions defines the supported Go major.minor versions for CNB builds.
var cnbGolangVersions = []CNBVersion{
	{Version: "1.24", Default: false},
	{Version: "1.25", Default: true},
}

// cnbPythonVersions defines the supported Python major.minor versions for CNB builds.
var cnbPythonVersions = []CNBVersion{
	{Version: "3.10", Default: false},
	{Version: "3.11", Default: false},
	{Version: "3.12", Default: false},
	{Version: "3.13", Default: false},
	{Version: "3.14", Default: true},
}

// GetCNBVersions returns the supported CNB versions for a given language.
// Supports composite languages like "dockerfile,Node.js" by checking each part.
func GetCNBVersions(lang string) []CNBVersion {
	lower := strings.ToLower(lang)
	for _, part := range strings.Split(lower, ",") {
		switch strings.TrimSpace(part) {
		case "nodejs", "node", "node.js":
			return cnbNodeVersions
		case "java", "openjdk", "java-maven", "java-war", "java-jar", "gradle", "java-gradle", "javagradle":
			return cnbJavaVersions
		case "go", "golang":
			return cnbGolangVersions
		case "python":
			return cnbPythonVersions
		}
	}
	return []CNBVersion{}
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

	if normalized, ok := normalizeCNBVersionSpec(lang, versionSpec); ok {
		for _, v := range versions {
			if v.Version == normalized {
				return v.Version
			}
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

func normalizeCNBVersionSpec(lang, spec string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "python":
		normalized, err := normalizePythonRuntimeVersion(trimVersionSpecPrefixes(spec))
		return normalized, err == nil
	case "go", "golang":
		normalized, err := normalizeGolangRuntimeVersion(trimVersionSpecPrefixes(spec))
		return normalized, err == nil
	default:
		return "", false
	}
}

// extractMajorFromSpec extracts the major version number from a version spec.
func extractMajorFromSpec(spec string) int {
	s := trimVersionSpecPrefixes(spec)
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

func trimVersionSpecPrefixes(spec string) string {
	s := strings.TrimSpace(spec)
	for _, prefix := range []string{">=", "<=", ">", "<", "^", "~", "=", "v"} {
		s = strings.TrimPrefix(s, prefix)
	}
	return s
}
