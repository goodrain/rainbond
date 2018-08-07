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

package taskrun

import (
	"context"
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
	clientv3 "github.com/coreos/etcd/clientv3"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/core/job"
	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/util/watch"
	"github.com/robfig/cron"
)

//Manager Manager
type Manager interface {
	Start(errchan chan error)
	Stop() error
}

//Jobs jobs
type Jobs map[string]*job.Job

//manager node manager server
type manager struct {
	cluster  client.ClusterClient
	etcdcli  *clientv3.Client
	HostNode *client.HostNode
	*cron.Cron
	ctx      context.Context
	cancel   context.CancelFunc
	Conf     *option.Conf
	jobs     Jobs // 和结点相关的任务
	onceJobs Jobs //记录执行的单任务
	jobLock  sync.Mutex
	cmds     map[string]*job.Cmd
	delIDs   map[string]bool
	ttl      int64
}

//Run taskrun start
func (n *manager) Start(errchan chan error) {
	go n.watchJobs(errchan)
	n.Cron.Start()
	return
}

func (n *manager) watchJobs(errChan chan error) error {
	watcher := watch.New(n.etcdcli, "")
	watchChan, err := watcher.WatchList(n.ctx, n.Conf.JobPath, "")
	if err != nil {
		errChan <- err
		return err
	}
	defer watchChan.Stop()
	for event := range watchChan.ResultChan() {
		switch event.Type {
		case watch.Added:
			j := new(job.Job)
			//fmt.Println(string(event.GetValue()))
			err := j.Decode(event.GetValue())
			if err != nil {
				logrus.Errorf("decode job error :%s", err)
				continue
			}
			n.addJob(j)
		case watch.Modified:
			j := new(job.Job)
			fmt.Println(string(event.GetValue()))
			err := j.Decode(event.GetValue())
			if err != nil {
				logrus.Errorf("decode job error :%s", err)
				continue
			}
			n.modJob(j)
		case watch.Deleted:
			n.delJob(event.GetKey())
		default:
			logrus.Errorf("watch job error:%v", event.Error)
			errChan <- event.Error
		}
	}
	return nil
}

//添加job缓存
func (n *manager) addJob(j *job.Job) {
	if !j.IsRunOn(n.HostNode.ID) {
		return
	}
	//一次性任务
	if j.Rules.Mode != job.Cycle {
		n.runOnceJob(j)
		return
	}
	n.jobLock.Lock()
	defer n.jobLock.Unlock()
	n.jobs[j.ID] = j
	cmds := j.Cmds()
	if len(cmds) == 0 {
		return
	}
	for _, cmd := range cmds {
		n.addCmd(cmd)
	}
	return
}

func (n *manager) delJob(id string) {
	n.jobLock.Lock()
	defer n.jobLock.Unlock()
	n.delIDs[id] = true
	job, ok := n.jobs[id]
	// 之前此任务没有在当前结点执行
	if !ok {
		return
	}
	cmds := job.Cmds()
	if len(cmds) == 0 {
		return
	}
	for _, cmd := range cmds {
		n.delCmd(cmd)
	}
	delete(n.jobs, id)
	return
}

func (n *manager) modJob(jobb *job.Job) {
	if !jobb.IsRunOn(n.HostNode.ID) {
		return
	}
	//一次性任务
	if jobb.Rules.Mode != job.Cycle {
		n.runOnceJob(jobb)
		return
	}
	oJob, ok := n.jobs[jobb.ID]
	// 之前此任务没有在当前结点执行，直接增加任务
	if !ok {
		n.addJob(jobb)
		return
	}
	prevCmds := oJob.Cmds()

	jobb.Count = oJob.Count
	*oJob = *jobb
	cmds := oJob.Cmds()
	for id, cmd := range cmds {
		n.modCmd(cmd)
		delete(prevCmds, id)
	}
	for _, cmd := range prevCmds {
		n.delCmd(cmd)
	}
}

func (n *manager) addCmd(cmd *job.Cmd) {
	n.Cron.Schedule(cmd.Rule.Schedule, cmd)
	n.cmds[cmd.GetID()] = cmd
	logrus.Infof("job[%s] rule[%s] timer[%s] has added", cmd.Job.ID, cmd.Rule.ID, cmd.Rule.Timer)
	return
}

func (n *manager) modCmd(cmd *job.Cmd) {
	c, ok := n.cmds[cmd.GetID()]
	if !ok {
		n.addCmd(cmd)
		return
	}
	sch := c.Rule.Timer
	*c = *cmd
	// 节点执行时间改变，更新 cron
	// 否则不用更新 cron
	if c.Rule.Timer != sch {
		n.Cron.Schedule(c.Rule.Schedule, c)
	}
	logrus.Infof("job[%s] rule[%s] timer[%s] has updated", c.Job.ID, c.Rule.ID, c.Rule.Timer)
}

func (n *manager) delCmd(cmd *job.Cmd) {
	delete(n.cmds, cmd.GetID())
	n.Cron.DelJob(cmd)
	logrus.Infof("job[%s] rule[%s] timer[%s] has deleted", cmd.Job.ID, cmd.Rule.ID, cmd.Rule.Timer)
}

//job must be schedulered
func (n *manager) runOnceJob(j *job.Job) {
	go j.RunWithRecovery()
}

//Stop 停止服务
func (n *manager) Stop() error {
	n.cancel()
	n.Cron.Stop()
	return nil
}

//Newmanager new server
func Newmanager(cfg *option.Conf, etcdCli *clientv3.Client) (Manager, error) {
	if cfg.TTL == 0 {
		cfg.TTL = 10
	}
	ctx, cancel := context.WithCancel(context.Background())
	n := &manager{
		ctx:      ctx,
		cancel:   cancel,
		Cron:     cron.New(),
		jobs:     make(Jobs, 8),
		onceJobs: make(Jobs, 8),
		cmds:     make(map[string]*job.Cmd),
		delIDs:   make(map[string]bool, 8),
		ttl:      cfg.TTL,
		etcdcli:  etcdCli,
		Conf:     cfg,
	}
	return n, nil
}
