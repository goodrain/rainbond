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
// 本文件主要定义了 Rainbond 平台中与存储类（StorageClass）相关的初始化和管理逻辑。
// 在 Kubernetes 中，StorageClass 用于动态配置存储卷，本文件定义了 Rainbond 平台支持的几种存储类。

// 文件内容包括以下几个主要部分：
// 1. Rainbond 支持的存储类常量定义：文件中定义了两个常量 `RainbondStatefuleShareStorageClass` 和 `RainbondStatefuleLocalStorageClass`，
//    分别表示 Rainbond 平台支持的共享存储和本地存储的 StorageClass 名称。

// 2. 初始化存储类：`init()` 函数用于初始化存储类列表，包括 Rainbond 平台支持的共享存储和本地存储的 StorageClass。
//    初始化过程中设置了存储类的 provisioner、卷绑定模式（VolumeBindingMode）和回收策略（ReclaimPolicy）。

// 3. 存储类的获取函数：`GetInitStorageClass()` 函数根据系统环境变量 `ALLINONE_MODE` 来决定返回的存储类列表。
//    如果 `ALLINONE_MODE` 为 `true`，则返回本地存储类列表，否则返回共享存储类列表。

// 通过这些逻辑，Rainbond 平台能够根据不同的部署环境，动态初始化和获取适合的存储类配置，为应用服务的持久化存储提供支持。

package v1

import (
	v1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
)

// kind: StorageClass
// apiVersion: storage.k8s.io/v1
// metadata:
//   name: local-storage
// provisioner: kubernetes.io/no-provisioner
// volumeBindingMode: WaitForFirstConsumer

var initStorageClass []*storagev1.StorageClass
var initLocalStorageClass []*storagev1.StorageClass

// RainbondStatefuleShareStorageClass rainbond support statefulset app share volume
var RainbondStatefuleShareStorageClass = "rainbondsssc"

// RainbondStatefuleLocalStorageClass rainbond support statefulset app local volume
var RainbondStatefuleLocalStorageClass = "rainbondslsc"

func init() {
	var volumeBindingImmediate = storagev1.VolumeBindingImmediate
	var columeWaitForFirstConsumer = storagev1.VolumeBindingWaitForFirstConsumer
	var Retain = v1.PersistentVolumeReclaimRetain
	initStorageClass = append(initStorageClass, &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: RainbondStatefuleShareStorageClass,
		},
		Provisioner:       "rainbond.io/provisioner-sssc",
		VolumeBindingMode: &volumeBindingImmediate,
		ReclaimPolicy:     &Retain,
	})
	initStorageClass = append(initStorageClass, &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: RainbondStatefuleLocalStorageClass,
		},
		Provisioner:       "rainbond.io/provisioner-sslc",
		VolumeBindingMode: &columeWaitForFirstConsumer,
		ReclaimPolicy:     &Retain,
	})
	initLocalStorageClass = append(initLocalStorageClass, &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: RainbondStatefuleShareStorageClass,
		},
		Provisioner:       "rancher.io/local-path",
		VolumeBindingMode: &columeWaitForFirstConsumer,
		ReclaimPolicy:     &Retain,
	})
	initLocalStorageClass = append(initLocalStorageClass, &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: RainbondStatefuleLocalStorageClass,
		},
		Provisioner:       "rancher.io/local-path",
		VolumeBindingMode: &columeWaitForFirstConsumer,
		ReclaimPolicy:     &Retain,
	})
}

// GetInitStorageClass get init storageclass list
func GetInitStorageClass() []*storagev1.StorageClass {
	if os.Getenv("ALLINONE_MODE") == "true" {
		return initLocalStorageClass
	}
	return initStorageClass
}
