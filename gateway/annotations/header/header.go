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

package header

import (
	"github.com/goodrain/rainbond/gateway/annotations/parser"
	"github.com/goodrain/rainbond/gateway/annotations/resolver"
	networkingv1 "k8s.io/api/networking/v1"
	"strings"
)

// Config -
type Config struct {
	Header map[string]string `json:"header"`
}

type header struct {
	r resolver.Resolver
}

// NewParser -
func NewParser(r resolver.Resolver) parser.IngressAnnotation {
	return header{r}
}

func (h header) Parse(ing *networkingv1.Ingress) (interface{}, error) {
	hr, err := parser.GetStringAnnotation("header", ing)
	if err != nil {
		return nil, err
	}
	hmap := transform(hr)

	return &Config{
		Header: hmap,
	}, nil
}

// transform transfers string to map
func transform(target string) map[string]string {
	target = strings.Replace(target, " ", "", -1)
	result := make(map[string]string)
	for _, item := range strings.Split(target, ";") {
		split := strings.Split(item, "=")
		if len(split) < 2 || split[0] == "" || split[1] == "" {
			continue
		}
		result[split[0]] = split[1]
	}
	if len(result) == 0 {
		return nil
	}

	return result
}
