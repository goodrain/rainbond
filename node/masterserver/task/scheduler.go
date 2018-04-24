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
	"crypto/sha1"
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	client "github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/node/api/model"
	"github.com/goodrain/rainbond/node/core/config"
	"github.com/goodrain/rainbond/node/core/job"
	"github.com/goodrain/rainbond/node/core/store"
	"github.com/goodrain/rainbond/util"
)

//Scheduler 调度器
type Scheduler struct {
	taskEngine *TaskEngine
	cache      chan *job.Job
	ctx        context.Context
	cancel     context.CancelFunc
}

func createScheduler(taskEngine *TaskEngine) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		taskEngine: taskEngine,
		cache:      make(chan *job.Job, 100),
		ctx:        ctx,
		cancel:     cancel,
	}
}
func (s *Scheduler) putSchedulerChan(jb *job.Job, duration time.Duration) {
	go func() {
		time.Sleep(duration)
		s.cache <- jb
	}()
}

//Next 下一个调度对象
func (s *Scheduler) Next() (*job.Job, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	select {
	case job := <-s.cache:
		return job, nil
	case <-s.ctx.Done():
		return nil, fmt.Errorf("ctx context cancel")
	case <-ctx.Done():
		return nil, fmt.Errorf("time out")
	}
}

//Stop 停止
func (s *Scheduler) Stop() {
	logrus.Infof("task engine scheduler worker is stopping")
	s.cancel()
}

//StartScheduler 开始调度
func (t *TaskEngine) startScheduler() {
	t.loadAndWatchJobs()
	logrus.Info("Start scheduler worker...")
	defer logrus.Info("scheduler worker closed....")
	for {
		next, err := t.scheduler.Next()
		if err != nil {
			if err.Error() == "time out" {
				continue
			}
			if err.Error() == "ctx context cancel" {
				logrus.Warningf("get next scheduler job ctx context cancel")
				return
			}
			continue
		}
		logrus.Infof("Start schedule job %s to node %s", next.Hash, next.NodeID)
		task := t.GetTask(next.TaskID)
		if task == nil {
			logrus.Errorf("job %s task %s not found when scheduler", next.ID, next.TaskID)
			continue
		}
		vas := t.GetValidationCriteria(task)
		for i, va := range vas {
			ok, err := va(next.NodeID, task)
			if err != nil {
				if task.Scheduler.Status == nil {
					task.Scheduler.Status = make(map[string]model.SchedulerStatus)
				}
				task.Scheduler.Status[next.NodeID] = model.SchedulerStatus{
					Status:          "Failure",
					Message:         err.Error(),
					SchedulerMaster: t.currentNode.ID,
					SchedulerTime:   time.Now(),
				}
				t.UpdateTask(task)
				next.Scheduler = &job.Scheduler{
					NodeID:          next.NodeID,
					SchedulerTime:   time.Now(),
					SchedulerStatus: "Failure",
					Message:         err.Error(),
				}
				t.UpdateJob(next)
				logrus.Errorf("Failure schedule job %s to node %s", next.Hash, next.NodeID)
				break
			}
			if !ok {
				if task.Scheduler.Status == nil {
					task.Scheduler.Status = make(map[string]model.SchedulerStatus)
				}
				task.Scheduler.Status[next.NodeID] = model.SchedulerStatus{
					Status:          "Waiting",
					Message:         "waiting validation criteria",
					SchedulerMaster: t.currentNode.ID,
					SchedulerTime:   time.Now(),
				}
				t.UpdateTask(task)
				t.scheduler.putSchedulerChan(next, 3*time.Second)
				break
			}
			//全部条件满足
			if i == len(vas)-1 {
				if task.Scheduler.Status == nil {
					task.Scheduler.Status = make(map[string]model.SchedulerStatus)
				}
				task.Scheduler.Status[next.NodeID] = model.SchedulerStatus{
					Status:          "Success",
					Message:         "Success",
					SchedulerMaster: t.currentNode.ID,
					SchedulerTime:   time.Now(),
				}
				task.Status[next.NodeID] = model.TaskStatus{
					JobID:     next.ID,
					Status:    "Start",
					StartTime: time.Now(),
				}
				next.Scheduler = &job.Scheduler{
					NodeID:          next.NodeID,
					SchedulerTime:   time.Now(),
					SchedulerStatus: "Success",
					CanRun:          true,
				}
				err := t.UpdateJobConfig(next, task.GroupID)
				if err != nil {
					task.Status[next.NodeID] = model.TaskStatus{
						JobID:        next.ID,
						Status:       "complete",
						CompleStatus: "Failure",
						Message:      "update job config error," + err.Error(),
						StartTime:    time.Now(),
						EndTime:      time.Now(),
					}
				}
				t.UpdateTask(task)
				t.UpdateJob(next)
				logrus.Infof("Success schedule job %s to node %s", next.Hash, next.NodeID)
			}
		}

	}
}

//UpdateJobConfig 更新job的配置
//解析可赋值变量 ${XXX}
func (t *TaskEngine) UpdateJobConfig(jb *job.Job, groupID string) error {
	var groupCtx *config.GroupContext
	if groupID != "" {
		groupCtx = t.dataCenterConfig.GetGroupConfig(groupID)
	}
	command, err := config.ResettingString(groupCtx, jb.Command)
	if err != nil {
		return err
	}
	stdin, err := config.ResettingString(groupCtx, jb.Stdin)
	if err != nil {
		return err
	}
	envMaps, err := config.ResettingArray(groupCtx, jb.Envs)
	if err != nil {
		return err
	}
	jb.Command = command
	jb.Stdin = stdin
	jb.Envs = envMaps
	return nil
}

func (t *TaskEngine) stopScheduler() {
	t.scheduler.Stop()
}

func (t *TaskEngine) loadAndWatchJobs() {
	load, _ := store.DefalutClient.Get(t.config.JobPath, client.WithPrefix())
	if load != nil && load.Count > 0 {
		for _, kv := range load.Kvs {
			jb, err := job.GetJobFromKv(kv)
			if err != nil {
				logrus.Errorf("load job(%s) error,%s", kv.Key, err.Error())
				continue
			}
			t.andOrUpdateJob(jb)
		}
	}
	logrus.Infof("load exist job success,count %d", len(t.jobs))
	go util.Exec(t.ctx, func() error {
		ctx, cancel := context.WithCancel(t.ctx)
		defer cancel()
		ch := store.DefalutClient.WatchByCtx(ctx, t.config.JobPath, client.WithPrefix())
		for event := range ch {
			if err := event.Err(); err != nil {
				logrus.Error("watch job error,", err.Error())
				return nil
			}
			for _, ev := range event.Events {
				switch {
				case ev.IsCreate(), ev.IsModify():
					jb, err := job.GetJobFromKv(ev.Kv)
					if err != nil {
						logrus.Errorf("load job(%s) error,%s", ev.Kv.Key, err.Error())
						continue
					}
					t.andOrUpdateJob(jb)
				case ev.Type == client.EventTypeDelete:
					t.deleteJob(job.GetIDFromKey(string(ev.Kv.Key)))
				}
			}
		}
		return nil
	}, 3)
}

func (t *TaskEngine) andOrUpdateJob(jb *job.Job) {
	t.jobsLock.Lock()
	defer t.jobsLock.Unlock()
	t.jobs[jb.Hash] = jb
	if jb.Scheduler == nil {
		t.scheduler.putSchedulerChan(jb, 0)
		logrus.Infof("cache a job and put scheduler")
	}
}

//UpdateJob 持久化增加or更新job
func (t *TaskEngine) UpdateJob(jb *job.Job) {
	t.jobsLock.Lock()
	defer t.jobsLock.Unlock()
	t.jobs[jb.Hash] = jb
	job.PutJob(jb)
}
func (t *TaskEngine) deleteJob(jbHash string) {
	t.jobsLock.Lock()
	defer t.jobsLock.Unlock()
	if _, ok := t.jobs[jbHash]; ok {
		delete(t.jobs, jbHash)
	}
}

//PutSchedul 发布调度需求，即定义task的某个执行节点
//taskID+nodeID = 一个调度单位,保证不重复
//node不能为空
func (t *TaskEngine) PutSchedul(taskID string, nodeID string) (err error) {
	if taskID == "" || nodeID == "" {
		return fmt.Errorf("taskid or nodeid can not be empty")
	}
	task := t.GetTask(taskID)
	if task == nil {
		return fmt.Errorf("task %s not found", taskID)
	}
	node := t.nodeCluster.GetNode(nodeID)
	if node == nil {
		return fmt.Errorf("node %s not found", nodeID)
	}
	hash := getHash(taskID, nodeID)
	logrus.Infof("put scheduler hash %s", hash)
	//初步判断任务是否能被创建
	if oldjob := t.GetJob(hash); oldjob != nil {
		if task.RunMode == string(job.OnlyOnce) || task.RunMode == string(job.Cycle) {
			if oldjob.Scheduler != nil && oldjob.Scheduler.SchedulerStatus == "Waiting" {
				return fmt.Errorf("task %s run on node %s job only run mode %s", taskID, nodeID, job.OnlyOnce)
			}
			if oldjob.Scheduler != nil && oldjob.Scheduler.SchedulerStatus == "Success" {
				if oldjob.RunStatus != nil && oldjob.RunStatus.Status == "Success" {
					return fmt.Errorf("task %s run on node %s job only run mode %s", taskID, nodeID, job.OnlyOnce)
				}
			}
		}
	}
	jb, err := job.CreateJobFromTask(task)
	if err != nil {
		return fmt.Errorf("create job error,%s", err.Error())
	}
	jb.NodeID = nodeID
	jb.Hash = hash
	jb.Scheduler = nil
	return job.PutJob(jb)
}

//GetJob 获取已经存在的job
func (t *TaskEngine) GetJob(hash string) *job.Job {
	if j, ok := t.jobs[hash]; ok {
		return j
	}
	return nil
}

func getHash(source ...string) string {
	h := sha1.New()
	for _, s := range source {
		h.Write([]byte(s))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

//ScheduleGroup 调度执行指定task
func (t *TaskEngine) ScheduleGroup(nextGroups *model.TaskGroup, node string) error {
	//TODO:调度组任务
	return nil
}

//StopTask 停止任务，即删除任务对应的JOB
func (t *TaskEngine) StopTask(task *model.Task, node string) {
	if status, ok := task.Status[node]; ok {
		if status.JobID != "" {
			_, err := store.DefalutClient.Delete(t.config.JobPath + "/" + status.JobID)
			if err != nil {
				logrus.Errorf("stop task %s error.%s", task.Name, err.Error())
			}
			_, err = store.DefalutClient.Delete(t.config.ExecutionRecordPath+"/"+status.JobID, client.WithPrefix())
			if err != nil {
				logrus.Errorf("delete execution record for task %s error.%s", task.Name, err.Error())
			}
		}
	}
}
