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

package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/shunfei/cronsun/log"
	"k8s.io/kubernetes/pkg/api"
)

// load environment variables from compose file
func loadEnvVars(envars []string) []EnvVar {
	envs := []EnvVar{}
	for _, e := range envars {
		character := ""
		equalPos := strings.Index(e, "=")
		colonPos := strings.Index(e, ":")
		switch {
		case equalPos == -1 && colonPos == -1:
			character = ""
		case equalPos == -1 && colonPos != -1:
			character = ":"
		case equalPos != -1 && colonPos == -1:
			character = "="
		case equalPos != -1 && colonPos != -1:
			if equalPos > colonPos {
				character = ":"
			} else {
				character = "="
			}
		}

		if character == "" {
			envs = append(envs, EnvVar{
				Name:  e,
				Value: os.Getenv(e),
			})
		} else {
			values := strings.SplitN(e, character, 2)
			// try to get value from os env
			if values[1] == "" {
				values[1] = os.Getenv(values[0])
			}
			envs = append(envs, EnvVar{
				Name:  values[0],
				Value: values[1],
			})
		}
	}

	return envs
}

// getComposeFileDir returns compose file directory
// Assume all the docker-compose files are in the same directory
// TODO: fix (check if file exists)
func getComposeFileDir(inputFiles []string) (string, error) {
	inputFile := inputFiles[0]
	if strings.Index(inputFile, "/") != 0 {
		workDir, err := os.Getwd()
		if err != nil {
			return "", errors.Wrap(err, "Unable to retrieve compose file directory")
		}
		inputFile = filepath.Join(workDir, inputFile)
	}
	return filepath.Dir(inputFile), nil
}

func handleServiceType(ServiceType string) (string, error) {
	switch strings.ToLower(ServiceType) {
	case "", "clusterip":
		return string(api.ServiceTypeClusterIP), nil
	case "nodeport":
		return string(api.ServiceTypeNodePort), nil
	case "loadbalancer":
		return string(api.ServiceTypeLoadBalancer), nil
	default:
		return "", errors.New("Unknown value " + ServiceType + " , supported values are 'NodePort, ClusterIP or LoadBalancer'")
	}
}

func normalizeServiceNames(svcName string) string {
	return strings.Replace(svcName, "_", "-", -1)
}

// ParseVolume parses a given volume, which might be [name:][host:]container[:access_mode]
func ParseVolume(volume string) (name, host, container, mode string, err error) {
	separator := ":"

	// Parse based on ":"
	volumeStrings := strings.Split(volume, separator)
	if len(volumeStrings) == 0 {
		return
	}

	// Set name if existed
	if !isPath(volumeStrings[0]) {
		name = volumeStrings[0]
		volumeStrings = volumeStrings[1:]
	}

	// Check if *anything* has been passed
	if len(volumeStrings) == 0 {
		err = fmt.Errorf("invalid volume format: %s", volume)
		return
	}

	// Get the last ":" passed which is presumingly the "access mode"
	possibleAccessMode := volumeStrings[len(volumeStrings)-1]

	// Check to see if :Z or :z exists. We do not support SELinux relabeling at the moment.
	// See https://github.com/kubernetes/kompose/issues/176
	// Otherwise, check to see if "rw" or "ro" has been passed
	if possibleAccessMode == "z" || possibleAccessMode == "Z" {
		log.Warnf("Volume mount \"%s\" will be mounted without labeling support. :z or :Z not supported", volume)
		mode = ""
		volumeStrings = volumeStrings[:len(volumeStrings)-1]
	} else if possibleAccessMode == "rw" || possibleAccessMode == "ro" {
		mode = possibleAccessMode
		volumeStrings = volumeStrings[:len(volumeStrings)-1]
	}

	// Check the volume format as well as host
	container = volumeStrings[len(volumeStrings)-1]
	volumeStrings = volumeStrings[:len(volumeStrings)-1]
	if len(volumeStrings) == 1 {
		host = volumeStrings[0]
	}
	if !isPath(container) || (len(host) > 0 && !isPath(host)) || len(volumeStrings) > 1 {
		err = fmt.Errorf("invalid volume format: %s", volume)
		return
	}
	return
}

func isPath(substring string) bool {
	return strings.Contains(substring, "/") || substring == "."
}
