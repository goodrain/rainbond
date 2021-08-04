/*
Copyright 2015 The Kubernetes Authors.

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

package parser

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/goodrain/rainbond/util/ingress-nginx/ingress/errors"
	networkingv1 "k8s.io/api/networking/v1"
)

var (
	// AnnotationsPrefix defines the common prefix used in the nginx ingress controller
	AnnotationsPrefix = "nginx.ingress.kubernetes.io"
)

// IngressAnnotation has a method to parse annotations located in Ingress
type IngressAnnotation interface {
	Parse(ing *networkingv1.Ingress) (interface{}, error)
}

type ingAnnotations map[string]string

func (a ingAnnotations) parseBool(name string) (bool, error) {
	val, ok := a[name]
	if ok {
		b, err := strconv.ParseBool(val)
		if err != nil {
			return false, errors.NewInvalidAnnotationContent(name, val)
		}
		return b, nil
	}
	return false, errors.ErrMissingAnnotations
}

func (a ingAnnotations) parseString(name string) (string, error) {
	val, ok := a[name]
	if ok {
		return val, nil
	}
	return "", errors.ErrMissingAnnotations
}

func (a ingAnnotations) parseInt(name string) (int, error) {
	val, ok := a[name]
	if ok {
		i, err := strconv.Atoi(val)
		if err != nil {
			return 0, errors.NewInvalidAnnotationContent(name, val)
		}
		return i, nil
	}
	return 0, errors.ErrMissingAnnotations
}

func checkAnnotation(name string, ing *networkingv1.Ingress) error {
	if ing == nil || len(ing.GetAnnotations()) == 0 {
		return errors.ErrMissingAnnotations
	}
	if name == "" {
		return errors.ErrInvalidAnnotationName
	}

	return nil
}

// GetBoolAnnotation extracts a boolean from an Ingress annotation
func GetBoolAnnotation(name string, ing *networkingv1.Ingress) (bool, error) {
	v := GetAnnotationWithPrefix(name)
	err := checkAnnotation(v, ing)
	if err != nil {
		return false, err
	}
	return ingAnnotations(ing.GetAnnotations()).parseBool(v)
}

// GetStringAnnotation extracts a string from an Ingress annotation
func GetStringAnnotation(name string, ing *networkingv1.Ingress) (string, error) {
	v := GetAnnotationWithPrefix(name)
	err := checkAnnotation(v, ing)
	if err != nil {
		return "", err
	}
	return ingAnnotations(ing.GetAnnotations()).parseString(v)
}

// GetIntAnnotation extracts an int from an Ingress annotation
func GetIntAnnotation(name string, ing *networkingv1.Ingress) (int, error) {
	v := GetAnnotationWithPrefix(name)
	err := checkAnnotation(v, ing)
	if err != nil {
		return 0, err
	}
	return ingAnnotations(ing.GetAnnotations()).parseInt(v)
}

// GetStringAnnotationWithPrefix extracts an string from an Ingress annotation
// based on the annotation prefix
func GetStringAnnotationWithPrefix(prefix string, ing *networkingv1.Ingress) (map[string]string, error) {
	v := GetAnnotationWithPrefix(prefix)
	err := checkAnnotation(v, ing)
	if err != nil {
		return nil, err
	}
	anns := ing.GetAnnotations()
	res := make(map[string]string)
	for key, val := range anns {
		if !strings.HasPrefix(key, v) {
			continue
		}
		k := strings.Replace(key, v, "", 1)
		if k != "" {
			res[k] = val
		}
	}
	return res, nil
}

// GetAnnotationWithPrefix returns the prefix of ingress annotations
func GetAnnotationWithPrefix(suffix string) string {
	return fmt.Sprintf("%v/%v", AnnotationsPrefix, suffix)
}
