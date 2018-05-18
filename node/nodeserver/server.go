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

package nodeserver

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goodrain/rainbond/node/core/job"

	"github.com/goodrain/rainbond/util/watch"

	conf "github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/node/api/model"
	corejob "github.com/goodrain/rainbond/node/core/job"
	"github.com/goodrain/rainbond/node/core/store"
	"github.com/goodrain/rainbond/util"
	"github.com/robfig/cron"

	"github.com/Sirupsen/logrus"
	client "github.com/coreos/etcd/clientv3"
)

//Config config server
type Config struct {
	EtcdEndPoints        []string
	EtcdTimeout          int
	EtcdPrefix           string
	ClusterName          string
	APIAddr              string
	K8SConfPath          string
	EventServerAddress   []string
	PrometheusMetricPath string
	TTL                  int64
}

//Jobs jobs
type Jobs map[string]*corejob.Job

//NodeServer node manager server
type NodeServer struct {
	*store.Client
	*model.HostNode
	*cron.Cron
	ctx      context.Context
	cancel   context.CancelFunc
	jobs     Jobs // 和结点相关的任务
	onceJobs Jobs //记录执行的单任务
	jobLock  sync.Mutex
	cmds     map[string]*corejob.Cmd
	// 删除的 job id，用于 group 更新
	delIDs     map[string]bool
	ttl        int64
	lID        client.LeaseID // lease id
	regLeaseID client.LeaseID // lease id
	done       chan struct{}
	//Config
	*conf.Conf
}

//Regist 节点注册
func (n *NodeServer) Regist() error {
	resp, err := n.Client.Grant(n.ttl + 2)
	if err != nil {
		return err
	}
	if _, err = n.HostNode.Update(); err != nil {
		return err
	}
	if _, err = n.HostNode.Put(client.WithLease(resp.ID)); err != nil {
		return err
	}
	n.lID = resp.ID
	logrus.Infof("node(%s) registe success", n.HostName)
	return nil
}

//Run 启动
func (n *NodeServer) Run(errchan chan error) (err error) {
	n.ctx, n.cancel = context.WithCancel(context.Background())
	n.Regist()
	go n.keepAlive()
	go n.watchJobs(errchan)
	n.Cron.Start()
	if err := corejob.StartProc(); err != nil {
		logrus.Warnf("[process key will not timeout]proc lease id set err: %s", err.Error())
	}
	return
}

func (n *NodeServer) watchJobs(errChan chan error) error {
	watcher := watch.New(store.DefalutClient.Client, "")
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
			err := j.Decode(event.GetValue())
			if err != nil {
				logrus.Errorf("decode job error :%s", err)
				continue
			}
			n.addJob(j)
		case watch.Modified:
			j := new(job.Job)
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
func (n *NodeServer) addJob(j *corejob.Job) {
	if !j.IsRunOn(n.HostNode) {
		return
	}
	//一次性任务
	if j.Rules.Mode != corejob.Cycle {
		n.runOnceJob(j)
		return
	}
	n.jobLock.Lock()
	defer n.jobLock.Unlock()
	n.jobs[j.ID] = j
	cmds := j.Cmds(n.HostNode)
	if len(cmds) == 0 {
		return
	}
	for _, cmd := range cmds {
		n.addCmd(cmd)
	}
	return
}

func (n *NodeServer) delJob(id string) {
	n.jobLock.Lock()
	defer n.jobLock.Unlock()
	n.delIDs[id] = true
	job, ok := n.jobs[id]
	// 之前此任务没有在当前结点执行
	if !ok {
		return
	}
	cmds := job.Cmds(n.HostNode)
	if len(cmds) == 0 {
		return
	}
	for _, cmd := range cmds {
		n.delCmd(cmd)
	}
	delete(n.jobs, id)
	return
}

func (n *NodeServer) modJob(job *corejob.Job) {
	if !job.IsRunOn(n.HostNode) {
		return
	}
	//一次性任务
	if job.Rules.Mode != corejob.Cycle {
		n.runOnceJob(job)
		return
	}
	oJob, ok := n.jobs[job.ID]
	// 之前此任务没有在当前结点执行，直接增加任务
	if !ok {
		n.addJob(job)
		return
	}
	prevCmds := oJob.Cmds(n.HostNode)

	job.Count = oJob.Count
	*oJob = *job
	cmds := oJob.Cmds(n.HostNode)
	for id, cmd := range cmds {
		n.modCmd(cmd)
		delete(prevCmds, id)
	}
	for _, cmd := range prevCmds {
		n.delCmd(cmd)
	}
}

func (n *NodeServer) addCmd(cmd *corejob.Cmd) {
	n.Cron.Schedule(cmd.Rule.Schedule, cmd)
	n.cmds[cmd.GetID()] = cmd
	logrus.Infof("job[%s] rule[%s] timer[%s] has added", cmd.Job.ID, cmd.Rule.ID, cmd.Rule.Timer)
	return
}

func (n *NodeServer) modCmd(cmd *corejob.Cmd) {
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

func (n *NodeServer) delCmd(cmd *corejob.Cmd) {
	delete(n.cmds, cmd.GetID())
	n.Cron.DelJob(cmd)
	logrus.Infof("job[%s] rule[%s] timer[%s] has deleted", cmd.Job.ID, cmd.Rule.ID, cmd.Rule.Timer)
}

//job must be schedulered
func (n *NodeServer) runOnceJob(j *corejob.Job) {
	go j.RunWithRecovery()
}

//Stop 停止服务
func (n *NodeServer) Stop(i interface{}) {
	n.cancel()
	n.HostNode.Down()
	close(n.done)
	n.HostNode.Del()
	n.Client.Close()
	n.Cron.Stop()
}

func (n *NodeServer) keepAlive() {
	duration := time.Duration(n.ttl) * time.Second
	timer := time.NewTimer(duration)
	for {
		select {
		case <-n.done:
			timer.Stop()
			return
		case <-timer.C:
			if n.lID > 0 {
				_, err := n.Client.KeepAliveOnce(n.lID)
				if err == nil {
					timer.Reset(duration)
					continue
				}
				logrus.Warnf("%s lid[%x] keepAlive err: %s, try to reset...", n.HostName, n.lID, err.Error())
				n.lID = 0
			}
			if err := n.Regist(); err != nil {
				logrus.Warnf("%s set lid err: %s, try to reset after %d seconds...", n.HostName, err.Error(), n.ttl)
			} else {
				logrus.Infof("%s set lid[%x] success", n.HostName, n.lID)
			}
			timer.Reset(duration)
		}
	}
}

//NewNodeServer new server
func NewNodeServer(cfg *conf.Conf) (*NodeServer, error) {
	currentNode, err := GetCurrentNode(cfg)
	if err != nil {
		return nil, err
	}
	if cfg.TTL == 0 {
		cfg.TTL = 10
	}
	n := &NodeServer{
		Client:   store.DefalutClient,
		HostNode: currentNode,
		Cron:     cron.New(),
		jobs:     make(Jobs, 8),
		onceJobs: make(Jobs, 8),
		cmds:     make(map[string]*corejob.Cmd),
		delIDs:   make(map[string]bool, 8),
		Conf:     cfg,
		ttl:      cfg.TTL,
		done:     make(chan struct{}),
	}
	return n, nil
}

//GetCurrentNode 获取当前节点
func GetCurrentNode(cfg *conf.Conf) (*model.HostNode, error) {
	uid, err := util.ReadHostID(cfg.HostIDFile)
	if err != nil {
		return nil, fmt.Errorf("Get host id error:%s", err.Error())
	}
	res, err := store.DefalutClient.Get(cfg.NodePath + "/" + uid)
	if err != nil {
		return nil, fmt.Errorf("Get host info error:%s", err.Error())
	}
	var node model.HostNode
	if res.Count == 0 {
		if cfg.HostIP == "" {
			ip, err := util.LocalIP()
			if err != nil {
				return nil, err
			}
			cfg.HostIP = ip.String()
		}
		node = CreateNode(cfg, uid, cfg.HostIP)
	} else {
		n := model.GetNodeFromKV(res.Kvs[0])
		if n == nil {
			return nil, fmt.Errorf("Get node info from etcd error")
		}
		node = *n
	}
	node.Role = strings.Split(cfg.NodeRule, ",")
	if node.Labels == nil || len(node.Labels) < 1 {
		node.Labels = map[string]string{}
	}
	for _, rule := range node.Role {
		node.Labels["rainbond_node_rule_"+rule] = "true"
	}
	if node.HostName == "" {
		hostname, _ := os.Hostname()
		node.HostName = hostname
	}
	if node.ClusterNode.PID == "" {
		node.ClusterNode.PID = strconv.Itoa(os.Getpid())
	}
	node.Labels["rainbond_node_hostname"] = node.HostName
	node.Labels["rainbond_node_ip"] = node.InternalIP
	node.UpdataCondition(model.NodeCondition{
		Type:               model.NodeInit,
		Status:             model.ConditionTrue,
		LastHeartbeatTime:  time.Now(),
		LastTransitionTime: time.Now(),
	})
	node.Mode = cfg.RunMode
	return &node, nil
}

//CreateNode 创建节点信息
func CreateNode(cfg *conf.Conf, nodeID, ip string) model.HostNode {
	HostNode := model.HostNode{
		ID: nodeID,
		ClusterNode: model.ClusterNode{
			PID:        strconv.Itoa(os.Getpid()),
			Conditions: make([]model.NodeCondition, 0),
		},
		InternalIP: ip,
		ExternalIP: ip,
		CreateTime: time.Now(),
	}
	return HostNode
}
