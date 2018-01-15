// Copyright 2013 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package exporter

import "fmt"

type matchType string

const (
	matchTypeGlob    matchType = "glob"
	matchTypeRegex   matchType = "regex"
	matchTypeDefault matchType = ""
)

func (t *matchType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var v string
	if err := unmarshal(&v); err != nil {
		return err
	}

	switch matchType(v) {
	case matchTypeRegex:
		*t = matchTypeRegex
	case matchTypeGlob, matchTypeDefault:
		*t = matchTypeGlob
	default:
		return fmt.Errorf("invalid match type %q", v)
	}
	return nil
}
