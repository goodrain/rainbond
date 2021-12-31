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

package v1

import (
	"github.com/goodrain/rainbond/gateway/annotations/proxy"
	"github.com/goodrain/rainbond/gateway/annotations/rewrite"
)

// ConditionType condition type
type ConditionType string

// HeaderType -
var HeaderType ConditionType = "header"

// CookieType -
var CookieType ConditionType = "cookie"

// DefaultType -
var DefaultType ConditionType = "default"

// Location -
type Location struct {
	Path          string
	NameCondition map[string]*Condition // papping between backend name and condition
	// Rewrite describes the redirection this location.
	// +optional
	Rewrite rewrite.Config `json:"rewrite,omitempty"`
	// Proxy contains information about timeouts and buffer sizes
	// to be used in connections against endpoints
	// +optional
	Proxy            proxy.Config `json:"proxy,omitempty"`
	DisableProxyPass bool
	PathRewrite      bool `json:"pathRewrite"`
}

// Condition is the condition that the traffic can reach the specified backend
type Condition struct {
	Type  ConditionType
	Value map[string]string
}

// Equals determines if two locations are equal
func (l *Location) Equals(c *Location) bool {
	if l == c {
		return true
	}
	if l == nil || c == nil {
		return false
	}
	if l.Path != c.Path {
		return false
	}

	if len(l.NameCondition) != len(c.NameCondition) {
		return false
	}
	for name, lc := range l.NameCondition {
		if cc, exists := c.NameCondition[name]; !exists || !cc.Equals(lc) {
			return false
		}
	}

	if !l.Proxy.Equal(&c.Proxy) {
		return false
	}

	if !l.Rewrite.Equal(&c.Rewrite) {
		return false
	}

	if l.PathRewrite != c.PathRewrite {
		return false
	}
	return true
}

// Equals determines if two conditions are equal
func (c *Condition) Equals(cc *Condition) bool {
	if c == cc {
		return true
	}
	if c == nil || cc == nil {
		return false
	}
	if c.Type != cc.Type {
		return false
	}

	if len(c.Value) != len(cc.Value) {
		return false
	}
	for k, v := range c.Value {
		if vv, ok := cc.Value[k]; !ok || v != vv {
			return false
		}
	}

	return true
}

func newFakeLocation() *Location {
	return &Location{
		Path: "foo-path",
	}
}
