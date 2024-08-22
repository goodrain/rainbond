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

// 该文件实现了Rainbond平台中的存储类 (StorageClass) 初始化逻辑。
// StorageClass 是 Kubernetes 中用于定义存储资源的接口，该文件通过初始化和管理 StorageClass，
// 确保在 Rainbond 平台上可以使用合适的存储资源。

// 文件中的主要功能包括：
// 1. `initStorageclass` 方法：用于初始化或更新 Kubernetes 中的 StorageClass 资源。
//    该方法会遍历一组预定义的 StorageClass，如果在 Kubernetes 集群中找不到对应的 StorageClass，
//    则会创建新的 StorageClass；如果已存在但配置不同，则会更新它。
// 2. 更新策略：当检测到现有的 StorageClass 需要更新时，先删除旧的 StorageClass，然后重新创建一个新的。
//    这样确保 StorageClass 的配置是最新的，并且能够正确应用于集群中的存储资源管理。
// 3. 错误处理与日志记录：在整个过程中，该方法会详细记录每一步操作的结果，包括创建、更新、删除操作的成功与失败，
//    以帮助运维人员监控和排查问题。

// 总的来说，该文件通过实现 StorageClass 的初始化和更新机制，确保 Rainbond 平台能够使用到正确配置的存储资源，
// 从而为平台上的应用服务提供稳定的存储支持。

package store

import (
	"context"

	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InitStorageclass init storage class
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
			update := false
			if old.VolumeBindingMode == nil {
				update = true
			}
			if !update && old.ReclaimPolicy == nil {
				update = true
			}
			if !update && string(*old.VolumeBindingMode) != string(*storageclass.VolumeBindingMode) {
				update = true
			}
			if !update && string(*old.ReclaimPolicy) != string(*storageclass.ReclaimPolicy) {
				update = true
			}
			if update {
				err := a.conf.KubeClient.StorageV1().StorageClasses().Delete(context.Background(), storageclass.Name, metav1.DeleteOptions{})
				if err == nil {
					_, err := a.conf.KubeClient.StorageV1().StorageClasses().Create(context.Background(), storageclass, metav1.CreateOptions{})
					if err != nil {
						logrus.Errorf("recreate strageclass %s failure %s", storageclass.Name, err.Error())
					}
					logrus.Infof("update storageclass %s success", storageclass.Name)
				} else {
					logrus.Errorf("recreate strageclass %s failure %s", storageclass.Name, err.Error())
				}
			}
		}
	}
	return nil
}
