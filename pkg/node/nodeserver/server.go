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

package nodeserver

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	conf "github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/core/job"
	"github.com/goodrain/rainbond/pkg/node/core/store"
	"github.com/goodrain/rainbond/pkg/util"
	"github.com/robfig/cron"
	"github.com/shunfei/cronsun/utils"

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

//NodeServer node manager server
type NodeServer struct {
	*store.Client
	*model.HostNode
	*cron.Cron
	jobs     Jobs // 和结点相关的任务
	onceJobs Jobs //记录执行的单任务
	jobLock  sync.Mutex
	cmds     map[string]*job.Cmd
	// 删除的 job id，用于 group 更新
	delIDs     map[string]bool
	ttl        int64
	lID        client.LeaseID // lease id
	regLeaseID client.LeaseID // lease id
	done       chan struct{}
	//Config
	*conf.Conf
}

func (n *NodeServer) set() error {
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
	return nil
}

//Run 启动
func (n *NodeServer) Run() (err error) {
	go n.keepAlive()
	defer func() {
		if err != nil {
			n.Stop(nil)
		}
	}()
	if err = n.loadJobs(); err != nil {
		return
	}
	n.Cron.Start()
	go n.watchJobs()
	go n.watchOnce()
	logrus.Info("node registe success")
	if err := job.StartProc(); err != nil {
		logrus.Warnf("[process key will not timeout]proc lease id set err: %s", err.Error())
	}
	return
}
func (n *NodeServer) loadJobs() (err error) {
	jobs, err := job.GetJobs()
	if err != nil {
		return err
	}
	if len(jobs) == 0 {
		return
	}
	for _, job := range jobs {
		job.Init(n.ID)
		n.addJob(job)
	}
	return
}
func (n *NodeServer) watchJobs() {
	rch := job.WatchJobs()
	for wresp := range rch {
		for _, ev := range wresp.Events {
			switch {
			case ev.IsCreate():
				j, err := job.GetJobFromKv(ev.Kv)
				if err != nil {
					logrus.Warnf("err: %s, kv: %s", err.Error(), ev.Kv.String())
					continue
				}
				j.Init(n.ID)
				n.addJob(j)
			case ev.IsModify():
				j, err := job.GetJobFromKv(ev.Kv)
				if err != nil {
					logrus.Warnf("err: %s, kv: %s", err.Error(), ev.Kv.String())
					continue
				}
				j.Init(n.ID)
				n.modJob(j)
			case ev.Type == client.EventTypeDelete:
				n.delJob(job.GetIDFromKey(string(ev.Kv.Key)))
			default:
				logrus.Warnf("unknown event type[%v] from job[%s]", ev.Type, string(ev.Kv.Key))
			}
		}
	}
}
func (n *NodeServer) addJob(j *job.Job) {
	if !j.IsRunOn(n.HostNode) {
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

func (n *NodeServer) modJob(job *job.Job) {
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

func (n *NodeServer) addCmd(cmd *job.Cmd) {
	n.Cron.Schedule(cmd.JobRule.Schedule, cmd)
	n.cmds[cmd.GetID()] = cmd
	logrus.Infof("job[%s] rule[%s] timer[%s] has added", cmd.Job.ID, cmd.JobRule.ID, cmd.JobRule.Timer)
	return
}

func (n *NodeServer) modCmd(cmd *job.Cmd) {
	c, ok := n.cmds[cmd.GetID()]
	if !ok {
		n.addCmd(cmd)
		return
	}
	sch := c.JobRule.Timer
	*c = *cmd
	// 节点执行时间改变，更新 cron
	// 否则不用更新 cron
	if c.JobRule.Timer != sch {
		n.Cron.Schedule(c.JobRule.Schedule, c)
	}
	logrus.Infof("job[%s] rule[%s] timer[%s] has updated", c.Job.ID, c.JobRule.ID, c.JobRule.Timer)
}

func (n *NodeServer) delCmd(cmd *job.Cmd) {
	delete(n.cmds, cmd.GetID())
	n.Cron.DelJob(cmd)
	logrus.Infof("job[%s] rule[%s] timer[%s] has deleted", cmd.Job.ID, cmd.JobRule.ID, cmd.JobRule.Timer)
}
func (n *NodeServer) watchOnce() {
	rch := job.WatchOnce()
	for wresp := range rch {
		for _, ev := range wresp.Events {
			switch {
			case ev.IsCreate():
				j, err := job.GetJobFromKv(ev.Kv)
				if err != nil {
					logrus.Warnf("err: %s, kv: %s", err.Error(), ev.Kv.String())
					continue
				}
				j.Init(n.ID)
				if !j.IsRunOn(n.HostNode) {
					continue
				}
				if !j.IsOnce {
					continue
				}
				go j.RunWithRecovery()
			}
		}
	}
}
func (n *NodeServer) watchBuildIn() {

	//todo 在这里给<-channel,如果没有，立刻返回,可以用无循环switch，default实现
	// rch := job.WatchBuildIn()
	// for wresp := range rch {
	// 	for _, ev := range wresp.Events {
	// 		switch {
	// 		case ev.IsCreate() || ev.IsModify():
	// 			canRun := store.DefalutClient.IsRunnable("/acp_node/runnable/" + n.ID)
	// 			if !canRun {
	// 				logrus.Infof("job can't run on node %s,skip", n.ID)
	// 				continue
	// 			}

	// 			logrus.Infof("new build-in job to run ,key is %s,local ip is %s", ev.Kv.Key, n.ID)
	// 			job := &job.Job{}
	// 			k := string(ev.Kv.Key)
	// 			paths := strings.Split(k, "/")

	// 			ps := strings.Split(paths[len(paths)-1], "-")
	// 			buildInJobId := ps[0]
	// 			jobResp, err := store.DefalutClient.Get(conf.Config.BuildIn + buildInJobId)
	// 			if err != nil {
	// 				logrus.Warnf("get build-in job failed")
	// 			}
	// 			json.Unmarshal(jobResp.Kvs[0].Value, job)

	// 			job.Init(n.ID)
	// 			//job.Check()
	// 			err = job.ResolveShell()
	// 			if err != nil {
	// 				logrus.Infof("resolve shell to runnable failed , details %s", err.Error())
	// 			}
	// 			n.addJob(job)

	// 			//logrus.Infof("is ok? %v and is job runing on %v",ok,job.IsRunOn(n.ID, n.groups))
	// 			////if !ok || !job.IsRunOn(n.ID, n.groups) {
	// 			////	continue
	// 			////}
	// 			for _, v := range job.Rules {
	// 				for _, v2 := range v.NodeIDs {
	// 					if v2 == n.ID {
	// 						logrus.Infof("prepare run new build-in job")
	// 						go job.RunBuildInWithRecovery(n.ID)
	// 						go n.watchBuildIn()
	// 						return
	// 					}
	// 				}
	// 			}

	// 		}
	// 	}
	// }
}

//Stop 停止服务
func (n *NodeServer) Stop(i interface{}) {
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
			if err := n.set(); err != nil {
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
	n := &NodeServer{
		Client:   store.DefalutClient,
		HostNode: currentNode,
		Cron:     cron.New(),
		jobs:     make(Jobs, 8),
		onceJobs: make(Jobs, 8),
		cmds:     make(map[string]*job.Cmd),
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
			ip, err := utils.LocalIP()
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
	node.Labels["rainbond_node_hostname"] = node.HostName
	node.Labels["rainbond_node_ip"] = node.InternalIP
	return &node, nil
}

//CreateNode 创建节点信息
func CreateNode(cfg *conf.Conf, nodeID, ip string) model.HostNode {
	hostname, _ := os.Hostname()
	HostNode := model.HostNode{
		ID: nodeID,
		ClusterNode: model.ClusterNode{
			PID: strconv.Itoa(os.Getpid()),
		},
		InternalIP: ip,
		ExternalIP: ip,
		HostName:   hostname,
	}
	return HostNode
}
