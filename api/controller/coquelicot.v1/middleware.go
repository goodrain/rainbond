package coquelicot

import (
	"log"
	"net/http"
)

type Adapter func(http.Handler) http.Handler

func Adapt(h http.Handler, adapters ...Adapter) http.Handler {
	for k := len(adapters) - 1; k >= 0; k-- {
		h = adapters[k](h)
	}
	return h
}

func CORSMiddleware() Adapter {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, PATCH, DELETE")
			w.Header().Set("Access-Control-Allow-Headers",
				"Content-Type, Content-Length, Accept-Encoding, Content-Range, Content-Disposition, Authorization")
			// Since we need to support cross-domain cookies, we must support XHR requests
			// with credentials, so the Access-Control-Allow-Credentials header is required
			// and Access-Control-Allow-Origin cannot be equal to "*" but reply with the same Origin.
			// See https://developer.mozilla.org/en-US/docs/Web/HTTP/Access_control_CORS.
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))

			if r.Method == "OPTIONS" {
				return
			}

			h.ServeHTTP(w, r)
		})
	}
}

func LogMiddleware(logger *log.Logger) Adapter {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r)
			path := r.URL.Path
			if len(r.URL.RawQuery) > 0 {
				path += "?" + r.URL.RawQuery
			}
			logger.Printf("%s %s [%s]\n", r.Method, path, r.RemoteAddr)
		})
	}
}
