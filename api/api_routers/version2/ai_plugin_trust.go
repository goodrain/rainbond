package version2

import (
	"net/http"
	"strings"
)

func aiEngineManagementPath(method, path string) bool {
	const prefix = "/api/v1/ai-engine"
	if path != prefix && !strings.HasPrefix(path, prefix+"/") {
		return false
	}
	if method == http.MethodGet && strings.HasPrefix(path, prefix+"/media/") {
		id := strings.TrimPrefix(path, prefix+"/media/")
		// Only the existing single-object GET route is authenticated by its
		// media signature; uploads, deletes and other paths remain protected.
		if id != "" && id != "." && id != ".." && !strings.ContainsAny(id, "/\\") {
			return false
		}
	}
	return true
}
