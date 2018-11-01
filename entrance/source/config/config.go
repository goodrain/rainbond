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

package config

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"

	"github.com/goodrain/rainbond/entrance/core"

	"k8s.io/api/core/v1"
)

//GainService GainService
type GainService interface {
	RePoolName()
	ReVSName()
	ReNodeName()
	ReRuleName()
	ReServiceId()
}

//SourceBranch SourceBranch
type SourceBranch struct {
	Tenant          string
	Service         string
	Domain          []string
	Port            int32
	Index           int64
	PodName         string
	ContainerPort   int32
	CertificateName string
	Method          core.EventMethod
	LBMapPort       string
	Protocol        string
	Note            string
	IsMidonet       bool
	Host            string
	State           string
	PodStatus       bool
	NodePort        int32
	Version         string
	Namespace       string
	EventID         string
	OriginPort      string
}

//ServiceInfo ServiceInfo
type ServiceInfo struct {
	Namespace string
	SerName   string
	Port      string
	Index     int64
}

type PodInfo struct {
	PodName string
	Port    string
}

const (
	VSPoolExpString string = "(.*)[@|_](.*)_([0-9]*)\\.[Pool|VS]"
	NodeExpString   string = "(.*)_([0-9]*)\\.Node"
	DomainAPIURI    string = "http://127.0.0.1:%s/v2/tenants/%s/services/%s/domains"
)

func (s *SourceBranch) RePoolName() string {
	return fmt.Sprintf("%s@%s_%d.Pool",
		s.Tenant,
		s.Service,
		s.Port,
	)
}

func (s *SourceBranch) ReVSName() string {
	return fmt.Sprintf("%s_%s_%d.VS",
		s.Tenant,
		s.Service,
		s.Port,
	)
}

func (s *SourceBranch) ReNodeName() string {
	return fmt.Sprintf("%s_%d.Node",
		s.PodName,
		s.ContainerPort,
	)
}

func sha8(s string) string {
	h := sha1.New()
	h.Write([]byte(s))
	bs := h.Sum(nil)
	return hex.EncodeToString(bs[:4])
}

func (s *SourceBranch) ReRuleName(domain string) string {
	return fmt.Sprintf("%s_%s_%d_%s.Rule",
		s.Tenant,
		s.Service,
		s.Port,
		sha8(domain),
	)
}

func (s *SourceBranch) ReServiceId() string {
	return fmt.Sprintf("%s.%s_%d",
		s.Tenant,
		s.Service,
		s.Port,
	)
}

// Operation is a type of operation of services or endpoints.
type Operation int

// These are the available operation types.
const (
	ADD Operation = iota
	UPDATE
	REMOVE
	SYNCED
)

// ServiceUpdate describes an operation of services, sent on the channel.
// You can add, update or remove single service by setting Op == ADD|UPDATE|REMOVE.
type ServiceUpdate struct {
	Service *v1.Service
	Op      Operation
}

// PodUpdate describes an operation of endpoints, sent on the channel.
// You can add, update or remove single endpoints by setting Op == ADD|UPDATE|REMOVE.
type PodUpdate struct {
	Pod *v1.Pod
	Op  Operation
}
