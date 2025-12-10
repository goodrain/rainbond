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

package etcd

import (
	"context"

	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	"github.com/sirupsen/logrus"
)

//HandleEtcdError 处理etcd错误
func HandleEtcdError(err error) {
	switch err {
	case context.Canceled:
		logrus.Errorf("ctx is canceled by another routine: %v", err)
	case context.DeadlineExceeded:
		logrus.Errorf("ctx is attached with a deadline is exceeded: %v", err)
	case rpctypes.ErrEmptyKey:
		logrus.Errorf("client-side error: %v", err)
	default:
		logrus.Errorf("bad cluster endpoints, which are not etcd servers: %v", err)
	}
}
