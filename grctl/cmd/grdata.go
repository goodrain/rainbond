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

package cmd

import (
	"errors"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/urfave/cli"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	// ErrUnsupportedPersistentVolumeType -
	ErrUnsupportedPersistentVolumeType = errors.New("unsupported persistentVolume")
)

//NewCmdGrdata -
func NewCmdGrdata() cli.Command {
	c := cli.Command{
		Name:  "grdata",
		Usage: "grctl grdata -h",
		Subcommands: []cli.Command{
			cli.Command{
				Name:  "get",
				Usage: "get the mount options for grdata",
				Action: func(c *cli.Context) error {
					Common(c)
					cmd, err := getmountCommand()
					if err != nil {
						logrus.Error("set default tenantname fail", err.Error())
						return nil
					}
					fmt.Println("Mount command: sudo mkdir /grdata & " + cmd)
					return nil
				},
			},
		},
	}
	return c
}

func getmountCommand() (string, error) {
	pvc, err := clients.K8SClient.CoreV1().PersistentVolumeClaims("rbd-system").Get("rbd-share-grdata", metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	// sudo mount -t nfs -o vers=4,minorversion=0,rsize=1048576,wsize=1048576,hard,timeo=600,retrans=2,noresvport 8bbca4a8ad-wgo45.cn-huhehaote.nas.aliyuncs.com:/ /mnt
	pv, err := clients.K8SClient.CoreV1().PersistentVolumes().Get(pvc.Spec.VolumeName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	pvs := pv.Spec.PersistentVolumeSource
	switch {
	case pvs.CSI != nil:
		return mountCommandFromAlicloudNAS(pvs.CSI)
	case pvs.NFS != nil:
		return mountCommandFromNFS(pvs.NFS), nil
	default:
		return "", ErrUnsupportedPersistentVolumeType
	}
}

func mountCommandFromAlicloudNAS(csi *corev1.CSIPersistentVolumeSource) (string, error) {
	if csi.Driver != "nasplugin.csi.alibabacloud.com" {
		return "", ErrUnsupportedPersistentVolumeType
	}

	path := csi.VolumeAttributes["path"]
	server := csi.VolumeAttributes["server"]

	format := "sudo mount -t nfs -o vers=4,minorversion=0,rsize=1048576,wsize=1048576,hard,timeo=600,retrans=2,noresvport %s:%s %s"
	cmd := fmt.Sprintf(format, server, path, "/grdata")

	return cmd, nil
}

func mountCommandFromNFS(nfs *corev1.NFSVolumeSource) string {
	path := nfs.Path
	server := nfs.Server

	// mount moonshot:/home /home
	format := "sudo mount -t nfs %s:%s %s"
	cmd := fmt.Sprintf(format, server, path, "/grdata")
	return cmd
}
