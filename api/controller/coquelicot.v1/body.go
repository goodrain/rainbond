package coquelicot

import (
	"bufio"
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
func newBody(xfile string, req_body io.Reader) (*body, error) {
	if xfile == "" {
		return &body{body: req_body, Available: true}, nil
	}

	fh, err := os.Open(xfile)
	if err != nil {
		return nil, err
	}

	return &body{XFile: fh, body: bufio.NewReader(fh), Available: true}, nil
}

// Close filehandler of body if XFile exists.
func (body *body) Close() error {
	if body.XFile != nil {
		return body.XFile.Close()
	}

	return nil
}
