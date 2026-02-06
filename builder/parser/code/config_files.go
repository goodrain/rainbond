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

package code

import (
	"os"
	"path"

	"github.com/goodrain/rainbond/util"
)

// ConfigFiles represents detected configuration files in a Node.js project
type ConfigFiles struct {
	HasNpmrc  bool   // .npmrc exists
	HasYarnrc bool   // .yarnrc or .yarnrc.yml exists
	HasPnpmrc bool   // .pnpmrc exists
	NpmrcPath string // path to .npmrc if exists
	YarnrcPath string // path to .yarnrc or .yarnrc.yml if exists
	PnpmrcPath string // path to .pnpmrc if exists
}

// DetectConfigFiles detects configuration files in a Node.js project
func DetectConfigFiles(buildPath string) ConfigFiles {
	config := ConfigFiles{}

	// Check .npmrc
	npmrcPath := path.Join(buildPath, ".npmrc")
	if ok, _ := util.FileExists(npmrcPath); ok {
		config.HasNpmrc = true
		config.NpmrcPath = npmrcPath
	}

	// Check .yarnrc (classic) or .yarnrc.yml (berry/modern)
	yarnrcPath := path.Join(buildPath, ".yarnrc")
	yarnrcYmlPath := path.Join(buildPath, ".yarnrc.yml")
	if ok, _ := util.FileExists(yarnrcPath); ok {
		config.HasYarnrc = true
		config.YarnrcPath = yarnrcPath
	} else if ok, _ := util.FileExists(yarnrcYmlPath); ok {
		config.HasYarnrc = true
		config.YarnrcPath = yarnrcYmlPath
	}

	// Check .pnpmrc (though pnpm typically uses .npmrc)
	pnpmrcPath := path.Join(buildPath, ".pnpmrc")
	if ok, _ := util.FileExists(pnpmrcPath); ok {
		config.HasPnpmrc = true
		config.PnpmrcPath = pnpmrcPath
	}

	return config
}

// ReadConfigFileContent reads the content of a configuration file
func ReadConfigFileContent(filePath string) (string, error) {
	if filePath == "" {
		return "", nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// GetNpmrcContent returns the content of .npmrc file if it exists
func (c ConfigFiles) GetNpmrcContent() (string, error) {
	if !c.HasNpmrc {
		return "", nil
	}
	return ReadConfigFileContent(c.NpmrcPath)
}

// GetYarnrcContent returns the content of .yarnrc or .yarnrc.yml file if it exists
func (c ConfigFiles) GetYarnrcContent() (string, error) {
	if !c.HasYarnrc {
		return "", nil
	}
	return ReadConfigFileContent(c.YarnrcPath)
}

// GetPnpmrcContent returns the content of .pnpmrc file if it exists
func (c ConfigFiles) GetPnpmrcContent() (string, error) {
	if !c.HasPnpmrc {
		return "", nil
	}
	return ReadConfigFileContent(c.PnpmrcPath)
}

// HasAnyConfigFile returns true if any configuration file exists
func (c ConfigFiles) HasAnyConfigFile() bool {
	return c.HasNpmrc || c.HasYarnrc || c.HasPnpmrc
}

// GetRelevantConfigFile returns the path to the most relevant config file
// based on the package manager being used
func (c ConfigFiles) GetRelevantConfigFile(pm PackageManager) string {
	switch pm {
	case PackageManagerPNPM:
		// pnpm uses .npmrc primarily, but also supports .pnpmrc
		if c.HasPnpmrc {
			return c.PnpmrcPath
		}
		if c.HasNpmrc {
			return c.NpmrcPath
		}
	case PackageManagerYarn:
		if c.HasYarnrc {
			return c.YarnrcPath
		}
	case PackageManagerNPM:
		if c.HasNpmrc {
			return c.NpmrcPath
		}
	}
	return ""
}
