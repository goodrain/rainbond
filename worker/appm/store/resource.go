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

// 该文件定义了一个用于管理和缓存Rainbond平台上租户资源的结构体 `ResourceCache` 及其相关方法。
// 通过实现对Kubernetes Pod 资源的管理和计算，该文件为平台提供了资源使用情况的实时监控和查询能力。

// 文件中的主要功能包括：
// 1. `ResourceCache` 结构体：这是一个资源缓存的实现，主要用于存储和管理各个租户的资源信息。
//    资源信息包括CPU和内存的请求值和限制值，这些信息通过 `NamespaceResource` 结构体来组织和管理。
// 2. 资源管理方法：`SetPodResource` 方法用于将一个Pod的资源信息添加到缓存中，`RemovePod` 方法用于从缓存中移除一个Pod的资源信息。
//    这些方法确保了缓存中的资源信息是最新的，并且能够准确反映当前的资源使用情况。
// 3. 资源查询方法：`GetTenantResource` 方法用于获取指定命名空间下的资源使用情况，`GetAllTenantResource` 方法则用于获取所有租户的资源信息。
//    这些方法使得平台能够快速查询和监控各个租户的资源使用情况，从而进行资源优化和管理。

// 总的来说，该文件通过定义和管理Rainbond平台的资源缓存，使平台能够高效地监控和管理租户资源的使用情况，
// 这对于优化资源配置、提高系统性能至关重要，特别是在多租户的云平台环境中。

package store

import (
	"sync"

	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
)

// ResourceCache resource cache
type ResourceCache struct {
	lock      sync.Mutex
	resources map[string]*NamespaceResource
}

// NewResourceCache new resource cache
func NewResourceCache() *ResourceCache {
	return &ResourceCache{
		resources: make(map[string]*NamespaceResource),
	}
}

// NamespaceResource namespace resource
type NamespaceResource map[string]*v1.PodResource

// SetPodResource set pod resource
func (r *NamespaceResource) SetPodResource(podName string, pr *v1.PodResource) {
	(*r)[podName] = pr
}

// RemovePod remove pod resource
func (r *NamespaceResource) RemovePod(podName string) {
	delete(*r, podName)
}

// TenantResource tenant resource
type TenantResource struct {
	Namespace     string
	MemoryRequest int64
	MemoryLimit   int64
	CPURequest    int64
	CPULimit      int64
}

// SetPodResource set pod resource
func (r *ResourceCache) SetPodResource(pod *corev1.Pod) {
	r.lock.Lock()
	defer r.lock.Unlock()
	namespace := pod.Namespace
	re := v1.CalculatePodResource(pod)
	// Compatible with resources with tenantID as namespace
	nsKeys := []string{namespace}
	labels := pod.Labels
	if tenantID, ok := labels["tenant_id"]; ok && tenantID != namespace {
		nsKeys = append(nsKeys, tenantID)
	}
	for _, ns := range nsKeys {
		if nr, ok := r.resources[ns]; ok && nr != nil {
			nr.SetPodResource(pod.Name, re)
		} else {
			nameR := make(NamespaceResource)
			nameR.SetPodResource(pod.Name, re)
			r.resources[ns] = &nameR
		}
	}
}

// RemovePod remove pod resource
func (r *ResourceCache) RemovePod(pod *corev1.Pod) {
	r.lock.Lock()
	defer r.lock.Unlock()
	namespace := pod.Namespace
	nsKeys := []string{namespace}
	labels := pod.Labels
	if tenantID, ok := labels["tenant_id"]; ok && tenantID != namespace {
		nsKeys = append(nsKeys, tenantID)
	}
	for _, ns := range nsKeys {
		if nr, ok := r.resources[ns]; ok && nr != nil {
			nr.RemovePod(pod.Name)
		}
	}
}

// GetTenantResource get tenant resource
func (r *ResourceCache) GetTenantResource(namespace string) (tr TenantResource) {
	r.lock.Lock()
	defer r.lock.Unlock()
	tr = r.getTenantResource(r.resources[namespace])
	tr.Namespace = namespace
	return tr
}

func (r *ResourceCache) getTenantResource(namespaceRe *NamespaceResource) (tr TenantResource) {
	if namespaceRe == nil {
		return
	}
	for _, v := range *namespaceRe {
		tr.CPULimit += v.CPULimit
		tr.MemoryLimit += v.MemoryLimit
		tr.CPURequest += v.CPURequest
		tr.MemoryRequest += v.MemoryRequest
	}
	return
}

// GetAllTenantResource get all tenant resources
func (r *ResourceCache) GetAllTenantResource() (trs []TenantResource) {
	r.lock.Lock()
	defer r.lock.Unlock()
	for k := range r.resources {
		tr := r.getTenantResource(r.resources[k])
		tr.Namespace = k
		trs = append(trs, tr)
	}
	return
}
