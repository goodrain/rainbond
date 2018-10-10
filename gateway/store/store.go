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

package store

import (
	"github.com/goodrain/rainbond/gateway/v1"
)

//EventMethod event method
type EventMethod string

//ADDEventMethod add method
const ADDEventMethod EventMethod = "ADD"

//UPDATEEventMethod add method
const UPDATEEventMethod EventMethod = "UPDATE"

//DELETEEventMethod add method
const DELETEEventMethod EventMethod = "DELETE"

//Storer is the interface that wraps the required methods to gather information
type Storer interface {
	GetPool(name string) *v1.Pool
	ListPool() []*v1.Pool
	GetNode(name string) *v1.Node
	ListNode() []*v1.Node
	GetHTTPRule(name string) *v1.HTTPRule
	ListHTTPRule() []*v1.HTTPRule
	GetVirtualService(name string) *v1.VirtualService
	ListVirtualService() []*v1.VirtualService
	GetSSLCert(name string) *v1.SSLCert
	ListSSLCert() []*v1.SSLCert
	PoolUpdateMethod(func(*v1.Pool, EventMethod))
	NodeUpdateMethod(func(*v1.Node, EventMethod))
	HTTPRuleUpdateMethod(func(*v1.HTTPRule, EventMethod))
	VirtualServiceUpdateMethod(func(*v1.VirtualService, EventMethod))
	SSLCertUpdateMethod(func(*v1.SSLCert, EventMethod))
	// Run initiates the synchronization of the controllers
	Run(stopCh chan struct{})
}
