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

package model

//BuildPluginTaskBody BuildPluginTaskBody
type BuildPluginTaskBody struct {
	VersionID     string `json:"version_id"`
	TenantID      string `json:"tenant_id"`
	PluginID      string `json:"plugin_id"`
	Operator      string `json:"operator"`
	Repo          string `json:"repo"`
	GitURL        string `json:"git_url"`
	GitUsername   string `json:"git_username"`
	GitPassword   string `json:"git_password"`
	ImageURL      string `json:"image_url"`
	EventID       string `json:"event_id"`
	DeployVersion string `json:"deploy_version"`
	Kind          string `json:"kind"`
	PluginCMD     string `json:"plugin_cmd"`
	PluginCPU     int    `json:"plugin_cpu"`
	PluginMemory  int    `json:"plugin_memory"`
	Arch          string `json:"arch"` // Plugin architecture (amd64, arm64, etc.)
	ImageInfo     struct {
		HubURL      string `json:"hub_url"`
		HubUser     string `json:"hub_user"`
		HubPassword string `json:"hub_password"`
		Namespace   string `json:"namespace"`
		IsTrust     bool   `json:"is_trust,omitempty"`
	} `json:"image_info,omitempty"`
}

//BuildPluginVersion BuildPluginVersion
type BuildPluginVersion struct {
	SourceImage string `json:"source_image"`
	InnerImage  string `json:"inner_image"`
	CreateTime  string `json:"create_time"`
	Repo        string `json:"repo"`
}

//CodeCheckResult CodeCheckResult
type CodeCheckResult struct {
	ServiceID    string `json:"service_id"`
	Condition    string `json:"condition"`
	CheckType    string `json:"check_type"`
	GitURL       string `json:"git_url"`
	CodeVersion  string `json:"code_version"`
	GitProjectId string `json:"git_project_id"`
	CodeFrom     string `json:"code_from"`
	URLRepos     string `json:"url_repos"`

	DockerFileReady bool              `json:"docker_file_ready,omitempty"`
	InnerPort       string            `json:"inner_port,omitempty"`
	VolumeMountPath string            `json:"volume_mount_path,omitempty"`
	BuildImageName  string            `json:"image,omitempty"`
	PortList        map[string]string `json:"port_list,omitempty"`
	VolumeList      []string          `json:"volume_list,omitempty"`

	//DFR          *DockerFileResult `json:"dockerfile,omitempty"`
}

//ImageName ImageName
type ImageName struct {
	Host      string `json:"host"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Tag       string `json:"tag"`
}
