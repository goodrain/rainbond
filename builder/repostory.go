// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

package builder

import "os"

func init() {
	if os.Getenv("BUILD_IMAGE_REPOSTORY_DOMAIN") != "" {
		REGISTRYDOMAIN = os.Getenv("BUILD_IMAGE_REPOSTORY_DOMAIN")
	}
	if os.Getenv("BUILD_IMAGE_REPOSTORY_USER") != "" {
		REGISTRYUSER = os.Getenv("BUILD_IMAGE_REPOSTORY_USER")
	}
	if os.Getenv("BUILD_IMAGE_REPOSTORY_PASS") != "" {
		REGISTRYPASS = os.Getenv("BUILD_IMAGE_REPOSTORY_PASS")
	}
	if os.Getenv("RUNNER_IMAGE_NAME") != "" {
		REGISTRYPASS = os.Getenv("RUNNER_IMAGE_NAME")
	}
	if os.Getenv("BUILDER_IMAGE_NAME") != "" {
		REGISTRYPASS = os.Getenv("BUILDER_IMAGE_NAME")
	}
}

//REGISTRYDOMAIN REGISTRY_DOMAIN
var REGISTRYDOMAIN = "goodrain.me"

//REGISTRYUSER REGSITRY USER NAME
var REGISTRYUSER = ""

//REGISTRYPASS REGSITRY PASSWORD
var REGISTRYPASS = ""

//RUNNERIMAGENAME runner image name
var RUNNERIMAGENAME = "goodrain.me/runner"

//BUILDERIMAGENAME builder image name
var BUILDERIMAGENAME = "goodrain.me/builder"
