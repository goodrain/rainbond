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
	"github.com/sirupsen/logrus"
)

var _ FreeImager = &FreeComponent{}

// FreeComponent is resposible for listing the free images belong to free components.
type FreeComponent struct {
	reg *registry.Registry
}

// NewFreeComponent creates a new FreeComponent.
func NewFreeComponent(reg *registry.Registry) *FreeComponent {
	return &FreeComponent{
		reg: reg,
	}
}

// List return a list of free images belong to free components.
func (f *FreeComponent) List() ([]*FreeImage, error) {
	// list free components
	components, err := db.GetManager().TenantServiceDeleteDao().List()
	if err != nil {
		return nil, err
	}

	var images []*FreeImage
	for _, cpt := range components {
		// component.ServiceID is the repository of image
		freeImages, err := f.listFreeImages(cpt.ServiceID)
		if err != nil {
			logrus.Warningf("list free images: %v", err)
			continue
		}
		images = append(images, freeImages...)
	}

	return images, nil
}

func (f *FreeComponent) listFreeImages(repository string) ([]*FreeImage, error) {
	// list tags, then list digest for every tag
	tags, err := f.reg.Tags(repository)
	if err != nil {
		if errors.Is(err, registry.ErrRepositoryNotFound) {
			return nil, nil
		}
		return nil, err
	}

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
