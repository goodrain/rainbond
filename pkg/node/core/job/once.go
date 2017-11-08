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

package job

import (
	client "github.com/coreos/etcd/clientv3"

	conf "github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/core/store"

	"github.com/Sirupsen/logrus"
)

// 马上执行 job 任务
// 注册到 /cronsun/once/group/<jobID>
// value
// 若执行单个结点，则值为 NodeID
// 若 job 所在的结点都需执行，则值为空 ""
func PutOnce(group, jobID, nodeID string) error {
	_, err := store.DefalutClient.Put(conf.Config.Once+group+"/"+jobID, nodeID)
	return err
}
func PutBuildIn(jobID, nodeID string) error {
	logrus.Infof("put build in job to watch,%s", jobID+"-"+nodeID)
	_, err := store.DefalutClient.Put(conf.Config.BuildInExec+jobID+"-"+nodeID, nodeID)
	return err
}

func WatchOnce() client.WatchChan {
	return store.DefalutClient.Watch(conf.Config.Once, client.WithPrefix())
}
func WatchBuildIn() client.WatchChan {
	return store.DefalutClient.Watch(conf.Config.BuildInExec, client.WithPrefix())
}
func WatchBuildInLog() client.WatchChan {
	return store.DefalutClient.Watch(conf.Config.JobLog+BuildIn_JobLog, client.WithPrefix())
}
