package upload

import (
	"io"
	"mime/multipart"
	"os"
)

// Upload body info.
type body struct {
	XFile     *os.File
	body      io.Reader
	MR        *multipart.Reader
	Available bool
}

// Check exists body in xfile and return body.
func newBody(req_body io.Reader) (*body, error) {
		return &body{body: req_body, Available: true}, nil
}

// Close filehandler of body if XFile exists.
func (body *body) Close() error {
	if body.XFile != nil {
		return body.XFile.Close()
	}

	return nil
}
