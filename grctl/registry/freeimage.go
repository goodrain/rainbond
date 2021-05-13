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
	"strings"

	"github.com/goodrain/rainbond/builder/sources/registry"
	"github.com/goodrain/rainbond/db"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// FreeImage represents a free image.
type FreeImage struct {
	Repository string
	Digest     string
}

// FreeImager is resposible for listing the free images.
type FreeImager interface {
	List() ([]*FreeImage, error)
}

// NewFreeImageres creates a list of new FreeImager.
func NewFreeImageres(reg *registry.Registry) map[string]FreeImager {
	freeImageres := make(map[string]FreeImager, 2)
	// there are two kinds of free images:
	// 1. images belongs to the free components
	// 2. images belongs to the free component versions.
	freeComponent := NewFreeComponent(reg)
	freeImageres["free component"] = freeComponent
	return freeImageres
}

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
		digests, err := f.listDigests(cpt.ServiceID)
		if err != nil {
			logrus.Warningf("list digests for repository %s: %v", cpt.ServiceID, err)
			continue
		}
		for _, digest := range digests {
			images = append(images, &FreeImage{
				Repository: cpt.ServiceID,
				Digest:     digest,
			})
		}
	}

	return images, nil
}

func (f *FreeComponent) listDigests(repository string) ([]string, error) {
	// list tags, then list digest for every tag
	tags, err := f.reg.Tags(repository)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return nil, nil
		}
		return nil, errors.Wrap(err, "list tags")
	}

	var digests []string
	for _, tag := range tags {
		digest, err := f.reg.ManifestDigestV2(repository, tag)
		if err != nil {
			logrus.Warningf("get digest for manifest %s/%s: %v", repository, tag, err)
			continue
		}
		digests = append(digests, digest.String())
	}
	return digests, nil
}
