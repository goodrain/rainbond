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

package compose

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	libcomposeyaml "github.com/docker/libcompose/yaml"
	"github.com/sirupsen/logrus"
	"github.com/twinj/uuid"

	"github.com/docker/cli/cli/compose/loader"
	"github.com/docker/cli/cli/compose/types"

	"os"

	"github.com/goodrain/rainbond/util"
)

// converts os.Environ() ([]string) to map[string]string
// based on https://github.com/docker/cli/blob/5dd30732a23bbf14db1c64d084ae4a375f592cfa/cli/command/stack/deploy_composefile.go#L143
func buildEnvironment() (map[string]string, error) {
	env := os.Environ()
	result := make(map[string]string, len(env))
	for _, s := range env {
		// if value is empty, s is like "K=", not "K".
		if !strings.Contains(s, "=") {
			return result, fmt.Errorf("unexpected environment %q", s)
		}
		kv := strings.SplitN(s, "=", 2)
		result[kv[0]] = kv[1]
	}
	return result, nil
}

// The purpose of this is not to deploy, but to be able to parse
// v3 of Docker Compose into a suitable format. In this case, whatever is returned
// by docker/cli's ServiceConfig
func parseV3(bodys [][]byte) (ComposeObject, error) {

	// In order to get V3 parsing to work, we have to go through some preliminary steps
	// for us to hack up github.com/docker/cli in order to correctly convert to a ComposeObject
	cacheName := uuid.NewV4().String()
	if err := util.CheckAndCreateDir("/cache/docker-compose/" + cacheName); err != nil {
		return ComposeObject{}, fmt.Errorf("create cache docker compose file dir error:%s", err.Error())
	}
	var files []string
	for i, body := range bodys {
		filename := fmt.Sprintf("/cache/docker-compose/%s/%d-docker-compose.yml", cacheName, i)
		if err := ioutil.WriteFile(filename, body, 0755); err != nil {
			return ComposeObject{}, fmt.Errorf("write cache docker compose file error:%s", err.Error())
		}
		files = append(files, filename)
	}
	// Gather the working directory
	workingDir := "/cache/docker-compose/" + cacheName

	// Parse the Compose File
	parsedComposeFile, err := loader.ParseYAML(bodys[0])
	if err != nil {
		return ComposeObject{}, err
	}

	// Config file
	configFile := types.ConfigFile{
		Filename: files[0],
		Config:   parsedComposeFile,
	}

	// get environment variables
	env, err := buildEnvironment()
	if err != nil {
		return ComposeObject{}, fmt.Errorf("cannot build environment variables")
	}

	// Config details
	configDetails := types.ConfigDetails{
		WorkingDir:  workingDir,
		ConfigFiles: []types.ConfigFile{configFile},
		Environment: env,
	}

	// Actual config
	// We load it in order to retrieve the parsed output configuration!
	// This will output a github.com/docker/cli ServiceConfig
	// Which is similar to our version of ServiceConfig
	config, err := loader.Load(configDetails)
	if err != nil {
		return ComposeObject{}, err
	}

	// TODO: Check all "unsupported" keys and output details
	// Specifically, keys such as "volumes_from" are not supported in V3.

	// Finally, we convert the object from docker/cli's ServiceConfig to our appropriate one
	komposeObject, err := dockerComposeToKomposeMapping(config)
	if err != nil {
		return ComposeObject{}, err
	}

	return komposeObject, nil
}

// Convert the Docker Compose v3 volumes to []string (the old way)
// TODO: Check to see if it's a "bind" or "volume". Ignore for now.
// TODO: Refactor it similar to loadV3Ports
// See: https://docs.docker.com/compose/compose-file/#long-syntax-2
func loadV3Volumes(volumes []types.ServiceVolumeConfig) []string {
	var volArray []string
	for _, vol := range volumes {

		// There will *always* be Source when parsing
		v := normalizeServiceNames(vol.Source)

		if vol.Target != "" {
			v = v + ":" + vol.Target
		}

		if vol.ReadOnly {
			v = v + ":ro"
		}

		volArray = append(volArray, v)
	}
	return volArray
}

// Convert Docker Compose v3 ports to Ports
func loadV3Ports(ports []types.ServicePortConfig) []Ports {
	komposePorts := []Ports{}

	for _, port := range ports {

		// Convert to a kobject struct with ports
		// NOTE: V3 doesn't use IP (they utilize Swarm instead for host-networking).
		// Thus, IP is blank.
		komposePorts = append(komposePorts, Ports{
			HostPort:      int32(port.Published),
			ContainerPort: int32(port.Target),
			HostIP:        "",
			Protocol:      strings.ToUpper(string(port.Protocol)),
		})

	}

	return komposePorts
}

func dockerComposeToKomposeMapping(composeObject *types.Config) (ComposeObject, error) {

	// Step 1. Initialize what's going to be returned
	co := ComposeObject{
		ServiceConfigs: make(map[string]ServiceConfig),
	}

	// Step 2. Parse through the object and convert it to ComposeObject!
	// Here we "clean up" the service configuration so we return something that includes
	// all relevant information as well as avoid the unsupported keys as well.
	for _, composeServiceConfig := range composeObject.Services {

		// Standard import
		// No need to modify before importation
		name := composeServiceConfig.Name
		logrus.Debugf("compose 3 service conf name is %s", name)
		serviceConfig := ServiceConfig{}
		serviceConfig.ContainerName = composeServiceConfig.ContainerName
		if serviceConfig.ContainerName == "" {
			serviceConfig.ContainerName = name
		}
		serviceConfig.Image = composeServiceConfig.Image
		serviceConfig.WorkingDir = composeServiceConfig.WorkingDir
		serviceConfig.Annotations = map[string]string(composeServiceConfig.Labels)
		serviceConfig.CapAdd = composeServiceConfig.CapAdd
		serviceConfig.CapDrop = composeServiceConfig.CapDrop
		serviceConfig.Expose = composeServiceConfig.Expose
		serviceConfig.Privileged = composeServiceConfig.Privileged
		serviceConfig.User = composeServiceConfig.User
		serviceConfig.Stdin = composeServiceConfig.StdinOpen
		serviceConfig.Tty = composeServiceConfig.Tty
		serviceConfig.TmpFs = composeServiceConfig.Tmpfs
		serviceConfig.Command = composeServiceConfig.Entrypoint
		serviceConfig.Args = composeServiceConfig.Command
		serviceConfig.Labels = composeServiceConfig.Labels
		serviceConfig.DependsON = composeServiceConfig.DependsOn
		serviceConfig.Links = composeServiceConfig.Links
		//
		// Deploy keys
		//

		// mode:
		serviceConfig.DeployMode = composeServiceConfig.Deploy.Mode

		if (composeServiceConfig.Deploy.Resources != types.Resources{}) {

			// memory:
			// TODO: Refactor yaml.MemStringorInt in go to int64
			// Since Deploy.Resources.Limits does not initialize, we must check type Resources before continuing
			serviceConfig.MemLimit = libcomposeyaml.MemStringorInt(composeServiceConfig.Deploy.Resources.Limits.MemoryBytes)
			serviceConfig.MemReservation = libcomposeyaml.MemStringorInt(composeServiceConfig.Deploy.Resources.Reservations.MemoryBytes)

			// cpu:
			// convert to k8s format, for example: 0.5 = 500m
			// See: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
			// "The expression 0.1 is equivalent to the expression 100m, which can be read as “one hundred millicpu”."

			cpuLimit, err := strconv.ParseFloat(composeServiceConfig.Deploy.Resources.Limits.NanoCPUs, 64)
			if err != nil {
				return ComposeObject{}, fmt.Errorf("Unable to convert cpu limits resources value")
			}
			serviceConfig.CPULimit = int64(cpuLimit * 1000)

			cpuReservation, err := strconv.ParseFloat(composeServiceConfig.Deploy.Resources.Reservations.NanoCPUs, 64)
			if err != nil {
				return ComposeObject{}, fmt.Errorf("Unable to convert cpu limits reservation value")
			}
			serviceConfig.CPUReservation = int64(cpuReservation * 1000)

		}

		// restart-policy:
		if composeServiceConfig.Deploy.RestartPolicy != nil {
			serviceConfig.Restart = composeServiceConfig.Deploy.RestartPolicy.Condition
		}

		// replicas:
		if composeServiceConfig.Deploy.Replicas != nil {
			serviceConfig.Replicas = int(*composeServiceConfig.Deploy.Replicas)
		}

		// placement:
		// placement := make(map[string]string)
		// for _, j := range composeServiceConfig.Deploy.Placement.Constraints {
		// 	p := strings.Split(j, " == ")
		// 	if p[0] == "node.hostname" {
		// 		placement["kubernetes.io/hostname"] = p[1]
		// 	} else if p[0] == "engine.labels.operatingsystem" {
		// 		placement["beta.kubernetes.io/os"] = p[1]
		// 	} else {
		// 		logrus.Warn(p[0], " constraints in placement is not supported, only 'node.hostname' and 'engine.labels.operatingsystem' is only supported as a constraint ")
		// 	}
		// }
		// serviceConfig.Placement = placement

		// TODO: Build is not yet supported, see:
		// https://github.com/docker/cli/blob/master/cli/compose/types/types.go#L9
		// We will have to *manually* add this / parse.
		// serviceConfig.Build = composeServiceConfig.Build.Context
		// serviceConfig.Dockerfile = composeServiceConfig.Build.Dockerfile
		// serviceConfig.BuildArgs = composeServiceConfig.Build.Args

		// Gather the environment values
		// DockerCompose uses map[string]*string while we use []string
		// So let's convert that using this hack
		for name, value := range composeServiceConfig.Environment {
			env := EnvVar{Name: name, Value: *value}
			serviceConfig.Environment = append(serviceConfig.Environment, env)
		}

		// Get env_file
		serviceConfig.EnvFile = composeServiceConfig.EnvFile

		// Parse the ports
		// v3 uses a new format called "long syntax" starting in 3.2
		// https://docs.docker.com/compose/compose-file/#ports
		serviceConfig.Port = loadV3Ports(composeServiceConfig.Ports)

		// Parse the volumes
		// Again, in v3, we use the "long syntax" for volumes in terms of parsing
		// https://docs.docker.com/compose/compose-file/#long-syntax-2
		serviceConfig.VolList = loadV3Volumes(composeServiceConfig.Volumes)

		// Label handler
		// Labels used to influence conversion of kompose will be handled
		// from here for docker-compose. Each loader will have such handler.
		for key, value := range composeServiceConfig.Labels {
			switch key {
			case "kompose.service.type":
				serviceType, err := handleServiceType(value)
				if err != nil {
					return ComposeObject{}, fmt.Errorf("handleServiceType failed")
				}

				serviceConfig.ServiceType = serviceType
			case "kompose.service.expose":
				serviceConfig.ExposeService = strings.ToLower(value)
			case "kompose.service.expose.tls-secret":
				serviceConfig.ExposeServiceTLS = value
			}
		}

		if serviceConfig.ExposeService == "" && serviceConfig.ExposeServiceTLS != "" {
			return ComposeObject{}, fmt.Errorf("kompose.service.expose.tls-secret was specified without kompose.service.expose")
		}

		// Log if the name will been changed
		if normalizeServiceNames(name) != name {
			logrus.Infof("Service name in docker-compose has been changed from %q to %q", name, normalizeServiceNames(name))
		}

		// Final step, add to the array!
		co.ServiceConfigs[normalizeServiceNames(name)] = serviceConfig
	}
	handleVolume(&co)

	return co, nil
}
