
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

package node

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	conf "github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/node/core"
	corenode "github.com/goodrain/rainbond/pkg/node/core/node"
	"github.com/goodrain/rainbond/pkg/node/core/store"
	"github.com/goodrain/rainbond/pkg/node/node/cron"
	"github.com/goodrain/rainbond/pkg/node/utils"

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
	Ttl                  int64
}

//NodeServer node manager server
type NodeServer struct {
	*store.Client
	*corenode.Node
	*cron.Cron

	jobs   Jobs // 和结点相关的任务
	groups Groups
	cmds   map[string]*core.Cmd

	link
	// 删除的 job id，用于 group 更新
	delIDs map[string]bool

	ttl        int64
	lID        client.LeaseID // lease id
	regLeaseID client.LeaseID // lease id
	done       chan struct{}

	//Config
	*conf.Conf
	//LogLevel string
	//RunMode  string //master,node
}

//Register 注册节点
func (n *NodeServer) Register() (err error) {
	pid, err := n.Node.Exist()
	if err != nil {
		return
	}
	if pid != -1 {
		return fmt.Errorf("node[%s] pid[%d] exist", n.Node.ID, pid)
	}
	logrus.Info("creating new node ", n.Node.ID)

	//if n.RunMode == "worker" {//todo管理节点上线的时候需要注释掉这个if
	//	//注册自己到 build-in 可执行范围内
	//	_,err=core.NewComputeNodeToInstall(n.ID)
	//	if err != nil {
	//		logrus.Warnf("reg node %s to build-in jobs failed,details: %s",n.ID,err.Error())
	//	}
	//}else {//master节点肯定有一个先于所有worker节点
	//	//check exist避免多次注册 exist由方法实现负责。
	//	err=core.RegWorkerInstallJobs()
	//	if err!=nil {
	//		logrus.Errorf("reg build-in jobs failed,details: %s",err.Error())
	//	}
	//}
	return n.set()
}

func (n *NodeServer) set() error {
	resp, err := n.Client.Grant(n.ttl + 2)
	if err != nil {
		return err
	}
	if n.RunMode == "master" {
		//n.Node.Put()
		n.Node.PutMaster(client.WithLease(resp.ID))
	}
	if _, err = n.Node.Put(client.WithLease(resp.ID)); err != nil {
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
	go n.watchGroups()
	go n.watchOnce()
	//可以在n里面加一个channel，用于锁定
	go n.watchBuildIn()

	err = core.RegWorkerInstallJobs()
	if err != nil {
		logrus.Errorf("reg build-in jobs failed,details: %s", err.Error())
	}
	return
}
func (n *NodeServer) loadJobs() (err error) {
	if n.groups, err = core.GetGroups(""); err != nil {
		return
	}

	jobs, err := core.GetJobs()
	if err != nil {
		return
	}

	if len(jobs) == 0 {
		return
	}

	for _, job := range jobs {
		job.Init(n.ID)
		n.addJob(job, false)
	}

	return
}
func (n *NodeServer) watchJobs() {
	rch := core.WatchJobs()
	for wresp := range rch {
		for _, ev := range wresp.Events {
			switch {
			case ev.IsCreate():
				job, err := core.GetJobFromKv(ev.Kv)
				if err != nil {
					logrus.Warnf("err: %s, kv: %s", err.Error(), ev.Kv.String())
					continue
				}
				job.Init(n.ID)
				n.addJob(job, true)
			case ev.IsModify():
				job, err := core.GetJobFromKv(ev.Kv)
				if err != nil {
					logrus.Warnf("err: %s, kv: %s", err.Error(), ev.Kv.String())
					continue
				}
				job.Init(n.ID)
				n.modJob(job)
			case ev.Type == client.EventTypeDelete:
				n.delJob(core.GetIDFromKey(string(ev.Kv.Key)))
			default:
				logrus.Warnf("unknown event type[%v] from job[%s]", ev.Type, string(ev.Kv.Key))
			}
		}
	}
}

func (n *NodeServer) watchGroups() {
	rch := core.WatchGroups()
	for wresp := range rch {
		for _, ev := range wresp.Events {
			switch {
			case ev.IsCreate():
				g, err := core.GetGroupFromKv(ev.Kv)
				if err != nil {
					logrus.Warnf("err: %s, kv: %s", err.Error(), ev.Kv.String())
					continue
				}

				n.addGroup(g)
			case ev.IsModify():
				g, err := core.GetGroupFromKv(ev.Kv)
				if err != nil {
					logrus.Warnf("err: %s, kv: %s", err.Error(), ev.Kv.String())
					continue
				}

				n.modGroup(g)
			case ev.Type == client.EventTypeDelete:
				n.delGroup(core.GetIDFromKey(string(ev.Kv.Key)))
			default:
				logrus.Warnf("unknown event type[%v] from group[%s]", ev.Type, string(ev.Kv.Key))
			}
		}
	}
}

func (n *NodeServer) addGroup(g *core.Group) {
	n.groups[g.ID] = g
}

func (n *NodeServer) delGroup(id string) {
	delete(n.groups, id)
	n.link.delGroup(id)

	job, ok := n.jobs[id]
	// 之前此任务没有在当前结点执行
	if !ok {
		return
	}

	cmds := job.Cmds(n.ID, n.groups)
	if len(cmds) == 0 {
		return
	}

	for _, cmd := range cmds {
		n.delCmd(cmd)
	}
	return
}

func (n *NodeServer) modGroup(g *core.Group) {
	oGroup, ok := n.groups[g.ID]
	if !ok {
		n.addGroup(g)
		return
	}

	// 都包含/都不包含当前节点，对当前节点任务无影响
	if (oGroup.Included(n.ID) && g.Included(n.ID)) || (!oGroup.Included(n.ID) && !g.Included(n.ID)) {
		*oGroup = *g
		return
	}

	// 增加当前节点
	if !oGroup.Included(n.ID) && g.Included(n.ID) {
		n.groupAddNode(g)
		return
	}

	// 移除当前节点
	n.groupRmNode(g, oGroup)
	return
}
func (n *NodeServer) groupAddNode(g *core.Group) {
	n.groups[g.ID] = g
	jls := n.link[g.ID]
	if len(jls) == 0 {
		return
	}

	var err error
	for jid, jl := range jls {
		job, ok := n.jobs[jid]
		if !ok {
			// job 已删除
			if n.delIDs[jid] {
				n.link.delGroupJob(g.ID, jid)
				continue
			}

			if job, err = core.GetJob(jl.gname, jid); err != nil {
				logrus.Warnf("get job[%s][%s] err: %s", jl.gname, jid, err.Error())
				n.link.delGroupJob(g.ID, jid)
				continue
			}
			job.Init(n.ID)
		}

		cmds := job.Cmds(n.ID, n.groups)
		for _, cmd := range cmds {
			n.addCmd(cmd, true)
		}
	}
	return
}

func (n *NodeServer) groupRmNode(g, og *core.Group) {
	jls := n.link[g.ID]
	if len(jls) == 0 {
		n.groups[g.ID] = g
		return
	}

	for jid, _ := range jls {
		job, ok := n.jobs[jid]
		// 之前此任务没有在当前结点执行
		if !ok {
			n.link.delGroupJob(g.ID, jid)
			continue
		}

		n.groups[og.ID] = og
		prevCmds := job.Cmds(n.ID, n.groups)
		n.groups[g.ID] = g
		cmds := job.Cmds(n.ID, n.groups)

		for id, cmd := range cmds {
			n.addCmd(cmd, true)
			delete(prevCmds, id)
		}

		for _, cmd := range prevCmds {
			n.delCmd(cmd)
		}
	}

	n.groups[g.ID] = g
}
func (n *NodeServer) addJob(job *core.Job, notice bool) {
	n.link.addJob(job)
	if job.IsRunOn(n.ID, n.groups) {
		n.jobs[job.ID] = job
	}

	cmds := job.Cmds(n.ID, n.groups)
	if len(cmds) == 0 {
		return
	}

	for _, cmd := range cmds {
		n.addCmd(cmd, notice)
	}
	return
}

func (n *NodeServer) delJob(id string) {
	n.delIDs[id] = true
	job, ok := n.jobs[id]
	// 之前此任务没有在当前结点执行
	if !ok {
		return
	}

	delete(n.jobs, id)
	n.link.delJob(job)

	cmds := job.Cmds(n.ID, n.groups)
	if len(cmds) == 0 {
		return
	}

	for _, cmd := range cmds {
		n.delCmd(cmd)
	}
	return
}

func (n *NodeServer) modJob(job *core.Job) {
	oJob, ok := n.jobs[job.ID]
	// 之前此任务没有在当前结点执行，直接增加任务
	if !ok {
		n.addJob(job, true)
		return
	}

	n.link.delJob(oJob)
	prevCmds := oJob.Cmds(n.ID, n.groups)

	job.Count = oJob.Count
	*oJob = *job
	cmds := oJob.Cmds(n.ID, n.groups)

	for id, cmd := range cmds {
		n.modCmd(cmd, true)
		delete(prevCmds, id)
	}

	for _, cmd := range prevCmds {
		n.delCmd(cmd)
	}

	n.link.addJob(oJob)
}

func (n *NodeServer) addCmd(cmd *core.Cmd, notice bool) {
	n.Cron.Schedule(cmd.JobRule.Schedule, cmd)
	n.cmds[cmd.GetID()] = cmd

	if notice {
		logrus.Infof("job[%s] group[%s] rule[%s] timer[%s] has added", cmd.Job.ID, cmd.Job.Group, cmd.JobRule.ID, cmd.JobRule.Timer)
	}
	return
}

func (n *NodeServer) modCmd(cmd *core.Cmd, notice bool) {
	c, ok := n.cmds[cmd.GetID()]
	if !ok {
		n.addCmd(cmd, notice)
		return
	}

	sch := c.JobRule.Timer
	*c = *cmd

	// 节点执行时间改变，更新 cron
	// 否则不用更新 cron
	if c.JobRule.Timer != sch {
		n.Cron.Schedule(c.JobRule.Schedule, c)
	}

	if notice {
		logrus.Infof("job[%s] group[%s] rule[%s] timer[%s] has updated", c.Job.ID, c.Job.Group, c.JobRule.ID, c.JobRule.Timer)
	}
}

func (n *NodeServer) delCmd(cmd *core.Cmd) {
	delete(n.cmds, cmd.GetID())
	n.Cron.DelJob(cmd)
	logrus.Infof("job[%s] group[%s] rule[%s] timer[%s] has deleted", cmd.Job.ID, cmd.Job.Group, cmd.JobRule.ID, cmd.JobRule.Timer)
}
func (n *NodeServer) watchOnce() {
	rch := core.WatchOnce()
	for wresp := range rch {
		for _, ev := range wresp.Events {
			switch {
			case ev.IsCreate(), ev.IsModify():
				if len(ev.Kv.Value) != 0 && string(ev.Kv.Value) != n.ID {
					continue
				}

				job, ok := n.jobs[core.GetIDFromKey(string(ev.Kv.Key))]
				if !ok || !job.IsRunOn(n.ID, n.groups) {
					continue
				}

				go job.RunWithRecovery()
			}
		}
	}
}
func (n *NodeServer) watchBuildIn() {

	//todo 在这里给<-channel,如果没有，立刻返回,可以用无循环switch，default实现

	rch := core.WatchBuildIn()
	for wresp := range rch {
		for _, ev := range wresp.Events {
			switch {
			case ev.IsCreate() || ev.IsModify():
				canRun := store.DefalutClient.IsRunnable("/acp_node/runnable/" + n.ID)
				if !canRun {
					logrus.Infof("job can't run on node %s,skip", n.ID)
					continue
				}

				logrus.Infof("new build-in job to run ,key is %s,local ip is %s", ev.Kv.Key, n.ID)
				job := &core.Job{}
				k := string(ev.Kv.Key)
				paths := strings.Split(k, "/")

				ps := strings.Split(paths[len(paths)-1], "-")
				buildInJobId := ps[0]
				jobResp, err := store.DefalutClient.Get(conf.Config.BuildIn + buildInJobId)
				if err != nil {
					logrus.Warnf("get build-in job failed")
				}
				json.Unmarshal(jobResp.Kvs[0].Value, job)

				job.Init(n.ID)
				//job.Check()
				err = job.ResolveShell()
				if err != nil {
					logrus.Infof("resolve shell to runnable failed , details %s", err.Error())
				}
				n.addJob(job, false)

				//logrus.Infof("is ok? %v and is job runing on %v",ok,job.IsRunOn(n.ID, n.groups))
				////if !ok || !job.IsRunOn(n.ID, n.groups) {
				////	continue
				////}
				for _, v := range job.Rules {
					for _, v2 := range v.NodeIDs {
						if v2 == n.ID {
							logrus.Infof("prepare run new build-in job")
							go job.RunBuildInWithRecovery(n.ID)
							go n.watchBuildIn()
							return
						}
					}
				}

			}
		}
	}
}

//Stop 停止服务
func (n *NodeServer) Stop(i interface{}) {
	n.Node.Down()
	close(n.done)
	n.Node.Del()
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

				logrus.Warnf("%s lid[%x] keepAlive err: %s, try to reset...", n.String(), n.lID, err.Error())
				n.lID = 0
			}

			if err := n.set(); err != nil {
				logrus.Warnf("%s set lid err: %s, try to reset after %d seconds...", n.String(), err.Error(), n.ttl)
			} else {
				logrus.Infof("%s set lid[%x] success", n.String(), n.lID)
			}
			timer.Reset(duration)
		}
	}
}
func (m *NodeServer) httpKey() string {
	return fmt.Sprintf("/traefik/backends/acp_node/servers/%s/url", m.ID)
}
func (m *NodeServer) httpValue() string {
	return m.ID + conf.Config.APIAddr
}

func (m *NodeServer) Reg() error {
	resp, err := m.Client.Grant(m.ttl + 2)
	if err != nil {
		return err
	}
	if _, err := m.Client.Put(
		m.httpKey(),
		m.httpValue(),
		client.WithLease(resp.ID)); err != nil {
		return err
	}
	m.regLeaseID = resp.ID
	return nil
}

func (m *NodeServer) RegKeepAlive() {
	duration := time.Duration(m.ttl) * time.Second
	timer := time.NewTimer(duration)
	for {
		select {
		case <-m.done:
			return
		case <-timer.C:
			if m.regLeaseID > 0 {
				_, err := m.Client.KeepAliveOnce(m.regLeaseID)
				if err == nil {
					timer.Reset(duration)
					continue
				}
				logrus.Warnf("%s lid[%x] keepAlive err: %s, try to reset...", m.ID, m.regLeaseID, err.Error())
				m.regLeaseID = 0
			}
			if err := m.Reg(); err != nil {
				logrus.Warnf("%s set lid err: %s, try to reset after %d seconds...", m.ID, err.Error(), m.ttl)
			} else {
				logrus.Infof("%s set lid[%x] success", m.ID, m.regLeaseID)
			}
			timer.Reset(duration)
		}
	}

}

//NewNodeServer new server
func NewNodeServer(cfg *conf.Conf) (*NodeServer, error) {
	ip, err := utils.LocalIP()
	if err != nil {
		return nil, err
	}

	n := &NodeServer{
		Client: store.DefalutClient,
		Node: &corenode.Node{
			ID:  ip.String(),
			PID: strconv.Itoa(os.Getpid()),
		},
		Cron: cron.New(),

		jobs: make(Jobs, 8),
		cmds: make(map[string]*core.Cmd),

		link:   newLink(8),
		delIDs: make(map[string]bool, 8),
		Conf:   cfg,
		ttl:    cfg.Ttl,

		done: make(chan struct{}),
	}
	return n, nil
}
