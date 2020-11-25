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

package weight

import (
	"github.com/goodrain/rainbond/gateway/annotations/parser"
	"github.com/goodrain/rainbond/gateway/annotations/resolver"
	"github.com/sirupsen/logrus"
	extensions "k8s.io/api/extensions/v1beta1"
	"strconv"
)

// Config contains weight or router
type Config struct {
	Weight int
}

type weight struct {
	r resolver.Resolver
}

// NewParser creates a new parser
func NewParser(r resolver.Resolver) parser.IngressAnnotation {
	return weight{r}
}

func (c weight) Parse(ing *extensions.Ingress) (interface{}, error) {
	wstr, err := parser.GetStringAnnotation("weight", ing)
	var w int
	if err != nil || wstr == "" {
		w = 1
	} else {
		w, err = strconv.Atoi(wstr)
		if err != nil {
			logrus.Warnf("Unexpected error occurred when convert string(%s) to int: %v", wstr, err)
			w = 1
		}
	}
	return &Config{
		Weight: w,
	}, nil
}
