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

package svclink

// ServiceList linked list of services, used to find closed loops in linked lists.
type ServiceList struct {
	head *service
}

// service linked list node about service
type service struct {
	sid  string
	next *service
}

// New creates a new ServiceList.
func New(sid string) *ServiceList {
	return &ServiceList{
		head: &service{
			sid: sid,
		},
	}
}

func (s *ServiceList) Add(sid string) {
	svc := &service{sid: sid}
	if s.head == nil {
		s.head = svc
		return
	}
}
