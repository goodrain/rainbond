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
	"github.com/docker/libcompose/yaml"
)

// ComposeObject holds the generic struct of Kompose transformation
type ComposeObject struct {
	ServiceConfigs map[string]ServiceConfig
	SupportReport  *FieldSupportReport // Field support report for warnings and errors
}

// ConvertOptions holds all options that controls transformation process
type ConvertOptions struct {
	ToStdout                    bool
	CreateD                     bool
	CreateRC                    bool
	CreateDS                    bool
	CreateDeploymentConfig      bool
	BuildRepo                   string
	BuildBranch                 string
	Build                       string
	CreateChart                 bool
	GenerateYaml                bool
	GenerateJSON                bool
	EmptyVols                   bool
	Volumes                     string
	InsecureRepository          bool
	Replicas                    int
	InputFiles                  []string
	OutFile                     string
	Provider                    string
	Namespace                   string
	Controller                  string
	IsDeploymentFlag            bool
	IsDaemonSetFlag             bool
	IsReplicationControllerFlag bool
	IsReplicaSetFlag            bool
	IsDeploymentConfigFlag      bool
	IsNamespaceFlag             bool
}

// ServiceConfig holds the basic struct of a container
type ServiceConfig struct {
	ContainerName    string
	Image            string              `compose:"image"`
	Environment      []EnvVar            `compose:"environment"`
	EnvFile          []string            `compose:"env_file"`
	Port             []Ports             `compose:"ports"`
	Command          []string            `compose:"command"`
	DependsON        []string            `compose:"depends_on"`
	Links            []string            `compose:"links"`
	WorkingDir       string              `compose:""`
	Args             []string            `compose:"args"`
	VolList          []string            `compose:"volumes"`
	Network          []string            `compose:"network"`
	Labels           map[string]string   `compose:"labels"`
	Annotations      map[string]string   `compose:""`
	CPUSet           string              `compose:"cpuset"`
	CPUShares        int64               `compose:"cpu_shares"`
	CPUQuota         int64               `compose:"cpu_quota"`
	CPULimit         int64               `compose:""`
	CPUReservation   int64               `compose:""`
	CapAdd           []string            `compose:"cap_add"`
	CapDrop          []string            `compose:"cap_drop"`
	Expose           []string            `compose:"expose"`
	Pid              string              `compose:"pid"`
	Privileged       bool                `compose:"privileged"`
	Restart          string              `compose:"restart"`
	User             string              `compose:"user"`
	VolumesFrom      []string            `compose:"volumes_from"`
	ServiceType      string              `compose:"kompose.service.type"`
	StopGracePeriod  string              `compose:"stop_grace_period"`
	Build            string              `compose:"build"`
	BuildArgs        map[string]*string  `compose:"build-args"`
	ExposeService    string              `compose:"kompose.service.expose"`
	ExposeServiceTLS string              `compose:"kompose.service.expose.tls-secret"`
	Stdin            bool                `compose:"stdin_open"`
	Tty              bool                `compose:"tty"`
	MemLimit         yaml.MemStringorInt `compose:"mem_limit"`
	MemReservation   yaml.MemStringorInt `compose:""`
	DeployMode       string              `compose:""`
	TmpFs            []string            `compose:"tmpfs"`
	Dockerfile       string              `compose:"dockerfile"`
	Replicas         int                 `compose:"replicas"`
	GroupAdd         []int64             `compose:"group_add"`
	Volumes          []Volumes           `compose:""`
	HealthChecks     HealthCheck         `compose:""`
	Placement        map[string]string   `compose:""`
}

// HealthCheck the healthcheck configuration for a service
// "StartPeriod" is not yet added to compose, see:
// https://github.com/docker/cli/issues/116
type HealthCheck struct {
	Test        []string
	Timeout     int32
	Interval    int32
	Retries     int32
	StartPeriod int32
	Disable     bool
}

// EnvVar holds the environment variable struct of a container
type EnvVar struct {
	Name  string
	Value string
}

// Ports holds the ports struct of a container
type Ports struct {
	HostPort      int32
	ContainerPort int32
	HostIP        string
	Protocol      string
}

// Volumes holds the volume struct of container
type Volumes struct {
	SvcName    string // Service name to which volume is linked
	MountPath  string // Mountpath extracted from docker-compose file
	VFrom      string // denotes service name from which volume is coming
	VolumeName string // name of volume if provided explicitly
	Host       string // host machine address
	Container  string // Mountpath
	Mode       string // access mode for volume
	PVCName    string // name of PVC
	PVCSize    string // PVC size
	VolumeType string // volume type: "config-file" or empty for regular volume
}
