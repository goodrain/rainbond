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

package cookie

import (
	"github.com/goodrain/rainbond/gateway/annotations/parser"
	"github.com/goodrain/rainbond/gateway/annotations/resolver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

// Config -
type Config struct {
	Cookie map[string]string `json:"cookie"`
}

type cookie struct {
	r resolver.Resolver
}

// NewParser -
func NewParser(r resolver.Resolver) parser.IngressAnnotation {
	return cookie{r}
}

func (c cookie) Parse(meta *metav1.ObjectMeta) (interface{}, error) {
	co, err := parser.GetStringAnnotation("cookie", meta)
	if err != nil {
		return nil, err
	}
	hmap := transform(co)

	return &Config{
		Cookie: hmap,
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
