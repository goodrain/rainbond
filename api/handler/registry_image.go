package handler

import (
	"errors"
	"github.com/goodrain/rainbond/api/util"
	sourceregistry "github.com/goodrain/rainbond/builder/sources/registry"
	"github.com/sirupsen/logrus"
	"net/http"
	"path"
	"strings"
)

// RegistryImageManifestDeleteResult is returned after deleting a registry manifest.
type RegistryImageManifestDeleteResult struct {
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Digest     string `json:"digest"`
	Deleted    bool   `json:"deleted"`
	Reason     string `json:"reason,omitempty"`
}

// RegistryImageRepositories -
func (s *ServiceAction) RegistryImageRepositories(namespace string) ([]string, *util.APIHandleError) {
	var tenantRepositories []string
	repositories, err := s.registryCli.Repositories()
	if err != nil {
		if isCatalogEnumerationUnsupported(err) {
			logrus.Warnf("registry catalog enumeration is unsupported, returning empty repository list: %v", err)
			return tenantRepositories, nil
		}
		logrus.Errorf("get tenant repositories failure: %v", err)
		return nil, util.CreateAPIHandleError(500, err)
	}
	for _, repository := range repositories {
		if strings.HasPrefix(repository, namespace+"/") {
			url := s.registryCli.URL
			urlList := strings.Split(url, "//")
			if urlList != nil && len(urlList) == 2 {
				url = urlList[1]
			}
			if url == "rbd-hub:5000" {
				url = "goodrain.me"
			}
			repository = path.Join(url, repository)
			tenantRepositories = append(tenantRepositories, repository)
		}
	}

	return tenantRepositories, nil
}

func isCatalogEnumerationUnsupported(err error) bool {
	var statusErr *sourceregistry.HttpStatusError
	if !errors.As(err, &statusErr) || statusErr.Response == nil {
		return false
	}

	switch statusErr.Response.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusMethodNotAllowed:
		return true
	default:
		return false
	}
}

// RegistryImageTags -
func (s *ServiceAction) RegistryImageTags(repository string) ([]string, *util.APIHandleError) {
	repositoryList := strings.SplitN(repository, "/", 2)
	if len(repositoryList) == 2 {
		repository = repositoryList[1]
	}
	tags, err := s.registryCli.Tags(repository)
	if err != nil {
		logrus.Errorf("get tenant repository %v tags failure: %v", repository, err)
		return nil, util.CreateAPIHandleError(500, err)
	}
	return tags, nil
}

// DeleteRegistryImageManifest deletes an internal registry image manifest by image name.
func (s *ServiceAction) DeleteRegistryImageManifest(image string) (*RegistryImageManifestDeleteResult, *util.APIHandleError) {
	if s.registryCli == nil {
		return nil, util.CreateAPIHandleErrorf(http.StatusServiceUnavailable, "registry client is not initialized")
	}
	repository, tag, err := s.parseInternalRegistryImage(image)
	if err != nil {
		return nil, err
	}

	dig, digestErr := s.registryCli.ManifestDigestV2(repository, tag)
	if digestErr != nil {
		if isRegistryManifestNotFound(digestErr) {
			return &RegistryImageManifestDeleteResult{
				Repository: repository,
				Tag:        tag,
				Deleted:    false,
				Reason:     "not_found",
			}, nil
		}
		logrus.Errorf("get registry manifest digest failure: image=%s repository=%s tag=%s err=%v", image, repository, tag, digestErr)
		return nil, util.CreateAPIHandleError(http.StatusInternalServerError, digestErr)
	}

	if deleteErr := s.registryCli.DeleteManifest(repository, dig); deleteErr != nil {
		if errors.Is(deleteErr, sourceregistry.ErrOperationIsUnsupported) {
			return nil, util.CreateAPIHandleError(http.StatusConflict, deleteErr)
		}
		if isRegistryManifestNotFound(deleteErr) {
			return &RegistryImageManifestDeleteResult{
				Repository: repository,
				Tag:        tag,
				Digest:     dig.String(),
				Deleted:    false,
				Reason:     "not_found",
			}, nil
		}
		logrus.Errorf("delete registry manifest failure: image=%s repository=%s tag=%s digest=%s err=%v", image, repository, tag, dig, deleteErr)
		return nil, util.CreateAPIHandleError(http.StatusInternalServerError, deleteErr)
	}

	return &RegistryImageManifestDeleteResult{
		Repository: repository,
		Tag:        tag,
		Digest:     dig.String(),
		Deleted:    true,
	}, nil
}

func (s *ServiceAction) parseInternalRegistryImage(image string) (string, string, *util.APIHandleError) {
	value := strings.TrimSpace(image)
	if value == "" {
		return "", "", util.CreateAPIHandleErrorf(http.StatusBadRequest, "image is empty")
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "/") {
		return "", "", util.CreateAPIHandleErrorf(http.StatusBadRequest, "image is not an internal registry image")
	}

	lastSlash := strings.LastIndex(value, "/")
	lastColon := strings.LastIndex(value, ":")
	if lastColon <= lastSlash || lastColon == len(value)-1 {
		return "", "", util.CreateAPIHandleErrorf(http.StatusBadRequest, "image must include a tag")
	}

	name := value[:lastColon]
	tag := value[lastColon+1:]
	repository := name
	host := ""
	if slash := strings.Index(name, "/"); slash > -1 {
		candidateHost := name[:slash]
		if isRegistryHostSegment(candidateHost) {
			host = candidateHost
			repository = name[slash+1:]
		}
	}
	if repository == "" || tag == "" {
		return "", "", util.CreateAPIHandleErrorf(http.StatusBadRequest, "image must include repository and tag")
	}
	if host != "" && !s.isInternalRegistryHost(host) {
		return "", "", util.CreateAPIHandleErrorf(http.StatusBadRequest, "image is not in the internal registry")
	}
	return repository, tag, nil
}

func isRegistryHostSegment(value string) bool {
	return value == "localhost" || strings.Contains(value, ".") || strings.Contains(value, ":")
}

func (s *ServiceAction) isInternalRegistryHost(host string) bool {
	normalizedHost := normalizeRegistryHost(host)
	if normalizedHost == "goodrain.me" {
		return true
	}
	registryHost := ""
	if s.registryCli != nil {
		registryHost = normalizeRegistryHost(s.registryCli.URL)
	}
	if registryHost == "" {
		return false
	}
	if normalizedHost == registryHost {
		return true
	}
	return (normalizedHost == "goodrain.me" && registryHost == "rbd-hub:5000") ||
		(normalizedHost == "rbd-hub:5000" && registryHost == "goodrain.me")
}

func normalizeRegistryHost(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	normalized = strings.TrimPrefix(normalized, "http://")
	normalized = strings.TrimPrefix(normalized, "https://")
	normalized = strings.TrimRight(normalized, "/")
	if slash := strings.Index(normalized, "/"); slash > -1 {
		normalized = normalized[:slash]
	}
	return normalized
}

func isRegistryManifestNotFound(err error) bool {
	if errors.Is(err, sourceregistry.ErrManifestNotFound) {
		return true
	}
	var statusErr *sourceregistry.HttpStatusError
	if errors.As(err, &statusErr) && statusErr.Response != nil && statusErr.Response.StatusCode == http.StatusNotFound {
		return true
	}
	return strings.Contains(err.Error(), "status=404")
}
