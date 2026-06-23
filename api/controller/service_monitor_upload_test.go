package controller

import (
	"mime/multipart"
	"net/textproto"
	"testing"
)

// capability_id: rainbond.file-operate.upload-folder-relative-path
func TestResolveUploadRelativePathPreservesFolderFromContentDisposition(t *testing.T) {
	header := &multipart.FileHeader{
		Filename: "file.txt",
		Header: textproto.MIMEHeader{
			"Content-Disposition": []string{`form-data; name="files"; filename="demo/sub/file.txt"`},
		},
	}

	relativePath, isDirectoryUpload, err := resolveUploadRelativePath(header)
	if err != nil {
		t.Fatalf("resolve upload relative path: %v", err)
	}

	if relativePath != "demo/sub/file.txt" {
		t.Fatalf("relative path = %q, want %q", relativePath, "demo/sub/file.txt")
	}
	if !isDirectoryUpload {
		t.Fatal("expected directory upload when filename contains a folder")
	}
}

// capability_id: rainbond.file-operate.upload-folder-path-traversal
func TestResolveUploadRelativePathRejectsTraversal(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{name: "parent prefix", filename: "../escape.txt"},
		{name: "parent segment", filename: "demo/../escape.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := &multipart.FileHeader{
				Filename: "escape.txt",
				Header: textproto.MIMEHeader{
					"Content-Disposition": []string{`form-data; name="files"; filename="` + tt.filename + `"`},
				},
			}

			_, _, err := resolveUploadRelativePath(header)
			if err == nil {
				t.Fatal("expected traversal path to be rejected")
			}
		})
	}
}
