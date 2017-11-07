
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

package model

//BuildPluginTaskBody BuildPluginTaskBody
type BuildPluginTaskBody struct {
	VersionID     string `json:"version_id"`
	TenantID      string `json:"tenant_id"`
	PluginID      string `json:"plugin_id"`
	Operator      string `json:"operator"`
	Repo          string `json:"repo"`
	GitURL        string `json:"git_url"`
	ImageURL      string `json:"image_url"`
	EventID       string `json:"event_id"`
	DeployVersion string `json:"deploy_version"`
	Kind          string `json:"kind"`
}

//BuildPluginVersion BuildPluginVersion
type BuildPluginVersion struct {
	SourceImage string `json:"source_image"`
	InnerImage  string `json:"inner_image"`
	CreateTime  string `json:"create_time"`
	Repo        string `json:"repo"`
}
