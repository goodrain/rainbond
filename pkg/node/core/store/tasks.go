
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
	"acp_node/pkg/api/model"
	"fmt"
	"strings"

	client "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	conf "github.com/goodrain/rainbond/cmd/node/option"
	"github.com/pquerna/ffjson/ffjson"
)

//SendTask 发送任务
func SendTask(tasks ...*model.Task) error {
	for _, task := range tasks {
		_, err := DefalutClient.Put(conf.Config.TaskPath+"/"+task.ID, task.String())
		if err != nil {
			return err
		}
	}
	return nil
}

//WatchTasks 监听任务
func WatchTasks() client.WatchChan {
	return DefalutClient.Watch(conf.Config.TaskPath, client.WithPrefix())
}

//GetTaskFromKv create task from etcd value
func GetTaskFromKv(kv *mvccpb.KeyValue) (task *model.Task, err error) {
	var t model.Task
	if err = ffjson.Unmarshal(kv.Value, &t); err != nil {
		err = fmt.Errorf("job[%s] umarshal err: %s", string(kv.Key), err.Error())
		return
	}
	return &t, err
}

//GetIDFromKey 从 etcd 的 key 中取 id
func GetIDFromKey(key string) string {
	index := strings.LastIndex(key, "/")
	if index < 0 {
		return ""
	}
	return key[index+1:]
}
