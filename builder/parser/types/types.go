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

package types

// Service represents a module in a multi-module project.
type Service struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`  // module name
	Cname     string          `json:"cname"` // service cname
	Packaging string          `json:"packaging"`
	Envs      map[string]*Env `json:"envs,omitempty"`
	Ports     map[int]*Port   `json:"ports,omitempty"`
}

// Port -
type Port struct {
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

// Volume -
type Volume struct {
	VolumePath  string `json:"volume_path"`
	VolumeType  string `json:"volume_type"`
	FileContent string `json:"file_content,omitempty"`
}

// Env env desc
type Env struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Image -
type Image struct {
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
}
