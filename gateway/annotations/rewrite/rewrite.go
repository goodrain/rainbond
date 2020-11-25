/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rewrite

import (
	"strings"

	"github.com/goodrain/rainbond/gateway/annotations/parser"
	"github.com/goodrain/rainbond/gateway/annotations/resolver"
	"github.com/sirupsen/logrus"
	extensions "k8s.io/api/extensions/v1beta1"
)

// Config describes the per location redirect config
type Config struct {
	Rewrites []*Rewrite
	// Target URI where the traffic must be redirected
	Target string `json:"target"`
	// SSLRedirect indicates if the location section is accessible SSL only
	SSLRedirect bool `json:"sslRedirect"`
	// ForceSSLRedirect indicates if the location section is accessible SSL only
	ForceSSLRedirect bool `json:"forceSSLRedirect"`
	// AppRoot defines the Application Root that the Controller must redirect if it's in '/' context
	AppRoot string `json:"appRoot"`
	// UseRegex indicates whether or not the locations use regex paths
	UseRegex bool `json:"useRegex"`
}

// Rewrite matching request URI to replacement.
type Rewrite struct {
	Regex       string
	Replacement string
	Flag        string
}

// Equal tests for equality between two Redirect types
func (r1 *Config) Equal(r2 *Config) bool {
	if r1 == r2 {
		return true
	}
	if r1 == nil || r2 == nil {
		return false
	}
	if r1.Target != r2.Target {
		return false
	}
	if r1.SSLRedirect != r2.SSLRedirect {
		return false
	}
	if r1.ForceSSLRedirect != r2.ForceSSLRedirect {
		return false
	}
	if r1.AppRoot != r2.AppRoot {
		return false
	}
	if r1.UseRegex != r2.UseRegex {
		return false
	}

	return true
}

type rewrite struct {
	r resolver.Resolver
}

// NewParser creates a new rewrite annotation parser
func NewParser(r resolver.Resolver) parser.IngressAnnotation {
	return rewrite{r}
}

// ParseAnnotations parses the annotations contained in the ingress
// rule used to rewrite the defined paths
func (a rewrite) Parse(ing *extensions.Ingress) (interface{}, error) {
	var err error
	config := &Config{}

	rewrites, err := parser.GetStringAnnotationWithPrefix("rewrite-", ing)
	config.Rewrites = convert(rewrites)

	config.Target, _ = parser.GetStringAnnotation("rewrite-target", ing)
	config.SSLRedirect, err = parser.GetBoolAnnotation("ssl-redirect", ing)
	if err != nil {
		config.SSLRedirect = a.r.GetDefaultBackend().SSLRedirect
	}

	config.ForceSSLRedirect, err = parser.GetBoolAnnotation("force-ssl-redirect", ing)
	if err != nil {
		config.ForceSSLRedirect = a.r.GetDefaultBackend().ForceSSLRedirect
	}

	config.AppRoot, _ = parser.GetStringAnnotation("app-root", ing)
	config.UseRegex, _ = parser.GetBoolAnnotation("use-regex", ing)

	return config, nil
}

func convert(in map[string]string) []*Rewrite {
	m := make(map[string]*Rewrite)
	for k, v := range in {
		sli := strings.Split(k, "-")
		if len(sli) < 2 {
			logrus.Warningf("Invalid key: %s", k)
			continue
		}
		rewrite := m[sli[0]]
		if rewrite == nil {
			rewrite = &Rewrite{}
			m[sli[0]] = rewrite
		}
		switch sli[1] {
		case "regex":
			rewrite.Regex = v
		case "replacement":
			rewrite.Replacement = v
		case "flag":
			rewrite.Flag = v
		}
	}
	var res []*Rewrite
	for _, rw := range m {
		res = append(res, rw)
	}
	return res
}
