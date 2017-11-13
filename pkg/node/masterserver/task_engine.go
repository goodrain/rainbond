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

// 当前包处理任务的执行，结果处理，任务自动调度
package masterserver

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/goodrain/rainbond/pkg/node/core/store"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/Sirupsen/logrus"

	client "github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/core/job"
)

//TaskEngine 任务引擎
type TaskEngine struct {
	ctx        context.Context
	cancel     context.CancelFunc
	config     *option.Conf
	statics    map[string]*model.Task
	staticLock sync.Mutex
}

//CreateTaskEngine 创建task管理引擎
func CreateTaskEngine() *TaskEngine {
	ctx, cancel := context.WithCancel(context.Background())
	return &TaskEngine{
		ctx:     ctx,
		cancel:  cancel,
		statics: make(map[string]*model.Task),
		config:  option.Config,
	}
}

//Start 启动
func (t *TaskEngine) Start() {
	logrus.Info("task engine start")
	go t.LoadStaticTask()
}

//Stop 启动
func (t *TaskEngine) Stop() {
	t.cancel()
}

//LoadStaticTask 从文件加载task
//TODO:动态加载
func (t *TaskEngine) LoadStaticTask() {
	logrus.Infof("start load static task form path:%s", t.config.StaticTaskPath)
	file, err := os.Stat(t.config.StaticTaskPath)
	if err != nil {
		logrus.Errorf("load static task error %s", err.Error())
		return
	}
	if file.IsDir() {
		files, err := ioutil.ReadDir(t.config.StaticTaskPath)
		if err != nil {
			logrus.Errorf("load static task error %s", err.Error())
			return
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			} else if strings.HasSuffix(file.Name(), ".json") {
				t.loadFile(path.Join(t.config.StaticTaskPath, file.Name()))
			}
		}
	} else {
		t.loadFile(t.config.StaticTaskPath)
	}
}
func (t *TaskEngine) loadFile(path string) {
	taskBody, err := ioutil.ReadFile(path)
	if err != nil {
		logrus.Errorf("read static task file %s error.%s", path, err.Error())
		return
	}
	var task model.Task
	if err := ffjson.Unmarshal(taskBody, &task); err != nil {
		logrus.Errorf("unmarshal static task file %s error.%s", path, err.Error())
		return
	}
	if task.ID == "" {
		task.ID = task.Name
	}
	if task.Name == "" {
		logrus.Errorf("task name can not be empty. file %s", path)
		return
	}
	if task.Temp == nil {
		logrus.Errorf("task [%s] temp can not be empty.", task.Name)
		return
	}
	if task.Temp.ID == "" {
		task.Temp.ID = task.Temp.Name
	}
	t.staticLock.Lock()
	defer t.staticLock.Unlock()
	t.statics[task.Name] = &task
	logrus.Infof("Load a static task %s, start run it.", task.Name)
	j, err := job.CreateJobFromTask(&task, nil)
	if err != nil {
		logrus.Errorf("run task %s error.%s", task.Name, err.Error())
		return
	}
	if j.IsOnce {
		if err := job.PutOnce(j); err != nil {
			logrus.Errorf("run task %s error.%s", task.Name, err.Error())
			return
		}
	} else {
		if err := job.AddJob(j); err != nil {
			logrus.Errorf("run task %s error.%s", task.Name, err.Error())
			return
		}
	}
}

//GetTask gettask
func (t *TaskEngine) GetTask(taskID string) *model.Task {
	res, err := store.DefalutClient.Get("/store/tasks/" + taskID)
	if err != nil {
		return nil
	}
	if res.Count < 1 {
		return nil
	}
	var task model.Task
	if err := ffjson.Unmarshal(res.Kvs[0].Value, &task); err != nil {
		return nil
	}
	return &task
}

//handleJobRecord 处理
func (t *TaskEngine) handleJobRecord(er *job.ExecutionRecord) {
	task := t.GetTask(er.TaskID)
	if task == nil {
		er.CompleteHandle()
	}
	output, err := model.ParseTaskOutPut(er.Output)
	if err != nil {
		logrus.Error("parse task out put error,", err.Error())
		return
	}
	if output.Global != nil && len(output.Global) > 0 {

	}
}

//HandleJobRecord 处理task执行记录
func (t *TaskEngine) HandleJobRecord() {
	jobRecord := t.loadJobRecord()
	if jobRecord != nil {
		for _, er := range jobRecord {
			if !er.IsHandle {
				t.handleJobRecord(er)
			}
		}
	}
	ch := store.DefalutClient.Watch(t.config.ExecutionRecordPath, client.WithPrefix())
	for {
		select {
		case <-t.ctx.Done():
			return
		case event := <-ch:
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
	}
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
