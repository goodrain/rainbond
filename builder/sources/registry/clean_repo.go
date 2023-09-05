package registry

import (
	"github.com/sirupsen/logrus"
	"sort"
)

// CleanRepo clean rbd-hub index
func (registry *Registry) CleanRepo(repository string, keep uint) error {
	tags, err := registry.Tags(repository)
	if err != nil {
		return err
	}
	sort.Strings(tags)
	logrus.Info("scan rbd-hub repository: ", repository)
	if uint(len(tags)) > keep {
		result := tags[:uint(len(tags))-keep]
		for _, tag := range result {
			registry.CleanRepoByTag(repository, tag)
		}
	}
	return nil
}

// CleanRepoByTag CleanRepoByTag
func (registry *Registry) CleanRepoByTag(repository string, tag string) error {
	dig, err := registry.ManifestDigestV2(repository, tag)
	if err != nil {
		logrus.Error("delete rbd-hub fail: ", repository)
		return err
	}
	if err := registry.DeleteManifest(repository, dig); err != nil {
		logrus.Error(err, "delete rbd-hub fail: ", repository, "; please set env REGISTRY_STORAGE_DELETE_ENABLED=true; see: https://t.goodrain.com/d/21-rbd-hub")
		return err
	}
	logrus.Info("delete rbd-hub tag: ", tag)
	return nil
}
