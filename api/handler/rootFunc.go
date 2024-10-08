// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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

package handler

import (
	"fmt"
	"strconv"
	"strings"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/sirupsen/logrus"
)

// RootAction  root function action struct
type RootAction struct{}

// CreateRootFuncManager get root func manager
func CreateRootFuncManager() *RootAction {
	return &RootAction{}
}

// VersionInfo VersionInfo
type VersionInfo struct {
	Version []*LangInfo `json:"version"`
}

// LangInfo LangInfo
type LangInfo struct {
	Lang  string `json:"lang"`
	Major []*MajorInfo
}

// MajorInfo MajorInfo
type MajorInfo struct {
	Major int `json:"major"`
	Minor []*MinorInfo
}

// MinorInfo MinorInfo
type MinorInfo struct {
	Minor int   `json:"minor"`
	Patch []int `json:"patch"`
}

//{"php":{"3":{"4":3, "5":2}, "4":5}}
//MinorVersion := make(map[string]int)
//MajorVersion := make(map[string]minorVersion)
//ListVersion := make(map[string](make(map[string](make(map[string]int)))))

// ResolvePHP php 应用构建
func (r *RootAction) ResolvePHP(cs *apimodel.ComposerStruct) (string, error) {
	lang := cs.Body.Lang
	data := cs.Body.Data
	logrus.Debugf("Composer got default_runtime=%v, json body=%v", lang, data)
	jsonData := cs.Body.Data.JSON
	if cs.Body.Data.JSON.PlatForm.PHP == "" {
		jsonData = cs.Body.Data.Lock
	}
	listVersions, err := createListVersion(cs)
	if err != nil {
		return "", err
	}
	isphp := jsonData.PlatForm.PHP
	if isphp != "" {
		var maxVersion string
		var errV error
		if strings.HasPrefix(isphp, "~") {
			si := strings.Split(isphp, "~")
			mm := strings.Split(si[1], ".")
			major, minor, _, err := transAtoi(mm)
			if err != nil {
				return "", err
			}
			maxVersion, errV = getMaxVersion(lang, &listVersions, major, minor)
			if errV != nil {
				return "", errV
			}
		} else if strings.HasPrefix(isphp, ">=") {
			si := strings.Split(isphp, ">=")
			mm := strings.Split(si[1], ".")
			major, minor, _, err := transAtoi(mm)
			if err != nil {
				return "", err
			}
			maxVersion, errV = getMaxVersion(lang, &listVersions, major, minor)
			if errV != nil {
				return "", errV
			}
		} else {
			mm := strings.Split(isphp, ".")
			major, minor, patch, err := transAtoi(mm)
			if err != nil {
				return "", err
			}
			maxVersion, errV = getMaxVersion(lang, &listVersions, major, minor, patch)
			if errV != nil {
				return "", errV
			}
		}
		return fmt.Sprintf("{%s|composer.json|%s|%s", lang, isphp, maxVersion), nil
	}
	maxVersion, errM := getMaxVersion(lang, &listVersions)
	if errM != nil {
		return "", errM
	}
	return fmt.Sprintf("{%s|default|*|%s}", lang, maxVersion), nil
}

func createListVersion(cs *apimodel.ComposerStruct) (map[string]*VersionInfo, error) {
	//listVersions := make(map[string]*VersionInfo)
	/*
		listVersions := make(map[string]interface{})
		var vi VersionInfo
		for _, p := range cs.Body.Data.Packages {
			mm := strings.Split(p, "-")
			name := mm[0]
			version := mm[1]
			var li LangInfo
			mp := strings.Split(version, ".")
			major, minor, patch, errT := transAtoi(mp)
			if errT != nil {
				return nil, errT
			}

				if _, ok := listVersions[name]; !ok {
					var nn VersionInfo
					listVersions[name] = &nn
				}
				mp := strings.Split(version, ".")
				major, minor, patch, errT := transAtoi(mp)
				if errT != nil {
					return nil, errT
				}
				listVersions[name].Majon = major
				listVersions[name].Minor = minor
				listVersions[name].Patch = patch
		}*/
	return nil, nil
}

func transAtoi(mm []string) (int, int, int, error) {
	major, minor, patch := 0, 0, 0
	major, errMa := strconv.Atoi(mm[0])
	if errMa != nil {
		return 0, 0, 0, errMa
	}
	minor, errMi := strconv.Atoi(mm[1])
	if errMi != nil {
		return 0, 0, 0, errMi
	}
	if len(mm) == 3 {
		var err error
		patch, err = strconv.Atoi(mm[2])
		if err != nil {
			return 0, 0, 0, err
		}
	} else if len(mm) == 2 {
		patch = 0
	} else {
		return 0, 0, 0, fmt.Errorf("version length error")
	}
	return major, minor, patch, nil
}

func getMaxVersion(l string, lv *map[string]*VersionInfo, opts ...int) (string, error) {

	return "", nil
}
