package controller

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestServeDownloadedFile(t *testing.T) {
	tests := []struct {
		name         string
		fileName     string
		content      string
		rangeHeader  string
		wantStatus   int
		wantBody     string
		contentRange string
	}{
		{
			name: "index HTML does not redirect", fileName: "index.html",
			content: "<html>download</html>", wantStatus: http.StatusOK, wantBody: "<html>download</html>",
		},
		{
			name: "ordinary file", fileName: "example.txt",
			content: "download", wantStatus: http.StatusOK, wantBody: "download",
		},
		{
			name: "empty index HTML", fileName: "index.html",
			wantStatus: http.StatusOK,
		},
		{
			name: "index HTML byte range", fileName: "index.html",
			content: "download", rangeHeader: "bytes=0-3",
			wantStatus: http.StatusPartialContent, wantBody: "down", contentRange: "bytes 0-3/8",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileName := filepath.Join(t.TempDir(), tt.fileName)
			if err := os.WriteFile(fileName, []byte(tt.content), 0600); err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodGet, "/v2/file-operate/download/"+url.PathEscape(tt.fileName)+
				"?path=/usr/share/nginx/html/e%2Fxz&fileName="+url.QueryEscape(tt.fileName), nil)
			if tt.rangeHeader != "" {
				req.Header.Set("Range", tt.rangeHeader)
			}
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("status", "failed")
			if err := serveDownloadedFile(w, req, fileName); err != nil {
				t.Fatalf("serve downloaded file: %v", err)
			}
			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; Location = %q", w.Code, tt.wantStatus, w.Header().Get("Location"))
			}
			if location := w.Header().Get("Location"); location != "" {
				t.Errorf("unexpected redirect to %q", location)
			}
			if got := w.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
			for name, want := range map[string]string{
				"Content-Type":        "application/octet-stream",
				"Content-Disposition": "attachment;filename=" + tt.fileName,
				"Content-Length":      strconv.Itoa(len(tt.wantBody)),
				"Content-Range":       tt.contentRange,
				"status":              "success",
			} {
				if got := w.Header().Get(name); got != want {
					t.Errorf("%s = %q, want %q", name, got, want)
				}
			}
		})
	}
}

func TestServeDownloadedFileMissingFile(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v2/file-operate/download/index.html", nil)
	w := httptest.NewRecorder()
	w.Header().Set("status", "failed")
	err := serveDownloadedFile(w, req, filepath.Join(t.TempDir(), "index.html"))
	if !os.IsNotExist(err) {
		t.Fatalf("error = %v, want file-not-found error", err)
	}
	if w.Header().Get("status") != "failed" || w.Header().Get("Content-Disposition") != "" || w.Body.Len() != 0 {
		t.Fatal("missing file must leave the response uncommitted for the caller's error handler")
	}
}
