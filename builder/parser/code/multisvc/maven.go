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

package multi

import (
	"encoding/xml"
	"io/ioutil"
	"path"
	"strings"

	"github.com/goodrain/rainbond/builder/parser/types"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
)

// maven is an implementation of MutilModuler.
type maven struct {
}

// NewMaven creates a new MultiModuler
func NewMaven() ServiceInterface {
	return &maven{}
}

// pom represents a maven pom.xml file
type pom struct {
	XMLName    xml.Name `xml:"project"`
	Name       string   `xml:"name"`
	ArtifactID string   `xml:"artifactId"`
	Version    string   `xml:"version"`
	Packaging  string   `xml:"packaging"`
	Modules    []string `xml:"modules>module"`
}

// module represents a maven module
type module struct {
	ID        string
	Name      string
	Packaging string
}

// ListModules lists all maven modules from pom.xml
func (m *maven) ListModules(path string) ([]*types.Service, error) {
	modules, err := listModules(path, strings.TrimRight(path, "/")+"/")
	if err != nil {
		return nil, err
	}
	var res []*types.Service
	for _, item := range modules {
		mo := &types.Service{
			ID:   item.ID,
			Name: item.Name,
			Cname: func(name string) string {
				cnames := strings.Split(name, "/")
				return cnames[len(cnames)-1]
			}(item.Name),
			Packaging: item.Packaging,
			Envs: map[string]*types.Env{
				"BUILD_MAVEN_BUILT_MODULE": {
					Name:  "BUILD_MAVEN_BUILT_MODULE",
					Value: item.Name,
				},
			},
		}
		res = append(res, mo)
	}
	return res, nil
}

func listModules(prefix, topPref string) ([]*module, error) {
	pomPath := path.Join(prefix, "pom.xml")
	pom, err := parsePom(pomPath)
	if err != nil {
		return nil, err
	}

	var modules []*module // module names
	// recursive end condition
	if pom.isValidModule() {
		// full module name. eg: foobar/rbd-worker
		name := strings.Replace(prefix, topPref, "", 1)
		mo := &module{
			ID:   util.NewUUID(),
			Name: name,
			Packaging: func() string {
				if pom.Packaging == "war" {
					return "war"
				}
				return "jar"
			}(),
		}
		return []*module{mo}, nil
	}

	for _, name := range pom.Modules {
		// submodule names
		submodules, err := listModules(path.Join(prefix, name), topPref)
		if err != nil {
			logrus.Warningf("Prefix: %s; error getting module names: %v",
				path.Join(prefix, name), err)
			continue
		}
		if submodules != nil && len(submodules) > 0 {
			modules = append(modules, submodules...)
		}
	}
	return modules, nil
}

// parsePom parses the pom.xml file into a pom struct
func parsePom(pomPath string) (*pom, error) {
	bytes, err := ioutil.ReadFile(pomPath)
	if err != nil {
		return nil, err
	}
	var pom pom
	if err = xml.Unmarshal(bytes, &pom); err != nil {
		return nil, err
	}
	return &pom, nil
}

// checks if the pom has submodules.
func (p *pom) hasSubmodules() bool {
	return len(p.Modules) > 0
}

func (p *pom) isValidModule() bool {
	if p.Packaging != "jar" && p.Packaging != "war" && p.Packaging != "" {
		return false
	}
	if p.Modules != nil || len(p.Modules) > 0 {
		return false
	}
	return true
}
