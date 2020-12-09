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

	"github.com/crossplane/oam-kubernetes-runtime/apis/core/v1alpha2"
	"github.com/goodrain/rainbond-oam/pkg/ram/v1alpha1"

	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/resource"
)

//NewMemoryQuantity new memory quantity
func NewMemoryQuantity(memory int) resource.Quantity {
	rq, err := resource.ParseQuantity(fmt.Sprintf("%dMi", memory))
	if err != nil {
		logrus.Warningf("parse memory quantity failure %s", err.Error())
	}
	return rq
}

//NewCPUQuantity new cpu quantity
func NewCPUQuantity(cpu int) resource.Quantity {
	rq, err := resource.ParseQuantity(fmt.Sprintf("%d", cpu))
	if err != nil {
		logrus.Warningf("parse cpu quantity failure %s", err.Error())
	}
	return rq
}

//NewDiskQuantity new disk quantity
func NewDiskQuantity(disk int) resource.Quantity {
	rq, err := resource.ParseQuantity(fmt.Sprintf("%dGi", disk))
	if err != nil {
		logrus.Warningf("parse cpu quantity failure %s", err.Error())
	}
	return rq
}

//NewVolumeAccess new volume access
func NewVolumeAccess(va v1alpha1.AccessMode) *v1alpha2.VolumeAccessMode {
	var ro = v1alpha2.VolumeAccessModeRO
	var rw = v1alpha2.VolumeAccessModeRW
	switch va {
	case v1alpha1.ROXAccessMode:
		return &ro
	case v1alpha1.RWOAccessMode:
		return &rw
	case v1alpha1.RWXAccessMode:
		return &rw
	default:
		return &rw
	}
}

//NewSharingPolicy new sharing policy
func NewSharingPolicy(sp string) *v1alpha2.VolumeSharingPolicy {
	var share = v1alpha2.VolumeSharingPolicyShared
	var exclusive = v1alpha2.VolumeSharingPolicyExclusive
	switch sp {
	case "Shared":
		return &share
	case "Exclusive":
		return &exclusive
	default:
		return &share
	}
}

//NewTransportProtocol -
func NewTransportProtocol(protocol string) *v1alpha2.TransportProtocol {
	var udp = v1alpha2.TransportProtocolUDP
	var tcp = v1alpha2.TransportProtocolTCP
	switch strings.ToLower(protocol) {
	case "udp":
		return &udp
	default:
		return &tcp
	}
}

//Uint32 -
func Uint32(s int) *uint32 {
	var ss = uint32(s)
	return &ss
}

//Int32 -
func Int32(s int) *int32 {
	var ss = int32(s)
	return &ss
}
