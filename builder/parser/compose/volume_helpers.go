package compose

import (
	"fmt"
	"strconv"

	"github.com/sirupsen/logrus"
)

// handleVolume resolves volumes_from relationships and fills the normalized
// volume descriptors for each parsed compose service.
func handleVolume(composeObject *ComposeObject) {
	for name := range composeObject.ServiceConfigs {
		vols, err := retrieveVolume(name, *composeObject)
		if err != nil {
			logrus.Errorf("could not retrieve volume %s", err.Error())
		}
		temp := composeObject.ServiceConfigs[name]
		temp.Volumes = vols
		composeObject.ServiceConfigs[name] = temp
	}
}

func retrieveVolume(serviceName string, composeObject ComposeObject) (volume []Volumes, err error) {
	if composeObject.ServiceConfigs[serviceName].VolumesFrom != nil {
		for _, depSvc := range composeObject.ServiceConfigs[serviceName].VolumesFrom {
			dVols, err := retrieveVolume(depSvc, composeObject)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve the volume")
			}
			cVols, err := parseVols(composeObject.ServiceConfigs[serviceName].VolList, serviceName)
			if err != nil {
				return nil, fmt.Errorf("error generting current volumes")
			}

			for _, cv := range cVols {
				ok, dv := getVolumeByMountPath(cv, dVols)
				if ok {
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

			for _, dv := range dVols {
				if checkDependentVolume(dv, volume) {
					dv.VFrom = dv.SvcName
					volume = append(volume, dv)
				}
			}
		}
	} else {
		volume, err = parseVols(composeObject.ServiceConfigs[serviceName].VolList, serviceName)
		if err != nil {
			return nil, fmt.Errorf("error generting current volumes")
		}
	}
	return
}

func checkDependentVolume(depVol Volumes, volume []Volumes) bool {
	for _, vol := range volume {
		if vol.PVCName == depVol.PVCName {
			return false
		}
	}
	return true
}

func parseVols(volumeNames []string, serviceName string) ([]Volumes, error) {
	var volumes []Volumes
	for i, volumeName := range volumeNames {
		var volume Volumes
		var err error
		volume.VolumeName, volume.Host, volume.Container, volume.Mode, err = ParseVolume(volumeName)
		if err != nil {
			return nil, fmt.Errorf("could not parse volume %q: %v", volumeName, err)
		}
		volume.SvcName = serviceName
		volume.MountPath = fmt.Sprintf("%s:%s", volume.Host, volume.Container)
		volume.PVCName = fmt.Sprintf("%s-claim%d", volume.SvcName, i)
		volumes = append(volumes, volume)
	}
	return volumes, nil
}

func getVolumeByMountPath(toFind Volumes, volumes []Volumes) (bool, Volumes) {
	for _, vol := range volumes {
		if toFind.MountPath == vol.MountPath {
			return true, vol
		}
	}
	return false, Volumes{}
}

func getGroupAdd(groups []string) ([]int64, error) {
	var groupAdd []int64
	for _, group := range groups {
		value, err := strconv.Atoi(group)
		if err != nil {
			return nil, fmt.Errorf("unable to get group_add key")
		}
		groupAdd = append(groupAdd, int64(value))
	}
	return groupAdd, nil
}
