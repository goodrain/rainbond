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

package code

import (
	"regexp"
	"strconv"
	"strings"
)

// DefaultNodeVersion is the default Node.js version when none is specified
const DefaultNodeVersion = "20.x"

// SupportedNodeMajorVersions lists the supported major versions
var SupportedNodeMajorVersions = []int{18, 20, 22}

// NodeVersionInfo contains parsed Node.js version information
type NodeVersionInfo struct {
	Original     string // original version string from package.json
	Resolved     string // resolved version for build
	Major        int    // major version number (e.g., 20)
	Minor        int    // minor version number (e.g., 10)
	Patch        int    // patch version number (e.g., 0)
	IsRange      bool   // whether the version is a range
	Source       string // where the version came from (engines.node, default)
}

// ResolveNodeVersion resolves a Node.js version specification to a concrete version
// Supports formats:
//   - ">=18.0.0" → latest 18.x
//   - "^20.0.0"  → latest 20.x
//   - "~20.10.0" → 20.10.x
//   - "20.x"     → 20.x (direct use)
//   - "20"       → 20.x
//   - "*"        → default version
//   - ""         → default version
func ResolveNodeVersion(versionSpec string) NodeVersionInfo {
	info := NodeVersionInfo{
		Original: versionSpec,
		Source:   "engines.node",
	}

	// Handle empty or wildcard
	if versionSpec == "" || versionSpec == "*" || versionSpec == "latest" {
		info.Resolved = DefaultNodeVersion
		info.Source = "default"
		info.Major = extractMajorVersion(DefaultNodeVersion)
		return info
	}

	// Clean the version string
	cleaned := cleanVersionSpec(versionSpec)

	// Try to parse the version
	info.IsRange = isRangeVersion(versionSpec)

	// Extract major version
	major := extractMajorVersion(cleaned)
	if major > 0 {
		info.Major = major
		// Check if it's a supported version
		if !isSupportedMajorVersion(major) {
			// Fall back to closest supported version
			info.Major = findClosestSupportedVersion(major)
		}
		info.Resolved = formatMajorVersion(info.Major)
	} else {
		// Could not parse, use default
		info.Resolved = DefaultNodeVersion
		info.Source = "default"
		info.Major = extractMajorVersion(DefaultNodeVersion)
	}

	// Extract minor and patch if available
	info.Minor, info.Patch = extractMinorPatch(cleaned)

	return info
}

// cleanVersionSpec removes common prefixes and normalizes the version string
func cleanVersionSpec(version string) string {
	// Remove common prefixes
	prefixes := []string{">=", "<=", ">", "<", "^", "~", "=", "v"}
	result := strings.TrimSpace(version)

	for _, prefix := range prefixes {
		if strings.HasPrefix(result, prefix) {
			result = strings.TrimPrefix(result, prefix)
			break
		}
	}

	// Handle range with ||
	if strings.Contains(result, "||") {
		// Take the first part of the range
		parts := strings.Split(result, "||")
		result = strings.TrimSpace(parts[0])
		// Clean again in case there are prefixes
		return cleanVersionSpec(result)
	}

	// Handle range with space (e.g., ">=18.0.0 <20.0.0")
	if strings.Contains(result, " ") {
		parts := strings.Fields(result)
		result = parts[0]
		return cleanVersionSpec(result)
	}

	return strings.TrimSpace(result)
}

// isRangeVersion checks if the version specification is a range
func isRangeVersion(version string) bool {
	rangeIndicators := []string{">=", "<=", ">", "<", "^", "~", "||", " ", "*", "x", "X"}
	for _, indicator := range rangeIndicators {
		if strings.Contains(version, indicator) {
			return true
		}
	}
	return false
}

// extractMajorVersion extracts the major version number from a version string
func extractMajorVersion(version string) int {
	// Handle "x" notation (e.g., "20.x")
	version = strings.Replace(version, ".x", "", -1)
	version = strings.Replace(version, ".X", "", -1)
	version = strings.Replace(version, ".*", "", -1)

	// Try to extract the first number
	re := regexp.MustCompile(`^(\d+)`)
	matches := re.FindStringSubmatch(version)
	if len(matches) >= 2 {
		major, err := strconv.Atoi(matches[1])
		if err == nil {
			return major
		}
	}
	return 0
}

// extractMinorPatch extracts minor and patch version numbers
func extractMinorPatch(version string) (minor, patch int) {
	// Match version pattern like "20.10.5" or "20.10"
	re := regexp.MustCompile(`^(\d+)(?:\.(\d+))?(?:\.(\d+))?`)
	matches := re.FindStringSubmatch(version)

	if len(matches) >= 3 && matches[2] != "" {
		minor, _ = strconv.Atoi(matches[2])
	}
	if len(matches) >= 4 && matches[3] != "" {
		patch, _ = strconv.Atoi(matches[3])
	}

	return minor, patch
}

// isSupportedMajorVersion checks if a major version is supported
func isSupportedMajorVersion(major int) bool {
	for _, v := range SupportedNodeMajorVersions {
		if v == major {
			return true
		}
	}
	return false
}

// findClosestSupportedVersion finds the closest supported major version
func findClosestSupportedVersion(major int) int {
	// If version is too old, use the oldest supported
	if major < SupportedNodeMajorVersions[0] {
		return SupportedNodeMajorVersions[0]
	}

	// If version is too new, use the newest supported
	if major > SupportedNodeMajorVersions[len(SupportedNodeMajorVersions)-1] {
		return SupportedNodeMajorVersions[len(SupportedNodeMajorVersions)-1]
	}

	// Find the closest
	closest := SupportedNodeMajorVersions[0]
	minDiff := abs(major - closest)

	for _, v := range SupportedNodeMajorVersions {
		diff := abs(major - v)
		if diff < minDiff {
			minDiff = diff
			closest = v
		}
	}

	return closest
}

// formatMajorVersion formats a major version number to "X.x" format
func formatMajorVersion(major int) string {
	return strconv.Itoa(major) + ".x"
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ParseNodeVersionFromPackageJSON parses the Node.js version from package.json engines field
func ParseNodeVersionFromPackageJSON(buildPath string) NodeVersionInfo {
	pkgJSON := readPackageJSON(buildPath)
	if pkgJSON == nil {
		return NodeVersionInfo{
			Resolved: DefaultNodeVersion,
			Source:   "default",
			Major:    extractMajorVersion(DefaultNodeVersion),
		}
	}

	// Try to get engines.node
	if engines := pkgJSON.Get("engines"); engines != nil {
		if nodeVersion := engines.Get("node"); nodeVersion != nil {
			version, err := nodeVersion.String()
			if err == nil && version != "" {
				return ResolveNodeVersion(version)
			}
		}
	}

	// No version specified, use default
	return NodeVersionInfo{
		Resolved: DefaultNodeVersion,
		Source:   "default",
		Major:    extractMajorVersion(DefaultNodeVersion),
	}
}

// GetNodeVersionDisplay returns a user-friendly display string for the version
func (v NodeVersionInfo) GetNodeVersionDisplay() string {
	if v.Original != "" && v.Original != v.Resolved {
		return v.Resolved + " (from " + v.Original + ")"
	}
	return v.Resolved
}

// IsLTS checks if the resolved version is an LTS version
func (v NodeVersionInfo) IsLTS() bool {
	// Even major versions are LTS
	return v.Major%2 == 0
}
