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
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/docker/libcompose/project"
	"github.com/sirupsen/logrus"
)

// Parse Docker Compose with libcompose (only supports v1 and v2). Eventually we will
// switch to using only libcompose once v3 is supported.
func parseV1V2(bodys [][]byte) (ComposeObject, error) {

	// Gather the appropriate context for parsing
	context := &project.Context{
		ProjectName: "rainbond-dockercompose-project",
	}
	context.ComposeBytes = bodys

	// Load the context and let's start parsing
	composeObject := project.NewProject(context, nil, nil)
	err := composeObject.Parse()
	if err != nil {
		return ComposeObject{}, fmt.Errorf("composeObject.Parse() failed, Failed to load compose file,%s", err)
	}

	noSupKeys := checkUnsupportedKey(composeObject)
	for _, keyName := range noSupKeys {
		logrus.Warningf("Unsupported %s key - ignoring", keyName)
	}
	// Map the parsed struct to a struct we understand (kobject)
	co, err := libComposeToKomposeMapping(composeObject)
	if err != nil {
		return ComposeObject{}, err
	}
	return co, nil
}

// Load ports from compose file
func loadPorts(composePorts []string) ([]Ports, error) {
	ports := []Ports{}
	character := ":"

	// For each port listed
	for _, port := range composePorts {

		// Get the TCP / UDP protocol. Checks to see if it splits in 2 with '/' character.
		// ex. 15000:15000/tcp
		// else, set a default protocol of using TCP
		proto := "TCP"
		protocolCheck := strings.Split(port, "/")
		if len(protocolCheck) == 2 {
			if strings.EqualFold("tcp", protocolCheck[1]) {
				proto = "TCP"
			} else if strings.EqualFold("udp", protocolCheck[1]) {
				proto = "UDP"
			} else {
				return nil, fmt.Errorf("invalid protocol %q", protocolCheck[1])
			}
		}

		// Split up the ports / IP without the "/tcp" or "/udp" appended to it
		justPorts := strings.Split(protocolCheck[0], character)

		if len(justPorts) == 3 {
			// ex. 127.0.0.1:80:80

			// Get the IP address
			hostIP := justPorts[0]
			ip := net.ParseIP(hostIP)
			if ip.To4() == nil && ip.To16() == nil {
				return nil, fmt.Errorf("%q contains an invalid IPv4 or IPv6 IP address", port)
			}

			// Get the host port
			hostPortInt, err := strconv.Atoi(justPorts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid host port %q valid example: 127.0.0.1:80:80", port)
			}

			// Get the container port
			containerPortInt, err := strconv.Atoi(justPorts[2])
			if err != nil {
				return nil, fmt.Errorf("invalid container port %q valid example: 127.0.0.1:80:80", port)
			}

			// Convert to a kobject struct with ports as well as IP
			ports = append(ports, Ports{
				HostPort:      int32(hostPortInt),
				ContainerPort: int32(containerPortInt),
				HostIP:        hostIP,
				Protocol:      proto,
			})

		} else if len(justPorts) == 2 {
			// ex. 80:80

			// Get the host port
			hostPortInt, err := strconv.Atoi(justPorts[0])
			if err != nil {
				return nil, fmt.Errorf("invalid host port %q valid example: 80:80", port)
			}

			// Get the container port
			containerPortInt, err := strconv.Atoi(justPorts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid container port %q valid example: 80:80", port)
			}

			// Convert to a kobject struct and add to the list of ports
			ports = append(ports, Ports{
				HostPort:      int32(hostPortInt),
				ContainerPort: int32(containerPortInt),
				Protocol:      proto,
			})

		} else {
			// ex. 80

			containerPortInt, err := strconv.Atoi(justPorts[0])
			if err != nil {
				return nil, fmt.Errorf("invalid container port %q valid example: 80", port)
			}
			ports = append(ports, Ports{
				ContainerPort: int32(containerPortInt),
				Protocol:      proto,
			})
		}

	}
	return ports, nil
}

// Uses libcompose's APIProject type and converts it to a Kompose object for us to understand
func libComposeToKomposeMapping(composeObject *project.Project) (ComposeObject, error) {

	// Initialize what's going to be returned
	co := ComposeObject{
		ServiceConfigs: make(map[string]ServiceConfig),
	}

	// Here we "clean up" the service configuration so we return something that includes
	// all relevant information as well as avoid the unsupported keys as well.
	for name, composeServiceConfig := range composeObject.ServiceConfigs.All() {
		serviceConfig := ServiceConfig{}
		serviceConfig.Image = composeServiceConfig.Image
		serviceConfig.Build = composeServiceConfig.Build.Context
		newName := normalizeServiceNames(composeServiceConfig.ContainerName)
		serviceConfig.ContainerName = newName
		if serviceConfig.ContainerName == "" {
			serviceConfig.ContainerName = name
		}
		logrus.Debugf("newName is %s, name is %s", newName, name)
		if newName != composeServiceConfig.ContainerName {
			logrus.Infof("Container name in service %q has been changed from %q to %q", name, composeServiceConfig.ContainerName, newName)
		}
		serviceConfig.Entrypoint = composeServiceConfig.Entrypoint
		serviceConfig.Command = composeServiceConfig.Command
		serviceConfig.Dockerfile = composeServiceConfig.Build.Dockerfile
		serviceConfig.BuildArgs = composeServiceConfig.Build.Args

		envs := loadEnvVars(composeServiceConfig.Environment)
		serviceConfig.Environment = envs

		// Validate dockerfile path
		if filepath.IsAbs(serviceConfig.Dockerfile) {
			logrus.Fatalf("%q defined in service %q is an absolute path, it must be a relative path.", serviceConfig.Dockerfile, name)
		}

		// load ports
		ports, err := loadPorts(composeServiceConfig.Ports)
		if err != nil {
			return ComposeObject{}, fmt.Errorf("loadPorts failed. " + name + " failed to load ports from compose file")
		}
		serviceConfig.Port = ports

		serviceConfig.WorkingDir = composeServiceConfig.WorkingDir

		if composeServiceConfig.Volumes != nil {
			for _, volume := range composeServiceConfig.Volumes.Volumes {
				v := normalizeServiceNames(volume.String())
				serviceConfig.VolList = append(serviceConfig.VolList, v)
			}
		}

		// canonical "Custom Labels" handler
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
		err = checkLabelsPorts(len(serviceConfig.Port), composeServiceConfig.Labels["kompose.service.type"], name)
		if err != nil {
			return ComposeObject{}, fmt.Errorf("kompose.service.type can't be set if service doesn't expose any ports.")
		}
		logrus.Debugf("inner service conf is %v", composeServiceConfig)
		// convert compose labels to annotations
		serviceConfig.Annotations = map[string]string(composeServiceConfig.Labels)
		serviceConfig.CPUQuota = int64(composeServiceConfig.CPUQuota)
		serviceConfig.CapAdd = composeServiceConfig.CapAdd
		serviceConfig.CapDrop = composeServiceConfig.CapDrop
		serviceConfig.Pid = composeServiceConfig.Pid
		serviceConfig.Expose = composeServiceConfig.Expose
		serviceConfig.Privileged = composeServiceConfig.Privileged
		serviceConfig.Restart = composeServiceConfig.Restart
		serviceConfig.User = composeServiceConfig.User
		serviceConfig.VolumesFrom = composeServiceConfig.VolumesFrom
		serviceConfig.Stdin = composeServiceConfig.StdinOpen
		serviceConfig.Tty = composeServiceConfig.Tty
		serviceConfig.MemLimit = composeServiceConfig.MemLimit
		serviceConfig.TmpFs = composeServiceConfig.Tmpfs
		serviceConfig.StopGracePeriod = composeServiceConfig.StopGracePeriod

		serviceConfig.DependsON = composeServiceConfig.DependsOn
		serviceConfig.Links = composeServiceConfig.Links
		// Get GroupAdd, group should be mentioned in gid format but not the group name
		groupAdd, err := getGroupAdd(composeServiceConfig.GroupAdd)
		if err != nil {
			return ComposeObject{}, fmt.Errorf("GroupAdd should be mentioned in gid format, not a group name")
		}
		serviceConfig.GroupAdd = groupAdd
		co.ServiceConfigs[normalizeServiceNames(name)] = serviceConfig
		if normalizeServiceNames(name) != name {
			logrus.Infof("Service name in docker-compose has been changed from %q to %q", name, normalizeServiceNames(name))
		}
	}

	// This will handle volume at earlier stage itself, it will resolves problems occurred due to `volumes_from` key
	handleVolume(&co)

	return co, nil
}

// This function will retrieve volumes for each service, as well as it will parse volume information and store it in Volumes struct
func handleVolume(ComposeObject *ComposeObject) {
	for name := range ComposeObject.ServiceConfigs {
		// retrieve volumes of service
		vols, err := retrieveVolume(name, *ComposeObject)
		if err != nil {
			logrus.Errorf("could not retrieve volume %s", err.Error())
		}
		// We can't assign value to struct field in map while iterating over it, so temporary variable `temp` is used here
		var temp = ComposeObject.ServiceConfigs[name]
		temp.Volumes = vols
		ComposeObject.ServiceConfigs[name] = temp
	}
}

func checkLabelsPorts(noOfPort int, labels string, svcName string) error {
	if noOfPort == 0 && (labels == "NodePort" || labels == "LoadBalancer") {
		return fmt.Errorf("%s defined in service %s with no ports present. Issues may occur when bringing up artifacts", labels, svcName)
	}
	return nil
}

// returns all volumes associated with service, if `volumes_from` key is used, we have to retrieve volumes from the services which are mentioned there. Hence, recursive function is used here.
func retrieveVolume(svcName string, ComposeObject ComposeObject) (volume []Volumes, err error) {
	// if volumes-from key is present
	if ComposeObject.ServiceConfigs[svcName].VolumesFrom != nil {
		// iterating over services from `volumes-from`
		for _, depSvc := range ComposeObject.ServiceConfigs[svcName].VolumesFrom {
			// recursive call for retrieving volumes of services from `volumes-from`
			dVols, err := retrieveVolume(depSvc, ComposeObject)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve the volume")
			}
			var cVols []Volumes
			cVols, err = ParseVols(ComposeObject.ServiceConfigs[svcName].VolList, svcName)
			if err != nil {
				return nil, fmt.Errorf("error generting current volumes")
			}

			for _, cv := range cVols {
				// check whether volumes of current service is same or not as that of dependent volumes coming from `volumes-from`
				ok, dv := getVol(cv, dVols)
				if ok {
					// change current volumes service name to dependent service name
					if dv.VFrom == "" {
						cv.VFrom = dv.SvcName
						cv.SvcName = dv.SvcName
					} else {
						cv.VFrom = dv.VFrom
						cv.SvcName = dv.SvcName
					}
					cv.PVCName = dv.PVCName
				}
				volume = append(volume, cv)

			}
			// iterating over dependent volumes
			for _, dv := range dVols {
				// check whether dependent volume is already present or not
				if checkVolDependent(dv, volume) {
					// if found, add service name to `VFrom`
					dv.VFrom = dv.SvcName
					volume = append(volume, dv)
				}
			}
		}
	} else {
		// if `volumes-from` is not present
		volume, err = ParseVols(ComposeObject.ServiceConfigs[svcName].VolList, svcName)
		if err != nil {
			return nil, fmt.Errorf("error generting current volumes")
		}
	}
	return
}

// checkVolDependent returns false if dependent volume is present
func checkVolDependent(dv Volumes, volume []Volumes) bool {
	for _, vol := range volume {
		if vol.PVCName == dv.PVCName {
			return false
		}
	}
	return true

}

func ParseVols(volNames []string, svcName string) ([]Volumes, error) {
	var volumes []Volumes
	var err error

	for i, vn := range volNames {
		var v Volumes
		v.VolumeName, v.Host, v.Container, v.Mode, err = ParseVolume(vn)
		if err != nil {
			return nil, fmt.Errorf("could not parse volume %q: %v", vn, err)
		}
		v.SvcName = svcName
		v.MountPath = fmt.Sprintf("%s:%s", v.Host, v.Container)
		v.PVCName = fmt.Sprintf("%s-claim%d", v.SvcName, i)
		volumes = append(volumes, v)
	}
	return volumes, nil
}

// for dependent volumes, returns true and the respective volume if mountpath are same
func getVol(toFind Volumes, Vols []Volumes) (bool, Volumes) {
	for _, dv := range Vols {
		if toFind.MountPath == dv.MountPath {
			return true, dv
		}
	}
	return false, Volumes{}
}

// getGroupAdd will return group in int64 format
func getGroupAdd(group []string) ([]int64, error) {
	var groupAdd []int64
	for _, i := range group {
		j, err := strconv.Atoi(i)
		if err != nil {
			return nil, fmt.Errorf("unable to get group_add key")
		}
		groupAdd = append(groupAdd, int64(j))

	}
	return groupAdd, nil
}
