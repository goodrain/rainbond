// RAINBOND, Application Management Platform
// Copyright (C) 2020-2020 Goodrain Co., Ltd.

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

package oam

import (
	"fmt"
	"strings"

	v1alpha2 "github.com/crossplane/oam-kubernetes-runtime/apis/core/v1alpha2"
	v1alpha1 "github.com/goodrain/rainbond-oam/pkg/ram/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type containerWorkloadBuilder struct {
	com     v1alpha1.Component
	plugins []v1alpha1.Plugin
	output  []v1alpha2.DataOutput
}

func (c *containerWorkloadBuilder) Build() runtime.RawExtension {
	oamOS := v1alpha2.OperatingSystemLinux
	oamCPU := v1alpha2.CPUArchitectureAMD64
	var cw = &v1alpha2.ContainerizedWorkload{
		ObjectMeta: metav1.ObjectMeta{
			Name:        c.com.ServiceCname,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
		Spec: v1alpha2.ContainerizedWorkloadSpec{
			OperatingSystem: &oamOS,
			CPUArchitecture: &oamCPU,
			Containers:      c.buildContainers(),
		},
	}
	return runtime.RawExtension{Object: cw}
}

func (c *containerWorkloadBuilder) Kind() string {
	return "ContainerWorkload"
}
func (c *containerWorkloadBuilder) Output() []v1alpha2.DataOutput {
	return c.output
}

func (c *containerWorkloadBuilder) buildContainers() []v1alpha2.Container {
	com := c.com
	var containers []v1alpha2.Container
	mainContainer := v1alpha2.Container{
		Name:  com.ServiceName,
		Image: com.Image,
		Resources: &v1alpha2.ContainerResources{
			Memory: v1alpha2.MemoryResources{
				Required: NewMemoryQuantity(com.Memory),
			},
			CPU: v1alpha2.CPUResources{
				Required: NewCPUQuantity(com.CPU),
			},
			Volumes: c.buildVolumes(com.ServiceVolumeMapList, com.MntReleationList),
		},
		Command:         strings.Split(com.Cmd, " "),
		Environment:     c.buildEnv(com.Envs, com.ServiceConnectInfoMapList, true),
		ConfigFiles:     c.buildConfigFile(com.ServiceVolumeMapList),
		Ports:           c.buildPorts(com.Ports),
		LivenessProbe:   c.buildLivenessProbe(com.Probes),
		ReadinessProbe:  c.buildReadinessProbe(com.Probes),
		ImagePullSecret: c.buildImagePullSecret(com.AppImage),
	}
	containers = append(containers, mainContainer)
	//plugin container
	for _, pluginConfig := range com.ServicePluginConfigs {
		plugin := c.getPlugin(pluginConfig.PluginKey)
		if plugin != nil {
			containers = append(containers, c.buildPluginContainer(*plugin, pluginConfig, com))
		}
	}
	return containers
}

//TODO: share volume
func (c *containerWorkloadBuilder) buildVolumes(volumes v1alpha1.ComponentVolumeList, shareVolume []v1alpha1.ComponentShareVolume) (re []v1alpha2.VolumeResource) {
	for _, volume := range volumes {
		if volume.VolumeType == v1alpha1.ConfigFileVolumeType {
			continue
		}
		vr := v1alpha2.VolumeResource{
			Name:          volume.VolumeName,
			MountPath:     volume.VolumeMountPath,
			AccessMode:    NewVolumeAccess(volume.AccessMode),
			SharingPolicy: NewSharingPolicy(volume.SharingPolicy),
			Disk:          &v1alpha2.DiskResource{},
		}
		if volume.VolumeCapacity > 0 {
			vr.Disk.Required = NewDiskQuantity(volume.VolumeCapacity)
		}
		re = append(re, vr)
	}
	return
}

//TODO: share config file
func (c *containerWorkloadBuilder) buildConfigFile(volumes v1alpha1.ComponentVolumeList) (re []v1alpha2.ContainerConfigFile) {
	for _, volume := range volumes {
		if volume.VolumeType != v1alpha1.ConfigFileVolumeType {
			continue
		}
		vr := v1alpha2.ContainerConfigFile{
			Path:  volume.VolumeMountPath,
			Value: &volume.FileConent,
		}
		re = append(re, vr)
	}
	return
}

func (c *containerWorkloadBuilder) buildEnv(envs, connect []v1alpha1.ComponentEnv, insetOutput bool) (re []v1alpha2.ContainerEnvVar) {
	for _, env := range envs {
		re = append(re, v1alpha2.ContainerEnvVar{
			Name:  env.AttrName,
			Value: &env.AttrValue,
		})
	}
	for _, out := range connect {
		re = append(re, v1alpha2.ContainerEnvVar{
			Name:  out.AttrName,
			Value: &out.AttrValue,
		})
		if insetOutput {
			c.output = append(c.output, v1alpha2.DataOutput{
				Name:      out.AttrName,
				FieldPath: fmt.Sprintf("spec.container[0].env[%d].value", len(re)-1),
			})
		}
	}
	return
}

//TODO: create service
func (c *containerWorkloadBuilder) buildPorts(ports []v1alpha1.ComponentPort) (re []v1alpha2.ContainerPort) {
	for _, p := range ports {
		re = append(re, v1alpha2.ContainerPort{
			Name:     strings.ToLower(p.PortAlias),
			Port:     int32(p.ContainerPort),
			Protocol: NewTransportProtocol(p.Protocol),
		})
	}
	return
}

//TODO: build secret
func (c *containerWorkloadBuilder) buildImagePullSecret(info v1alpha1.ImageInfo) *string {
	var secret string
	return &secret
}

func (c *containerWorkloadBuilder) buildLivenessProbe(probes []v1alpha1.ComponentProbe) *v1alpha2.ContainerHealthProbe {
	for _, probe := range probes {
		if probe.Mode == "livebess" {
			return createProbe(probe)
		}
	}
	return nil
}

func (c *containerWorkloadBuilder) buildReadinessProbe(probes []v1alpha1.ComponentProbe) *v1alpha2.ContainerHealthProbe {
	for _, probe := range probes {
		if probe.Mode == "readiness" {
			return createProbe(probe)
		}
	}
	return nil
}

func (c *containerWorkloadBuilder) buildPluginContainer(plugin v1alpha1.Plugin, pluginConfig v1alpha1.ComponentPluginConfig, com v1alpha1.Component) v1alpha2.Container {
	return v1alpha2.Container{
		Name:  plugin.PluginName,
		Image: plugin.Image,
		Resources: &v1alpha2.ContainerResources{
			Memory: v1alpha2.MemoryResources{
				Required: NewMemoryQuantity(pluginConfig.MemoryRequired),
			},
			CPU: v1alpha2.CPUResources{
				Required: NewCPUQuantity(pluginConfig.CPURequired),
			},
			Volumes: c.buildVolumes(com.ServiceVolumeMapList, com.MntReleationList),
		},
		Command:         strings.Split(com.Cmd, " "),
		Environment:     c.buildEnv(c.com.Envs, c.com.ServiceConnectInfoMapList, false),
		ConfigFiles:     c.buildConfigFile(c.com.ServiceVolumeMapList),
		ImagePullSecret: c.buildImagePullSecret(plugin.PluginImage),
	}
}

func createProbe(probe v1alpha1.ComponentProbe) *v1alpha2.ContainerHealthProbe {
	return &v1alpha2.ContainerHealthProbe{
		Exec: func() *v1alpha2.ExecProbe {
			if probe.Cmd != "" {
				return &v1alpha2.ExecProbe{
					Command: strings.Split(probe.Cmd, " "),
				}
			}
			return nil
		}(),
		HTTPGet: func() *v1alpha2.HTTPGetProbe {
			if probe.Scheme == "http" {
				return &v1alpha2.HTTPGetProbe{
					Path: probe.Path,
					Port: int32(probe.Port),
					HTTPHeaders: func() []v1alpha2.HTTPHeader {
						if probe.HTTPHeader == "" {
							return nil
						}
						hds := strings.Split(probe.HTTPHeader, ",")
						var headers []v1alpha2.HTTPHeader
						for _, hd := range hds {
							kv := strings.Split(hd, "=")
							if len(kv) == 1 {
								header := v1alpha2.HTTPHeader{
									Name:  kv[0],
									Value: "",
								}
								headers = append(headers, header)
							} else if len(kv) == 2 {
								header := v1alpha2.HTTPHeader{
									Name:  kv[0],
									Value: kv[1],
								}
								headers = append(headers, header)
							}
						}
						return headers
					}(),
				}
			}
			return nil
		}(),
		TCPSocket: func() *v1alpha2.TCPSocketProbe {
			if probe.Scheme == "tcp" {
				return &v1alpha2.TCPSocketProbe{
					Port: int32(probe.Port),
				}
			}
			return nil
		}(),
		InitialDelaySeconds: Int32(probe.InitialDelaySecond),
		PeriodSeconds:       Int32(probe.PeriodSecond),
		TimeoutSeconds:      Int32(probe.TimeoutSecond),
		SuccessThreshold:    Int32(probe.SuccessThreshold),
		FailureThreshold:    Int32(probe.FailureThreshold),
	}
}

func (c *containerWorkloadBuilder) getPlugin(key string) *v1alpha1.Plugin {
	for _, p := range c.plugins {
		if p.PluginKey == key {
			return &p
		}
	}
	return nil
}
