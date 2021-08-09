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

package lbtype

import (
	"github.com/goodrain/rainbond/gateway/annotations/parser"
	"github.com/goodrain/rainbond/gateway/annotations/resolver"
	networkingv1 "k8s.io/api/networking/v1"
)

type lbtype struct {
	r resolver.Resolver
}

// NewParser creates a new CORS annotation parser
func NewParser(r resolver.Resolver) parser.IngressAnnotation {
	return lbtype{r}
}

// Parse parses the annotations contained in the ingress rule
// used to indicate if the location/s contains a fragment of
// configuration to be included inside the paths of the rules
func (a lbtype) Parse(ing *networkingv1.Ingress) (interface{}, error) {
	return parser.GetStringAnnotation("lb-type", ing)
}
