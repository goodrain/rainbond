// Copyright (C) 2014-2021 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package registry

import (
	"github.com/goodrain/rainbond/builder/sources/registry"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
)

// Cleaner is responsible for cleaning up the free images in registry.
type Cleaner struct {
	reg          *registry.Registry
	freeImageres map[string]FreeImager
}

// NewRegistryCleaner creates a new Cleaner.
func NewRegistryCleaner(url, username, password string) (*Cleaner, error) {
	reg, err := registry.NewInsecure(url, username, password)

	freeImageres := NewFreeImageres(reg)

	return &Cleaner{
		reg:          reg,
		freeImageres: freeImageres,
	}, err
}

// Cleanup cleans up the free image in the registry.
func (r *Cleaner) Cleanup() {
	logrus.Info("Start cleaning up the free images. Please be patient.")
	logrus.Info("The clean up time will be affected by the number of free images and the network environment.")

	// list images needed to be cleaned up
	freeImages := r.ListFreeImages()
	if len(freeImages) == 0 {
		logrus.Info("Free images not Found")
		return
	}
	logrus.Infof("Found %d free images", len(freeImages))

	// delete images
	r.DeleteImages(freeImages)
}

// ListFreeImages return a list of free images needed to be cleaned up.
func (r *Cleaner) ListFreeImages() []*FreeImage {
	var freeImages []*FreeImage
	for name, freeImager := range r.freeImageres {
		images, err := freeImager.List()
		if err != nil {
			logrus.Warningf("list free images for %s", name)
		}
		logrus.Infof("Found %d free images from %s", len(images), name)
		freeImages = append(freeImages, images...)
	}

	// deduplicate
	var result []*FreeImage
	m := make(map[string]struct{})
	for _, fi := range freeImages {
		fi := fi
		key := fi.Key()
		_, ok := m[key]
		if ok {
			continue
		}
		m[key] = struct{}{}
		result = append(result, fi)
	}

	return result
}

// DeleteImages deletes images.
func (r *Cleaner) DeleteImages(freeImages []*FreeImage) {
	for _, image := range freeImages {
		if err := r.deleteManifest(image.Repository, image.Digest); err != nil {
			logrus.Infof("delete manifest %s/%s: %v", image.Repository, image.Digest, err)
			continue
		}
		logrus.Infof("manifest %s/%s deleted", image.Repository, image.Digest)
	}
}

func (r *Cleaner) deleteManifest(repository, dig string) error {
	return r.reg.DeleteManifest(repository, digest.Digest(dig))
}
