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
	"errors"

	"github.com/goodrain/rainbond/builder/sources/registry"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/util"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

var _ FreeImager = &FreeVersion{}

// FreeVersion is resposible for listing the free images belong to free component versions.
type FreeVersion struct {
	reg *registry.Registry
}

// NewFreeVersion creates a free version.
func NewFreeVersion(reg *registry.Registry) *FreeVersion {
	return &FreeVersion{
		reg: reg,
	}
}

// List return a list of free images belong to free component versions.
func (f *FreeVersion) List() ([]*FreeImage, error) {
	// list components
	components, err := db.GetManager().TenantServiceDao().GetAllServicesID()
	if err != nil {
		return nil, err
	}

	var images []*FreeImage
	for _, cpt := range components {
		// list free tags
		freeTags, err := f.listFreeTags(cpt.ServiceID)
		if err != nil {
			logrus.Warningf("list free tags for repository %s: %v", cpt.ServiceID, err)
			continue
		}

		// component.ServiceID is the repository of image
		freeImages, err := f.listFreeImages(cpt.ServiceID, freeTags)
		if err != nil {
			logrus.Warningf("list digests for repository %s: %v", cpt.ServiceID, err)
			continue
		}
		images = append(images, freeImages...)
	}

	return images, nil
}

func (f *FreeVersion) listFreeTags(serviceID string) ([]string, error) {
	// all tags
	// serviceID is the repository of image
	tags, err := f.reg.Tags(serviceID)
	if err != nil {
		if errors.Is(err, registry.ErrRepositoryNotFound) {
			return nil, nil
		}
		return nil, err
	}

	// versions being used.
	versions, err := f.listVersions(serviceID)
	if err != nil {
		return nil, err
	}

	var freeTags []string
	for _, tag := range tags {
		_, ok := versions[tag]
		if !ok {
			freeTags = append(freeTags, tag)
		}
	}

	return freeTags, nil
}

func (f *FreeVersion) listVersions(serviceID string) (map[string]struct{}, error) {
	// tags being used
	rawVersions, err := db.GetManager().VersionInfoDao().ListByServiceIDStatus(serviceID, util.Bool(true))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return map[string]struct{}{}, nil
		}
		return nil, err
	}
	// make a map of versions
	versions := make(map[string]struct{})
	for _, version := range rawVersions {
		versions[version.BuildVersion] = struct{}{}
	}
	return versions, nil
}

func (f *FreeVersion) listFreeImages(repository string, tags []string) ([]*FreeImage, error) {
	var images []*FreeImage
	for _, tag := range tags {
		digest, err := f.reg.ManifestDigestV2(repository, tag)
		if err != nil {
			logrus.Warningf("get digest for manifest %s/%s: %v", repository, tag, err)
			continue
		}
		images = append(images, &FreeImage{
			Repository: repository,
			Digest:     digest.String(),
			Tag:        tag,
			Type:       string(FreeImageTypeFreeVersion),
		})
	}
	return images, nil
}
