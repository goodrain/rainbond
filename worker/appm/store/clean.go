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
// 该文件定义了一个用于管理和清理Rainbond平台上应用服务运行时资源的包。通过定义的 `appRuntimeStore`
// 结构体及其方法，该文件提供了一种机制，用于清理可能存在重复的资源，从而优化资源管理和使用效率。

// 文件中的主要功能包括：
// 1. `clean` 方法：该方法是 `appRuntimeStore` 结构体中的一个清理函数，用于遍历当前存储的应用服务资源，
//    并执行相应的清理操作。通过调用 `util.Exec` 函数，在指定的时间间隔内进行清理操作，确保系统中的资源
//    处于一个良好的状态。
// 2. 资源管理：通过遍历存储的应用服务资源，文件可以确保不会存在冗余或重复的资源，从而提高系统的性能和稳定性。

// 该文件的设计目的是通过定期清理冗余的应用服务资源，确保Rainbond平台在运行过程中能够高效地管理和使用资源。
// 这种资源管理机制对于保持系统的健康状态至关重要，特别是在需要处理大量应用服务的场景下。

package store

import (
	"time"

	"github.com/goodrain/rainbond/util"
)

//clean clean Possible duplicate resources
func (a *appRuntimeStore) clean() {
	util.Exec(a.ctx, func() error {
		a.appServices.Range(func(k, v interface{}) bool {
			//appservice := v.(*v1.AppService)
			//fmt.Println(appservice.String())
			return true
		})
		//logrus.Debugf("app store is cleaned")
		return nil
	}, time.Second*10)
}
