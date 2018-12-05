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

package task

import (
	"context"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	client "github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/node/api/model"
	"github.com/goodrain/rainbond/node/core/job"
	"github.com/goodrain/rainbond/node/core/store"
	"github.com/goodrain/rainbond/util"
	"github.com/pquerna/ffjson/ffjson"
)

//startHandleJobRecord 处理task执行记录
func (t *TaskEngine) startHandleJobRecord() {
	jobRecord := t.loadJobRecord()
	if jobRecord != nil {
		for _, er := range jobRecord {
			if !er.IsHandle {
				t.handleJobRecord(er)
			}
		}
	}
	util.Exec(t.ctx, func() error {
		ctx, cancel := context.WithCancel(t.ctx)
		defer cancel()
		ch := store.DefalutClient.WatchByCtx(ctx, t.config.ExecutionRecordPath, client.WithPrefix())
		for event := range ch {
			if err := event.Err(); err != nil {
				logrus.Error("watch job recoder error,", err.Error())
				return nil
			}
			for _, ev := range event.Events {
				switch {
				case ev.IsCreate():
					var er job.ExecutionRecord
					if err := ffjson.Unmarshal(ev.Kv.Value, &er); err == nil {
						if !er.IsHandle {
							t.handleJobRecord(&er)
						}
					}
				}
			}
		}
		return nil
	}, 1)
}

//stopHandleJobRecord
func (t *TaskEngine) stopHandleJobRecord() {

}
func (t *TaskEngine) loadJobRecord() (ers []*job.ExecutionRecord) {
	res, err := store.DefalutClient.Get(t.config.ExecutionRecordPath, client.WithPrefix())
	if err != nil {
		logrus.Error("load job execution record error.", err.Error())
		return nil
	}
	for _, re := range res.Kvs {
		var er job.ExecutionRecord
		if err := ffjson.Unmarshal(re.Value, &er); err == nil {
			ers = append(ers, &er)
		}
	}
	return
}

//handleJobRecord 处理
func (t *TaskEngine) handleJobRecord(er *job.ExecutionRecord) {
	task := t.GetTask(er.TaskID)
	if task == nil {
		return
	}
	//更新task信息
	defer t.UpdateTask(task)
	defer er.CompleteHandle()
	taskStatus := model.TaskStatus{
		JobID:        er.JobID,
		StartTime:    er.BeginTime,
		EndTime:      er.EndTime,
		Status:       "complete",
		CompleStatus: "Failure",
	}
	if status, ok := task.Status[er.Node]; ok {
		taskStatus = status
		taskStatus.EndTime = time.Now()
		taskStatus.Status = "complete"
	}
	if er.Output != "" {
		index := strings.Index(er.Output, "{")
		jsonOutPut := er.Output
		if index > -1 {
			jsonOutPut = er.Output[index:]
		}
		output, err := model.ParseTaskOutPut(jsonOutPut)
		output.JobID = er.JobID
		if err != nil {
			taskStatus.Status = "Parse task output error"
			taskStatus.CompleStatus = "Unknow"
			logrus.Warning("parse task output error:", err.Error())
			output.NodeID = er.Node
		} else {
			output.NodeID = er.Node
			if output.Global != nil && len(output.Global) > 0 {
				for k, v := range output.Global {
					if strings.Index(v, ",") > -1 {
						values := strings.Split(v, ",")
						err := t.dataCenterConfig.PutConfig(&model.ConfigUnit{
							Name:           strings.ToUpper(k),
							Value:          values,
							ValueType:      "array",
							IsConfigurable: false,
						})
						if err != nil {
							logrus.Errorf("save datacenter config %s=%s error.%s", k, v, err.Error())
						}
					} else {
						err := t.dataCenterConfig.PutConfig(&model.ConfigUnit{
							Name:           strings.ToUpper(k),
							Value:          v,
							ValueType:      "string",
							IsConfigurable: false,
						})
						if err != nil {
							logrus.Errorf("save datacenter config %s=%s error.%s", k, v, err.Error())
						}
					}
				}
			}
			//groupID不为空，处理group连环操作
			if output.Inner != nil && len(output.Inner) > 0 && task.GroupID != "" {
				t.AddGroupConfig(task.GroupID, output.Inner)
			}
			for _, status := range output.Status {
				if status.ConditionType != "" && status.ConditionStatus != "" {
					//t.nodeCluster.UpdateNodeCondition(er.Node, status.ConditionType, status.ConditionStatus)
				}
				if status.NextTask != nil && len(status.NextTask) > 0 {
					for _, taskID := range status.NextTask {
						//由哪个节点发起的执行请求，当前task只在此节点执行
						t.PutSchedul(taskID, output.NodeID)
					}
				}
				if status.NextGroups != nil && len(status.NextGroups) > 0 {
					for _, groupID := range status.NextGroups {
						group := t.GetTaskGroup(groupID)
						if group == nil {
							continue
						}
						t.ScheduleGroup(group, output.NodeID)
					}
				}
			}
			taskStatus.CompleStatus = output.ExecStatus
		}
		task.UpdataOutPut(output)
	}
	task.ExecCount++
	if task.Status == nil {
		task.Status = make(map[string]model.TaskStatus)
	}
	task.Status[er.Node] = taskStatus
}
