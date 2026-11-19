package version2

import (
	"net/http"
	"path"
	"strings"
)

func aiEngineManagementPath(r *http.Request, target string) bool {
	const prefix = "/api/v1/ai-engine"
	if target != prefix && !strings.HasPrefix(target, prefix+"/") {
		return false
	}
	// Match Engine's existing capability routes exactly. Its handlers still
	// validate the signed media URL or opaque upload ID themselves.
	if r.Method != http.MethodGet || r.URL.Path != path.Clean(r.URL.Path) ||
		r.URL.EscapedPath() != r.URL.Path || strings.Contains(target, "\\") {
		return true
	}
	for _, assetPrefix := range []string{prefix + "/media/media-", prefix + "/uploads/"} {
		if id := strings.TrimPrefix(target, assetPrefix); id != target {
			if len(id) != 32 {
				return true
			}
			for _, c := range id {
				if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
					return true
				}
			}
			return false
		}
	}
	return true
}
