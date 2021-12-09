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

package store

import (
	"sync"

	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	corev1 "k8s.io/api/core/v1"
)

//ResourceCache resource cache
type ResourceCache struct {
	lock      sync.Mutex
	resources map[string]*NamespaceResource
}

//NewResourceCache new resource cache
func NewResourceCache() *ResourceCache {
	return &ResourceCache{
		resources: make(map[string]*NamespaceResource),
	}
}

//NamespaceResource namespace resource
type NamespaceResource map[string]*v1.PodResource

//SetPodResource set pod resource
func (r *NamespaceResource) SetPodResource(podName string, pr *v1.PodResource) {
	(*r)[podName] = pr
}

//RemovePod remove pod resource
func (r *NamespaceResource) RemovePod(podName string) {
	delete(*r, podName)
}

//TenantResource tenant resource
type TenantResource struct {
	Namespace     string
	MemoryRequest int64
	MemoryLimit   int64
	CPURequest    int64
	CPULimit      int64
}

//SetPodResource set pod resource
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

//RemovePod remove pod resource
func (r *ResourceCache) RemovePod(pod *corev1.Pod) {
	r.lock.Lock()
	defer r.lock.Unlock()
	namespace := pod.Namespace
	if nr, ok := r.resources[namespace]; ok && nr != nil {
		nr.RemovePod(pod.Name)
	}
}

//GetTenantResource get tenant resource
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

//GetAllTenantResource get all tenant resources
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
