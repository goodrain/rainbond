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

package l4

import (
	"fmt"
	"github.com/goodrain/rainbond/gateway/annotations/parser"
	"github.com/goodrain/rainbond/gateway/annotations/resolver"
	extensions "k8s.io/api/extensions/v1beta1"
)

type Config struct {
	L4Enable bool
	L4Host   string
	L4Port   int
}

type l4 struct {
	r resolver.Resolver
}

func NewParser(r resolver.Resolver) parser.IngressAnnotation {
	return l4{r}
}

func (l l4) Parse(ing *extensions.Ingress) (interface{}, error) {
	l4Enable, _ := parser.GetBoolAnnotation("l4-enable", ing)
	l4Host, _ := parser.GetStringAnnotation("l4-host", ing)
	if l4Host == "" {
		l4Host = "0.0.0.0"
	}

	l4Port, _ := parser.GetIntAnnotation("l4-port", ing)
	if l4Enable && (l4Port <= 0 || l4Port > 65535) {
		return nil, fmt.Errorf("error l4Port: %d", l4Port)
	}
	return &Config{
		L4Enable: l4Enable,
		L4Host:   l4Host,
		L4Port:   l4Port,
	}, nil
}
