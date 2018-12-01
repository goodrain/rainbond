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

package provider

import (
	"os"
	"path"

	"github.com/Sirupsen/logrus"

	"github.com/goodrain/rainbond/util"
	"github.com/goodrain/rainbond/worker/master/volumes/provider/lib/controller"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type rainbondssscProvisioner struct {
	// The directory to create PV-backing directories in
	pvDir string
	name  string
}

// NewRainbondssscProvisioner creates a new Rainbond statefulset share volume provisioner
func NewRainbondssscProvisioner() controller.Provisioner {
	sharePath := os.Getenv("SHARE_DATA_PATH")
	if sharePath == "" {
		sharePath = "/grdata"
	}
	return &rainbondssscProvisioner{
		pvDir: sharePath,
		name:  "rainbond.io/provisioner-sssc",
	}
}

var _ controller.Provisioner = &rainbondssscProvisioner{}

// Provision creates a storage asset and returns a PV object representing it.
func (p *rainbondssscProvisioner) Provision(options controller.VolumeOptions) (*v1.PersistentVolume, error) {
	tenantID := options.PVC.Labels["tenant_id"]
	serviceID := options.PVC.Labels["service_id"]
	path := path.Join(p.pvDir, "tenant", tenantID, "service", serviceID, options.PVC.Name)
	if err := util.CheckAndCreateDirByMode(path, 0777); err != nil {
		return nil, err
	}
	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: options.PVName,
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: options.PersistentVolumeReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)],
			},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: path,
				},
			},
		},
	}
	logrus.Infof("create rainbondsssc pv %s for pvc %s", pv.Name, options.PVC.Name)
	return pv, nil
}

// Delete removes the storage asset that was created by Provision represented
// by the given PV.
func (p *rainbondssscProvisioner) Delete(volume *v1.PersistentVolume) error {

	return nil
}

func (p *rainbondssscProvisioner) Name() string {
	return p.name
}
