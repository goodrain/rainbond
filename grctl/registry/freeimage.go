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
)

// FreeImageType is the type of FreeImage
type FreeImageType string

// FreeImageType -
var (
	FreeImageTypeFreeComponent FreeImageType = "FreeComponent"
	FreeImageTypeFreeVersion   FreeImageType = "FreeVersion"
)

// FreeImage represents a free image.
type FreeImage struct {
	Repository string
	Digest     string
	Tag        string
	Type       string
}

// Key returns the key of the FreeImaeg.
func (f *FreeImage) Key() string {
	return f.Repository + "/" + f.Digest
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
	freeImageres["free component"] = NewFreeComponent(reg)
	freeImageres["free version"] = NewFreeVersion(reg)
	return freeImageres
}
