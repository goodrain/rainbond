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

package sources

// ContainerMetadata holds all necessary information for building the container
// name. The container runtime is encouraged to expose the metadata in its user
// interface for better user experience. E.g., runtime can construct a unique
// container name based on the metadata. Note that (name, attempt) is unique
// within a sandbox for the entire lifetime of the sandbox.
type ContainerMetadata struct {
	// Name of the container. Same as the container name in the PodSpec.
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// Attempt number of creating the container. Default: 0.
	Attempt uint32 `protobuf:"varint,2,opt,name=attempt,proto3" json:"attempt,omitempty"`
}

// Device specifies a host device to mount into a container.
type Device struct {
	// Path of the device within the container.
	ContainerPath string `protobuf:"bytes,1,opt,name=container_path,json=containerPath,proto3" json:"container_path,omitempty"`
	// Path of the device on the host.
	HostPath string `protobuf:"bytes,2,opt,name=host_path,json=hostPath,proto3" json:"host_path,omitempty"`
	// Cgroups permissions of the device, candidates are one or more of
	// * r - allows container to read from the specified device.
	// * w - allows container to write to the specified device.
	// * m - allows container to create device files that do not yet exist.
	Permissions string `protobuf:"bytes,3,opt,name=permissions,proto3" json:"permissions,omitempty"`
}

// ImageSpec is an internal representation of an image.  Currently, it wraps the
// value of a Container's Image field (e.g. imageID or imageDigest), but in the
// future it will include more detailed information about the different image types.
type ImageSpec struct {
	Image string `protobuf:"bytes,1,opt,name=image,proto3" json:"image,omitempty"`
}

//KeyValue KeyValue
type KeyValue struct {
	Key   string `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Value string `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
}

// Mount specifies a host volume to mount into a container.
type Mount struct {
	// Path of the mount within the container.
	ContainerPath string `protobuf:"bytes,1,opt,name=container_path,json=containerPath,proto3" json:"container_path,omitempty"`
	// Path of the mount on the host.
	HostPath string `protobuf:"bytes,2,opt,name=host_path,json=hostPath,proto3" json:"host_path,omitempty"`
	// If set, the mount is read-only.
	Readonly bool `protobuf:"varint,3,opt,name=readonly,proto3" json:"readonly,omitempty"`
	// If set, the mount needs SELinux relabeling.
	SelinuxRelabel bool `protobuf:"varint,4,opt,name=selinux_relabel,json=selinuxRelabel,proto3" json:"selinux_relabel,omitempty"`
}

//NetworkConfig network config for container
type NetworkConfig struct {
	NetworkMode string `json:"network_mode"`
}

// ContainerConfig holds all the required and optional fields for creating a
// container.
type ContainerConfig struct {
	// Metadata of the container. This information will uniquely identify the
	// container, and the runtime should leverage this to ensure correct
	// operation. The runtime may also use this information to improve UX, such
	// as by constructing a readable name.
	Metadata *ContainerMetadata `protobuf:"bytes,1,opt,name=metadata" json:"metadata,omitempty"`
	// Image to use.
	Image *ImageSpec `protobuf:"bytes,2,opt,name=image" json:"image,omitempty"`
	// Command to execute (i.e., entrypoint for docker)
	Command []string `protobuf:"bytes,3,rep,name=command" json:"command,omitempty"`
	// Args for the Command (i.e., command for docker)
	Args []string `protobuf:"bytes,4,rep,name=args" json:"args,omitempty"`
	// Current working directory of the command.
	WorkingDir string `protobuf:"bytes,5,opt,name=working_dir,json=workingDir,proto3" json:"working_dir,omitempty"`
	// List of environment variable to set in the container.
	Envs []*KeyValue `protobuf:"bytes,6,rep,name=envs" json:"envs,omitempty"`
	// Mounts for the container.
	Mounts []*Mount `protobuf:"bytes,7,rep,name=mounts" json:"mounts,omitempty"`
	// Devices for the container.
	Devices []*Device `protobuf:"bytes,8,rep,name=devices" json:"devices,omitempty"`
	// Key-value pairs that may be used to scope and select individual resources.
	// Label keys are of the form:
	//     label-key ::= prefixed-name | name
	//     prefixed-name ::= prefix '/' name
	//     prefix ::= DNS_SUBDOMAIN
	//     name ::= DNS_LABEL
	Labels map[string]string `protobuf:"bytes,9,rep,name=labels" json:"labels,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	// Unstructured key-value map that may be used by the kubelet to store and
	// retrieve arbitrary metadata.
	//
	// Annotations MUST NOT be altered by the runtime; the annotations stored
	// here MUST be returned in the ContainerStatus associated with the container
	// this ContainerConfig creates.
	//
	// In general, in order to preserve a well-defined interface between the
	// kubelet and the container runtime, annotations SHOULD NOT influence
	// runtime behaviour.
	Annotations map[string]string `protobuf:"bytes,10,rep,name=annotations" json:"annotations,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	// Path relative to PodSandboxConfig.LogDirectory for container to store
	// the log (STDOUT and STDERR) on the host.
	// E.g.,
	//     PodSandboxConfig.LogDirectory = `/var/log/pods/<podUID>/`
	//     ContainerConfig.LogPath = `containerName_Instance#.log`
	//
	// WARNING: Log management and how kubelet should interface with the
	// container logs are under active discussion in
	// https://issues.k8s.io/24677. There *may* be future change of direction
	// for logging as the discussion carries on.
	LogPath string `protobuf:"bytes,11,opt,name=log_path,json=logPath,proto3" json:"log_path,omitempty"`
	// Variables for interactive containers, these have very specialized
	// use-cases (e.g. debugging).
	// TODO: Determine if we need to continue supporting these fields that are
	// part of Kubernetes's Container Spec.
	Stdin     bool `protobuf:"varint,12,opt,name=stdin,proto3" json:"stdin,omitempty"`
	StdinOnce bool `protobuf:"varint,13,opt,name=stdin_once,json=stdinOnce,proto3" json:"stdin_once,omitempty"`
	Tty       bool `protobuf:"varint,14,opt,name=tty,proto3" json:"tty,omitempty"`
	// Configuration specific to Linux containers.
	//Linux *LinuxContainerConfig `protobuf:"bytes,15,opt,name=linux" json:"linux,omitempty"`
	AttachStdin   bool
	AttachStdout  bool
	AttachStderr  bool
	NetworkConfig *NetworkConfig
}

//GetMetadata GetMetadata
func (m *ContainerConfig) GetMetadata() *ContainerMetadata {
	if m != nil {
		return m.Metadata
	}
	return nil
}

//GetImage GetImage
func (m *ContainerConfig) GetImage() *ImageSpec {
	if m != nil {
		return m.Image
	}
	return nil
}

//GetEnvs GetEnvs
func (m *ContainerConfig) GetEnvs() []*KeyValue {
	if m != nil {
		return m.Envs
	}
	return nil
}

//GetMounts GetMounts
func (m *ContainerConfig) GetMounts() []*Mount {
	if m != nil {
		return m.Mounts
	}
	return nil
}

//GetDevices GetDevices
func (m *ContainerConfig) GetDevices() []*Device {
	if m != nil {
		return m.Devices
	}
	return nil
}

//GetLabels GetLabels
func (m *ContainerConfig) GetLabels() map[string]string {
	if m != nil {
		return m.Labels
	}
	return nil
}

//GetAnnotations GetAnnotations
func (m *ContainerConfig) GetAnnotations() map[string]string {
	if m != nil {
		return m.Annotations
	}
	return nil
}
