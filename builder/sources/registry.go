// RAINBOND, Application Management Platform
// Copyright (C) 2014-2019 Goodrain Co., Ltd.

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

package sources

import (
	"fmt"

	"github.com/goodrain/rainbond/builder/sources/registry"

	"github.com/docker/distribution/reference"
	"github.com/sirupsen/logrus"
)

//GetTagFromNamedRef get image tag by name
func GetTagFromNamedRef(ref reference.Named) string {
	if digested, ok := ref.(reference.Digested); ok {
		return digested.Digest().String()
	}
	ref = reference.TagNameOnly(ref)
	if tagged, ok := ref.(reference.Tagged); ok {
		return tagged.Tag()
	}
	return ""
}

//ImageExist check image exist
func ImageExist(imageName, user, password string) (bool, error) {
	ref, err := reference.ParseAnyReference(imageName)
	if err != nil {
		logrus.Errorf("reference image error: %s", err.Error())
		return false, err
	}
	name, err := reference.ParseNamed(ref.String())
	if err != nil {
		logrus.Errorf("reference parse image name error: %s", err.Error())
		return false, err
	}
	domain := reference.Domain(name)
	if domain == "docker.io" {
		domain = "registry-1.docker.io"
	}
	retry := 2
	var rerr error
	for retry > 0 {
		retry--
		reg, err := registry.New(domain, user, password)
		if err != nil {
			reg, err = registry.NewInsecure(domain, user, password)
			if err != nil {
				logrus.Errorf("new registry client failure %s", err.Error())
				rerr = err
				continue
			}
		}
		tag := GetTagFromNamedRef(name)
		if err := reg.CheckManifest(reference.Path(name), tag); err != nil {
			rerr = fmt.Errorf("[ImageExist] check manifest v2: %v", err)
			continue
		}
		return true, nil
	}
	return false, rerr
}
