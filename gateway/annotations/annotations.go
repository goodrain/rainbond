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

package annotations

import (
	"github.com/goodrain/rainbond/gateway/annotations/cookie"
	"github.com/goodrain/rainbond/gateway/annotations/header"
	"github.com/goodrain/rainbond/gateway/annotations/l4"
	"github.com/goodrain/rainbond/gateway/annotations/lbtype"
	"github.com/goodrain/rainbond/gateway/annotations/parser"
	"github.com/goodrain/rainbond/gateway/annotations/proxy"
	"github.com/goodrain/rainbond/gateway/annotations/resolver"
	"github.com/goodrain/rainbond/gateway/annotations/rewrite"
	"github.com/goodrain/rainbond/gateway/annotations/upstreamhashby"
	weight "github.com/goodrain/rainbond/gateway/annotations/wight"
	"github.com/goodrain/rainbond/util/ingress-nginx/ingress/errors"
	"github.com/imdario/mergo"
	"github.com/sirupsen/logrus"
	extensions "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeniedKeyName name of the key that contains the reason to deny a location
const DeniedKeyName = "Denied"

// Ingress defines the valid annotations present in one NGINX Ingress rule
type Ingress struct {
	metav1.ObjectMeta
	Header            header.Config
	Cookie            cookie.Config
	Weight            weight.Config
	Rewrite           rewrite.Config
	L4                l4.Config
	UpstreamHashBy    string
	LoadBalancingType string
	Proxy             proxy.Config
}

// Extractor defines the annotation parsers to be used in the extraction of annotations
type Extractor struct {
	annotations map[string]parser.IngressAnnotation
}

// NewAnnotationExtractor creates a new annotations extractor
func NewAnnotationExtractor(cfg resolver.Resolver) Extractor {
	return Extractor{
		map[string]parser.IngressAnnotation{
			"Header":            header.NewParser(cfg),
			"Cookie":            cookie.NewParser(cfg),
			"Weight":            weight.NewParser(cfg),
			"Rewrite":           rewrite.NewParser(cfg),
			"L4":                l4.NewParser(cfg),
			"UpstreamHashBy":    upstreamhashby.NewParser(cfg),
			"LoadBalancingType": lbtype.NewParser(cfg),
			"Proxy":             proxy.NewParser(cfg),
		},
	}
}

// Extract extracts the annotations from an Ingress
func (e Extractor) Extract(ing *extensions.Ingress) *Ingress {
	pia := &Ingress{
		ObjectMeta: ing.ObjectMeta,
	}

	data := make(map[string]interface{})
	for name, annotationParser := range e.annotations {
		val, err := annotationParser.Parse(ing)
		if err != nil {
			if errors.IsMissingAnnotations(err) {
				continue
			}

			if !errors.IsLocationDenied(err) {
				continue
			}

			_, alreadyDenied := data[DeniedKeyName]
			if !alreadyDenied {
				data[DeniedKeyName] = err
				logrus.Errorf("error reading %v annotation in Ingress %v/%v: %v", name, ing.GetNamespace(), ing.GetName(), err)
				continue
			}

			logrus.Infof("error reading %v annotation in Ingress %v/%v: %v", name, ing.GetNamespace(), ing.GetName(), err)
		}

		if val != nil {
			data[name] = val
		}
	}

	err := mergo.MapWithOverwrite(pia, data)
	if err != nil {
		logrus.Errorf("unexpected error merging extracted annotations: %v", err)
	}

	return pia
}
