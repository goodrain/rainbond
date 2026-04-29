package controller

import "net/http"

const packageBuildCORSAllowedHeaders = "Content-Type, Authorization, X-Custom-Header, X-Requested-With, X-TEAM-NAME, X-REGION-NAME, X_TEAM_NAME, X_REGION_NAME"

// SetPackageBuildCORSHeaders sets CORS headers shared by package build upload APIs.
func SetPackageBuildCORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}

	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", packageBuildCORSAllowedHeaders)
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Max-Age", "3600")
}
