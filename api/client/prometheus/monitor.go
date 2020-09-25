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

package prometheus

//Level -
type Level int

const (
	LevelCluster = 1 << iota
	LevelNode
	LevelWorkspace
	LevelNamespace
	LevelWorkload
	LevelPod
	LevelContainer
	LevelPVC
	LevelComponent
)

type QueryOption interface {
	Apply(*QueryOptions)
}

type QueryOptions struct {
	Level Level

	ResourceFilter            string
	NodeName                  string
	WorkspaceName             string
	NamespaceName             string
	WorkloadKind              string
	WorkloadName              string
	PodName                   string
	ContainerName             string
	StorageClassName          string
	PersistentVolumeClaimName string
}

func NewQueryOptions() *QueryOptions {
	return &QueryOptions{}
}

type ClusterOption struct{}

func (_ ClusterOption) Apply(o *QueryOptions) {
	o.Level = LevelCluster
}

type NodeOption struct {
	ResourceFilter string
	NodeName       string
}

func (no NodeOption) Apply(o *QueryOptions) {
	o.Level = LevelNode
	o.ResourceFilter = no.ResourceFilter
	o.NodeName = no.NodeName
}

type WorkspaceOption struct {
	ResourceFilter string
	WorkspaceName  string
}

func (wo WorkspaceOption) Apply(o *QueryOptions) {
	o.Level = LevelWorkspace
	o.ResourceFilter = wo.ResourceFilter
	o.WorkspaceName = wo.WorkspaceName
}

type NamespaceOption struct {
	ResourceFilter string
	WorkspaceName  string
	NamespaceName  string
}

func (no NamespaceOption) Apply(o *QueryOptions) {
	o.Level = LevelNamespace
	o.ResourceFilter = no.ResourceFilter
	o.WorkspaceName = no.WorkspaceName
	o.NamespaceName = no.NamespaceName
}

type WorkloadOption struct {
	ResourceFilter string
	NamespaceName  string
	WorkloadKind   string
}

func (wo WorkloadOption) Apply(o *QueryOptions) {
	o.Level = LevelWorkload
	o.ResourceFilter = wo.ResourceFilter
	o.NamespaceName = wo.NamespaceName
	o.WorkloadKind = wo.WorkloadKind
}

type PodOption struct {
	ResourceFilter string
	NodeName       string
	NamespaceName  string
	WorkloadKind   string
	WorkloadName   string
	PodName        string
}

func (po PodOption) Apply(o *QueryOptions) {
	o.Level = LevelPod
	o.ResourceFilter = po.ResourceFilter
	o.NodeName = po.NodeName
	o.NamespaceName = po.NamespaceName
	o.WorkloadKind = po.WorkloadKind
	o.WorkloadName = po.WorkloadName
	o.PodName = po.PodName
}

type ContainerOption struct {
	ResourceFilter string
	NamespaceName  string
	PodName        string
	ContainerName  string
}

func (co ContainerOption) Apply(o *QueryOptions) {
	o.Level = LevelContainer
	o.ResourceFilter = co.ResourceFilter
	o.NamespaceName = co.NamespaceName
	o.PodName = co.PodName
	o.ContainerName = co.ContainerName
}

type PVCOption struct {
	ResourceFilter            string
	NamespaceName             string
	StorageClassName          string
	PersistentVolumeClaimName string
}

func (po PVCOption) Apply(o *QueryOptions) {
	o.Level = LevelPVC
	o.ResourceFilter = po.ResourceFilter
	o.NamespaceName = po.NamespaceName
	o.StorageClassName = po.StorageClassName
	o.PersistentVolumeClaimName = po.PersistentVolumeClaimName
}

type ComponentOption struct{}

func (_ ComponentOption) Apply(o *QueryOptions) {
	o.Level = LevelComponent
}
