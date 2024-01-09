package interceptors

import (
	"fmt"
	"github.com/go-chi/chi/middleware"
	"github.com/goodrain/rainbond/pkg/component/etcd"
	"net/http"
	"runtime/debug"
	"strings"
)

func Recoverer(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil && rvr != http.ErrAbortHandler {
				// Check if the panic is a nil pointer exception
				if isNilPointerException(rvr) && etcd.Default().EtcdClient == nil {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}

				// Handle other types of panics or re-panic
				logEntry := middleware.GetLogEntry(r)
				if logEntry != nil {
					logEntry.Panic(rvr, debug.Stack())
				} else {
					middleware.PrintPrettyStack(rvr)
				}

				w.WriteHeader(http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

// Check if the panic is a nil pointer exception
func isNilPointerException(p interface{}) bool {
	if p == nil {
		return false
	}

	errMsg := fmt.Sprintf("%v", p)
	return strings.Contains(errMsg, "runtime error: invalid memory address or nil pointer dereference")
}
