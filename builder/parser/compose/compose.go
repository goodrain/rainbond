// Copyright (C) 2014-2018 Goodrain Co., Ltd.
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

package compose

import (
	"fmt"
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"

	"github.com/sirupsen/logrus"
)

// Compose is docker compose file loader, implements Loader interface
type Compose struct {
}

func checkUnsupportedKey(_ interface{}) []string {
	return nil
}

// LoadBytes loads a compose file byte into KomposeObject
func (c *Compose) LoadBytes(bodys [][]byte) (ComposeObject, error) {
	return c.LoadBytesWithWorkDir(bodys, "")
}

// LoadBytesWithWorkDir loads a compose file byte into KomposeObject with optional working directory
func (c *Compose) LoadBytesWithWorkDir(bodys [][]byte, workDir string) (ComposeObject, error) {

	// Load the json / yaml file in order to get the version value
	var version string

	for _, body := range bodys {
		composeVersion, err := getVersionFromByte(body)
		if err != nil {
			return ComposeObject{}, fmt.Errorf("Unable to load yaml/json file for version parsing,%s", err.Error())
		}

		// Check that the previous file loaded matches.
		if len(bodys) > 0 && version != "" && version != composeVersion {
			return ComposeObject{}, fmt.Errorf("All Docker Compose files must be of the same version")
		}
		version = composeVersion
	}

	// If no version specified, infer it from the content
	if version == "" {
		version = inferComposeVersion(bodys[0])
		logrus.Infof("No version specified, inferred version: %s", version)
	}

	logrus.Debugf("Docker Compose version: %s", version)

	// Convert based on version
	switch version {
	// Use libcompose for 1 or 2
	case "1", "1.0", "2", "2.0", "2.1", "2.2", "2.3", "2.4":
		co, err := parseV1V2(bodys)
		if err != nil {
			return ComposeObject{}, err
		}
		return co, nil
	// Use docker/cli for 3.0-3.7
	case "3", "3.0", "3.1", "3.2", "3.3", "3.4", "3.5", "3.6", "3.7":
		co, err := parseV3(bodys, workDir)
		if err != nil {
			return ComposeObject{}, err
		}
		return co, nil
	// Use compose-go for 3.8+, spec, or inferred spec
	case "3.8", "3.9", "3.10", "spec", "compose-spec":
		co, report, err := parseSpec(bodys, workDir)
		if err != nil {
			return ComposeObject{}, err
		}
		co.SupportReport = report
		return co, nil
	default:
		return ComposeObject{}, fmt.Errorf("Version %s of Docker Compose is not supported. Please use version 1, 2, 3, or Compose Spec", version)
	}

}

func getVersionFromFile(file string) (string, error) {
	type ComposeVersion struct {
		Version string `json:"version"` // This affects YAML as well
	}
	var version ComposeVersion

	loadedFile, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}

	err = yaml.Unmarshal(loadedFile, &version)
	if err != nil {
		return "", err
	}

	return version.Version, nil
}

func getVersionFromByte(body []byte) (string, error) {
	type ComposeVersion struct {
		Version string `json:"version"` // This affects YAML as well
	}
	var version ComposeVersion

	err := yaml.Unmarshal(body, &version)
	if err != nil {
		return "", err
	}
	return version.Version, nil
}
