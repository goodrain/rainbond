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

package discovery

import (
	"strings"
)

// Discoverier blablabla
type Discoverier interface {
	Connect() error
	Fetch() ([]*Endpoint, error)
	Close() error
}

// NewDiscoverier creates a new Discoverier
func NewDiscoverier(info *Info) Discoverier {
	switch strings.ToUpper(info.Type) {
	case "ETCD":
		return NewEtcd(info)
	}
	return nil
}

// Info holds service discovery center information.
type Info struct {
	Type     string   `json:"type"`
	Servers  []string `json:"servers"`
	Key      string   `json:"key"`
	Username string   `json:"username"`
	Password string   `json:"password"`
}

// Endpoint holds endpoint and endpoint status(online or offline).
type Endpoint struct {
	Ep       string `json:"endpoint"`
	IsOnline bool   `json:"is_online"`
}
