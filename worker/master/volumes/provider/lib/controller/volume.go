/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// 本文件定义了一个持久卷（Persistent Volume, PV）供应器接口及其相关类型和方法，主要用于与 Kubernetes 集群中的存储资源进行交互。
// 该接口使得实现自定义 PV 供应器成为可能，这些供应器能够根据 PersistentVolumeClaim (PVC) 的请求动态地在底层存储中创建和删除卷。

// 1. **Provisioner 接口**：
//    - `Provisioner` 是一个接口，定义了两个主要方法：`Provision` 和 `Delete`。
//    - `Provision` 方法用于根据 PVC 请求创建一个新的存储卷，并返回一个代表该卷的 PV 对象。
//    - `Delete` 方法用于删除由 `Provision` 方法创建的存储卷，但不会删除 PV 对象本身。
//    - `Name` 方法用于返回该供应器的名称。

// 2. **Qualifier 接口**：
//    - `Qualifier` 是一个可选接口，供应器可以实现该接口来决定是否应该尽早为 PVC 进行资源分配（如在领导者选举之前）。
//    - `ShouldProvision` 方法根据给定的 PVC 判断是否应尝试进行资源分配。

// 3. **BlockProvisioner 接口**：
//    - `BlockProvisioner` 是另一个可选接口，用于判断供应器是否支持块存储卷（Block Volume）。
//    - `SupportsBlock` 方法返回该供应器是否支持块存储卷的布尔值。

// 4. **IgnoredError 结构体**：
//    - `IgnoredError` 是一个错误类型，当 `Delete` 方法忽略了对某个 PV 的删除请求时，返回该类型的错误。
//    - 当有多个供应器服务于同一个存储类时，供应器可以通过返回 `IgnoredError` 来忽略它们没有创建的 PV，从而避免误导性的 `VolumeFailedDelete` 事件。

// 5. **VolumeOptions 结构体**：
//    - `VolumeOptions` 包含有关卷的选项信息，用于为 PV 创建提供所需的详细信息。
//    - 该结构体包括了 PV 的回收策略（`PersistentVolumeReclaimPolicy`）、PV 名称（`PVName`）、挂载选项（`MountOptions`）、PVC 引用（`PVC`）、实际的卷资源（`PersistentVolumeSource`）、从存储类获取的参数（`Parameters`）、调度器选择的节点（`SelectedNode`）、拓扑约束参数（`AllowedTopologies`）等信息。
//    - 这些信息对于动态创建和管理存储卷至关重要。

// 总的来说，本文件定义了一个通用接口，使得 Kubernetes 能够通过自定义实现与底层存储提供者进行交互，提供了动态存储卷的创建和管理能力。

package controller

import (
	"fmt"

	"k8s.io/api/core/v1"
)

// Provisioner is an interface that creates templates for PersistentVolumes
// and can create the volume as a new resource in the infrastructure provider.
// It can also remove the volume it created from the underlying storage
// provider.
type Provisioner interface {
	// Provision creates a volume i.e. the storage asset and returns a PV object
	// for the volume
	Provision(VolumeOptions) (*v1.PersistentVolume, error)
	// Delete removes the storage asset that was created by Provision backing the
	// given PV. Does not delete the PV object itself.
	//
	// May return IgnoredError to indicate that the call has been ignored and no
	// action taken.
	Delete(*v1.PersistentVolume) error
	Name() string
}

// Qualifier is an optional interface implemented by provisioners to determine
// whether a claim should be provisioned as early as possible (e.g. prior to
// leader election).
type Qualifier interface {
	// ShouldProvision returns whether provisioning for the claim should
	// be attempted.
	ShouldProvision(*v1.PersistentVolumeClaim) bool
}

// BlockProvisioner is an optional interface implemented by provisioners to determine
// whether it supports block volume.
type BlockProvisioner interface {
	Provisioner
	// SupportsBlock returns whether provisioner supports block volume.
	SupportsBlock() bool
}

// IgnoredError is the value for Delete to return to indicate that the call has
// been ignored and no action taken. In case multiple provisioners are serving
// the same storage class, provisioners may ignore PVs they are not responsible
// for (e.g. ones they didn't create). The controller will act accordingly,
// i.e. it won't emit a misleading VolumeFailedDelete event.
type IgnoredError struct {
	Reason string
}

func (e *IgnoredError) Error() string {
	return fmt.Sprintf("ignored because %s", e.Reason)
}

// VolumeOptions contains option information about a volume
// https://github.com/kubernetes/kubernetes/blob/release-1.4/pkg/volume/plugins.go
type VolumeOptions struct {
	// Reclamation policy for a persistent volume
	PersistentVolumeReclaimPolicy v1.PersistentVolumeReclaimPolicy
	// PV.Name of the appropriate PersistentVolume. Used to generate cloud
	// volume name.
	PVName string

	// PV mount options. Not validated - mount of the PVs will simply fail if one is invalid.
	MountOptions []string

	// PVC is reference to the claim that lead to provisioning of a new PV.
	// Provisioners *must* create a PV that would be matched by this PVC,
	// i.e. with required capacity, accessMode, labels matching PVC.Selector and
	// so on.
	PVC *v1.PersistentVolumeClaim
	// The actual volume backing the persistent volume.
	v1.PersistentVolumeSource `json:",inline"`
	// Volume provisioning parameters from StorageClass
	Parameters map[string]string

	// Node selected by the scheduler for the volume.
	SelectedNode *v1.Node
	// Topology constraint parameter from StorageClass
	AllowedTopologies []v1.TopologySelectorTerm
}
