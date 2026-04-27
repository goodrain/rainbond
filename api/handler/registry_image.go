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
