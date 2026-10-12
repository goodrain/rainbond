// Package registryproxy implements the coordinated Registry ingress protocol.
package registryproxy

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// ErrUnsupportedRequest rejects ambiguous or uncoordinated Registry operations.
var ErrUnsupportedRequest = errors.New("unsupported registry coordination request")

// RequestDescriptor identifies the exact repository and request target without
// retaining credentials, upload state query values, or request bodies.
type RequestDescriptor struct {
	Kind       string
	Repository string
	Target     string
	MountFrom  string
	Mutating   bool
}

var repositoryName = regexp.MustCompile(`^[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*(?:/[a-z0-9]+(?:(?:[._]|__|-+)[a-z0-9]+)*)*$`)
var digestReference = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var tagReference = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,127}$`)
var uploadReference = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._=-]{0,255}$`)

func validRepository(value string) bool {
	return len(value) <= 255 && repositoryName.MatchString(value)
}

// ClassifyRequest permits only explicitly covered Distribution v2 operations.
// Direct blob deletion is excluded: physical reclaim belongs to separate GC.
func ClassifyRequest(method string, u *url.URL) (RequestDescriptor, error) {
	reject := func() (RequestDescriptor, error) { return RequestDescriptor{}, ErrUnsupportedRequest }
	if u == nil || u.User != nil || u.Fragment != "" {
		return reject()
	}
	raw := strings.ToLower(u.EscapedPath())
	if strings.Contains(raw, "%2f") || strings.Contains(raw, "%5c") || strings.ContainsAny(u.Path, "\\\x00") {
		return reject()
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return reject()
	}
	for _, key := range []string{"digest", "mount", "from"} {
		if len(query[key]) > 1 {
			return reject()
		}
	}
	read := method == http.MethodGet || method == http.MethodHead
	if u.Path == "/v2/" || u.Path == "/v2/_catalog" {
		if !read {
			return reject()
		}
		return RequestDescriptor{Kind: "read"}, nil
	}
	if !strings.HasPrefix(u.Path, "/v2/") {
		return reject()
	}
	path := strings.TrimPrefix(u.Path, "/v2/")
	descriptor := RequestDescriptor{Mutating: !read}
	if strings.HasSuffix(path, "/tags/list") {
		descriptor.Repository = strings.TrimSuffix(path, "/tags/list")
		if !read || !validRepository(descriptor.Repository) {
			return reject()
		}
		descriptor.Kind = "read"
		return descriptor, nil
	}
	uploadIndex := strings.LastIndex(path, "/blobs/uploads/")
	manifestIndex := strings.LastIndex(path, "/manifests/")
	blobIndex := strings.LastIndex(path, "/blobs/")
	// Reserved words may also be repository path segments; match the final
	// endpoint suffix rather than an earlier occurrence inside the repository.
	if index := uploadIndex; index >= 0 && index >= manifestIndex && index >= blobIndex {
		descriptor.Repository = path[:index]
		descriptor.Target = path[index+len("/blobs/uploads/"):]
		if !validRepository(descriptor.Repository) {
			return reject()
		}
		if descriptor.Target == "" {
			if method != http.MethodPost {
				return reject()
			}
			descriptor.Kind = "upload_start"
			mount, from := query.Get("mount"), query.Get("from")
			if mount != "" || from != "" {
				if !digestReference.MatchString(mount) || !validRepository(from) {
					return reject()
				}
				descriptor.MountFrom = from
			}
			return descriptor, nil
		}
		if !uploadReference.MatchString(descriptor.Target) {
			return reject()
		}
		switch method {
		case http.MethodGet, http.MethodHead:
			descriptor.Kind = "read"
		case http.MethodPatch:
			descriptor.Kind = "upload_part"
		case http.MethodPut:
			if !digestReference.MatchString(query.Get("digest")) {
				return reject()
			}
			descriptor.Kind = "upload_complete"
		case http.MethodDelete:
			descriptor.Kind = "upload_abort"
		default:
			return reject()
		}
		return descriptor, nil
	}
	if index := manifestIndex; index >= 0 && index >= blobIndex {
		descriptor.Repository = path[:index]
		descriptor.Target = path[index+len("/manifests/"):]
		if !validRepository(descriptor.Repository) || (!digestReference.MatchString(descriptor.Target) && !tagReference.MatchString(descriptor.Target)) {
			return reject()
		}
		switch method {
		case http.MethodGet, http.MethodHead:
			descriptor.Kind = "read"
		case http.MethodPut:
			descriptor.Kind = "manifest_put"
		case http.MethodDelete:
			if !digestReference.MatchString(descriptor.Target) {
				return reject()
			}
			descriptor.Kind = "manifest_delete"
		default:
			return reject()
		}
		return descriptor, nil
	}
	if index := strings.LastIndex(path, "/blobs/"); index >= 0 {
		descriptor.Repository = path[:index]
		descriptor.Target = path[index+len("/blobs/"):]
		if !read || !validRepository(descriptor.Repository) || !digestReference.MatchString(descriptor.Target) {
			return reject()
		}
		descriptor.Kind = "read"
		return descriptor, nil
	}
	return reject()
}
