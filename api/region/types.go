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

package region

//ServiceDeployInfo service deploy info
type ServiceDeployInfo struct {
	Namespace    string            `protobuf:"bytes,1,opt,name=namespace,proto3" json:"namespace,omitempty"`
	Statefuleset string            `protobuf:"bytes,2,opt,name=statefuleset,proto3" json:"statefuleset,omitempty"`
	Deployment   string            `protobuf:"bytes,3,opt,name=deployment,proto3" json:"deployment,omitempty"`
	Pods         map[string]string `protobuf:"bytes,4,rep,name=pods,proto3" json:"pods,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	Services     map[string]string `protobuf:"bytes,5,rep,name=services,proto3" json:"services,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	Secrets      map[string]string `protobuf:"bytes,6,rep,name=secrets,proto3" json:"secrets,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	Ingresses    map[string]string `protobuf:"bytes,7,rep,name=ingresses,proto3" json:"ingresses,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	Replicatset  map[string]string `protobuf:"bytes,8,rep,name=replicatset,proto3" json:"replicatset,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	Status       string            `protobuf:"bytes,9,opt,name=status,proto3" json:"status,omitempty"`
}
