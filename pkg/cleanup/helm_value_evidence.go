package cleanup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"

	"helm.sh/helm/v3/pkg/strvals"
)

func helmValueDigest(value interface{}) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	kind := "value"
	if _, ok := value.(map[string]interface{}); ok {
		kind = "object"
	}
	return kind + ":" + hex.EncodeToString(sum[:])
}
func helmValuePath(path []string) string { raw, _ := json.Marshal(path); return string(raw) }

func indexHelmValues(config map[string]interface{}) (map[string]string, bool) {
	result := map[string]string{}
	var walk func(interface{}, []string) bool
	walk = func(value interface{}, path []string) bool {
		if len(path) > 32 || len(result) >= 16384 {
			return false
		}
		result[helmValuePath(path)] = helmValueDigest(value)
		switch values := value.(type) {
		case map[string]interface{}:
			for key, child := range values {
				if !walk(child, append(append([]string{}, path...), key)) {
					return false
				}
			}
		case []interface{}:
			for i, child := range values {
				if !walk(child, append(append([]string{}, path...), strconv.Itoa(i))) {
					return false
				}
			}
		}
		return true
	}
	return result, walk(config, []string{})
}

// HelmReleaseMatchesOverrides uses the same --set parser as Rainbond HelmApp
// installation. Only digests of actual config remain in the release evidence;
// missing keys and different values cannot be hidden by stale HelmApp status.
func HelmReleaseMatchesOverrides(release HelmReleaseIdentity, overrides []string) bool {
	if len(overrides) > 2048 || release.ValueHashes == nil {
		return false
	}
	desired := map[string]interface{}{}
	bytes := 0
	for _, value := range overrides {
		bytes += len(value)
		if bytes > 256<<10 || strvals.ParseInto(value, desired) != nil {
			return false
		}
	}
	var matches func(interface{}, []string) bool
	matches = func(value interface{}, path []string) bool {
		if len(path) > 32 {
			return false
		}
		actual, found := release.ValueHashes[helmValuePath(path)]
		if !found {
			return false
		}
		if object, ok := value.(map[string]interface{}); ok {
			if !strings.HasPrefix(actual, "object:") {
				return false
			}
			for key, child := range object {
				if !matches(child, append(append([]string{}, path...), key)) {
					return false
				}
			}
			return true
		}
		return actual == helmValueDigest(value)
	}
	return matches(desired, []string{})
}
