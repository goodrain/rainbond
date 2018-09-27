package coquelicot

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
)

// Info about request headers
type meta struct {
	MediaType string
	Boundary  string
	Range     *dataRange
	Filename  string
	UploadSid string
}

type dataRange struct {
	Start int64
	End   int64
	Size  int64
}

// Parse request headers and make Meta.
func parseMeta(req *http.Request) (*meta, error) {
	m := &meta{}

	if err := m.parseContentType(req.Header.Get("Content-Type")); err != nil {
		return nil, err
	}

	if err := m.parseContentRange(req.Header.Get("Content-Range")); err != nil {
		return nil, err
	}

	if err := m.parseContentDisposition(req.Header.Get("Content-Disposition")); err != nil {
		return nil, err
	}

	cookie, err := req.Cookie("coquelicot")
	if err != nil {
		return nil, err
	}
	if cookie != nil {
		m.UploadSid = cookie.Value
	}

	return m, nil
}

func (m *meta) parseContentType(ct string) error {
	if ct == "" {
		m.MediaType = "application/octet-stream"
		return nil
	}

	mediatype, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return err
	}

	if mediatype == "multipart/form-data" {
		boundary, ok := params["boundary"]
		if !ok {
			return errors.New("meta: boundary not defined")
		}

		m.MediaType = mediatype
		m.Boundary = boundary
	} else {
		m.MediaType = "application/octet-stream"
	}

	return nil
}

func (m *meta) parseContentRange(cr string) error {
	if cr == "" {
		return nil
	}

	var start, end, size int64

	_, err := fmt.Sscanf(cr, "bytes %d-%d/%d", &start, &end, &size)
	if err != nil {
		return err
	}

	m.Range = &dataRange{Start: start, End: end, Size: size}

	return nil
}

func (m *meta) parseContentDisposition(cd string) error {
	if cd == "" {
		return nil
	}

	_, params, err := mime.ParseMediaType(cd)
	if err != nil {
		return err
	}

	filename, ok := params["filename"]
	if !ok {
		return errors.New("meta: filename in Content-Disposition not defined")
	}

	m.Filename = filename

	return nil
}
