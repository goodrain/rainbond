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
	"time"

	"github.com/Sirupsen/logrus"

	"github.com/goodrain/rainbond/worker/appm/types/v1"

	"github.com/goodrain/rainbond/util"
)

//clean clean Possible duplicate resources
func (a *appRuntimeStore) clean() {
	util.Exec(a.ctx, func() error {
		var deleteKey []v1.CacheKey
		a.appServices.Range(func(k, v interface{}) bool {
			appservice, ok := v.(*v1.AppService)
			if !ok || appservice == nil {
				deleteKey = append(deleteKey, k.(v1.CacheKey))
			}
			if appservice.IsClosed() {
				deleteKey = append(deleteKey, k.(v1.CacheKey))
			}
			return true
		})
		for _, key := range deleteKey {
			a.appServices.Delete(key)
		}
		logrus.Debugf("app store is cleaned")
		return nil
	}, time.Second*10)
}
