package compose

import (
	"gopkg.in/yaml.v2"
)

// inferComposeVersion attempts to infer the Docker Compose version from the file content
// when no explicit version is specified. Returns "spec" for Compose Spec format,
// or a specific version string like "3.0" or "2.0".
func inferComposeVersion(body []byte) string {
	// Parse the YAML into a generic map
	var raw map[string]interface{}
	if err := yaml.Unmarshal(body, &raw); err != nil {
		// If we can't parse, default to spec (most permissive)
		return "spec"
	}

	// Check for Compose Spec features
	if hasProfiles(raw) {
		return "spec"
	}

	if hasExtends(raw) {
		return "spec"
	}

	if hasLongDependsOn(raw) {
		return "spec"
	}

	// Check for v3 features
	if hasDeploy(raw) {
		return "3.8"
	}

	// Check if it has services (v2/v3 feature)
	if _, hasServices := raw["services"]; hasServices {
		// Default to v3.0 if we have services but no v2-specific features
		return "3.0"
	}

	// If no clear indicators, default to spec (most permissive)
	return "spec"
}

// hasProfiles checks if the compose file uses profiles (Compose Spec feature)
func hasProfiles(raw map[string]interface{}) bool {
	services, ok := raw["services"].(map[interface{}]interface{})
	if !ok {
		return false
	}

	for _, svc := range services {
		svcMap, ok := svc.(map[interface{}]interface{})
		if !ok {
			continue
		}
		if _, hasProfiles := svcMap["profiles"]; hasProfiles {
			return true
		}
	}
	return false
}

// hasExtends checks if the compose file uses extends (Compose Spec feature)
func hasExtends(raw map[string]interface{}) bool {
	services, ok := raw["services"].(map[interface{}]interface{})
	if !ok {
		return false
	}

	for _, svc := range services {
		svcMap, ok := svc.(map[interface{}]interface{})
		if !ok {
			continue
		}
		if _, hasExtends := svcMap["extends"]; hasExtends {
			return true
		}
	}
	return false
}

// hasLongDependsOn checks if depends_on uses the long format (map instead of array)
// which is a Compose Spec feature
func hasLongDependsOn(raw map[string]interface{}) bool {
	services, ok := raw["services"].(map[interface{}]interface{})
	if !ok {
		return false
	}

	for _, svc := range services {
		svcMap, ok := svc.(map[interface{}]interface{})
		if !ok {
			continue
		}
		if dependsOn, hasDependsOn := svcMap["depends_on"]; hasDependsOn {
			// If depends_on is a map, it's the long format (Compose Spec)
			if _, isMap := dependsOn.(map[interface{}]interface{}); isMap {
				return true
			}
		}
	}
	return false
}

// hasDeploy checks if the compose file uses deploy section (v3 feature)
func hasDeploy(raw map[string]interface{}) bool {
	services, ok := raw["services"].(map[interface{}]interface{})
	if !ok {
		return false
	}

	for _, svc := range services {
		svcMap, ok := svc.(map[interface{}]interface{})
		if !ok {
			continue
		}
		if _, hasDeploy := svcMap["deploy"]; hasDeploy {
			return true
		}
	}
	return false
}
