/*
Copyright 2017 The Kubernetes Authors.

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

package util

import "k8s.io/api/core/v1"

// Common allocation units
const (
	KiB int64 = 1024
	MiB int64 = 1024 * KiB
	GiB int64 = 1024 * MiB
	TiB int64 = 1024 * GiB
)

// RoundUpSize calculates how many allocation units are needed to accommodate
// a volume of given size. E.g. when user wants 1500MiB volume, while AWS EBS
// allocates volumes in gibibyte-sized chunks,
// RoundUpSize(1500 * 1024*1024, 1024*1024*1024) returns '2'
// (2 GiB is the smallest allocatable volume that can hold 1500MiB)
func RoundUpSize(volumeSizeBytes int64, allocationUnitBytes int64) int64 {
	return (volumeSizeBytes + allocationUnitBytes - 1) / allocationUnitBytes
}

// RoundUpToGiB rounds up given quantity upto chunks of GiB
func RoundUpToGiB(sizeBytes int64) int64 {
	return RoundUpSize(sizeBytes, GiB)
}

// AccessModesContains returns whether the requested mode is contained by modes
func AccessModesContains(modes []v1.PersistentVolumeAccessMode, mode v1.PersistentVolumeAccessMode) bool {
	for _, m := range modes {
		if m == mode {
			return true
		}
	}
	return false
}

// AccessModesContainedInAll returns whether all of the requested modes are contained by modes
func AccessModesContainedInAll(indexedModes []v1.PersistentVolumeAccessMode, requestedModes []v1.PersistentVolumeAccessMode) bool {
	for _, mode := range requestedModes {
		if !AccessModesContains(indexedModes, mode) {
			return false
		}
	}
	return true
}

// CheckPersistentVolumeClaimModeBlock checks VolumeMode.
// If the mode is Block, return true otherwise return false.
func CheckPersistentVolumeClaimModeBlock(pvc *v1.PersistentVolumeClaim) bool {
	return pvc.Spec.VolumeMode != nil && *pvc.Spec.VolumeMode == v1.PersistentVolumeBlock
}
