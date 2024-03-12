package handler

import (
	"github.com/goodrain/rainbond/api/util"
	"github.com/sirupsen/logrus"
	"path"
	"strings"
)

// RegistryImageRepositories -
func (s *ServiceAction) RegistryImageRepositories(namespace string) ([]string, *util.APIHandleError) {
	var tenantRepositories []string
	logrus.Info(s.registryCli == nil)
	repositories, err := s.registryCli.Repositories()
	if err != nil {
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
