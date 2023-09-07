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
			registry.CleanRepoByTag(repository, tag, keep)
		}
	}
	return nil
}

// CleanRepoByTag clean rbd-hub index by tag
// 注意：如果这个tag的新版本镜像层依赖于老版本的镜像层，即sha256一致的情况下，并不会执行DeleteManifest方法
func (registry *Registry) CleanRepoByTag(repository string, tag string, keep uint) error {
	// 获取镜像所有的tag
	tags, err := registry.Tags(repository)
	if err != nil {
		return err
	}
	sort.Strings(tags)

	if uint(len(tags)) < keep {
		return nil
	}

	// 取出最后 n 个标签 拿到digests
	lastTags := tags[uint(len(tags))-keep:]
	digests := make([]string, len(lastTags))

	for i, tagVal := range lastTags {
		digest, err := registry.ManifestDigestV2(repository, tagVal) // 调用 registry.ManifestDigestV2 方法
		if err != nil {
			logrus.Error("Error processing tag %s: %v\n", tagVal, err)
			continue
		}
		digests[i] = digest.String() // 将结果存储到新的切片中
	}

	dig, err := registry.ManifestDigestV2(repository, tag)
	if err != nil {
		logrus.Error("get manifest fail: ", repository)
		return nil
	}
	for i, digest := range digests {
		if digest == dig.String() {
			logrus.Warnf("delete rbd-hub tag fail, but new tag %s dependents", lastTags[i])
			return nil
		}
	}
	if err := registry.DeleteManifest(repository, dig); err != nil {
		logrus.Error(err, "delete rbd-hub fail: ", repository, "; please set env REGISTRY_STORAGE_DELETE_ENABLED=true; see: https://t.goodrain.com/d/21-rbd-hub")
		return err
	}
	logrus.Info("delete rbd-hub tag success: ", tag)
	return nil
}
