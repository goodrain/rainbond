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

package store

import (
	"context"

	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//InitStorageclass init storage class
func (a *appRuntimeStore) initStorageclass() error {
	for _, storageclass := range v1.GetInitStorageClass() {
		old, err := a.conf.KubeClient.StorageV1().StorageClasses().Get(context.Background(), storageclass.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				_, err = a.conf.KubeClient.StorageV1().StorageClasses().Create(context.Background(), storageclass, metav1.CreateOptions{})
			}
			if err != nil {
				return err
			}
			logrus.Info("create storageclass %s", storageclass.Name)
		} else {
			if old.VolumeBindingMode != storageclass.VolumeBindingMode || old.ReclaimPolicy != storageclass.ReclaimPolicy {
				err := a.conf.KubeClient.StorageV1().StorageClasses().Delete(context.Background(), storageclass.Name, metav1.DeleteOptions{})
				if err == nil {
					_, err := a.conf.KubeClient.StorageV1().StorageClasses().Create(context.Background(), storageclass, metav1.CreateOptions{})
					if err != nil {
						logrus.Errorf("recreate strageclass %s failure %s", storageclass.Name, err.Error())
					}
					logrus.Info("update storageclass %s success", storageclass.Name)
				} else {
					logrus.Errorf("recreate strageclass %s failure %s", err.Error())
				}
			}
		}
	}
	return nil
}
