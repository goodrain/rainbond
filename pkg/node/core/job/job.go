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
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	client "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	conf "github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/pkg/event"
	"github.com/goodrain/rainbond/pkg/node/api/model"
	"github.com/goodrain/rainbond/pkg/node/core/store"
	"github.com/goodrain/rainbond/pkg/node/utils"
	"github.com/robfig/cron"
	"github.com/twinj/uuid"
)

const (
	DefaultJobGroup = "default"
)

const (
	KindCommon   = iota
	KindAlone    // 任何时间段只允许单机执行
	KindInterval // 一个任务执行间隔内允许执行一次
)

//Job 需要执行的任务
type Job struct {
	ID      string     `json:"id"`
	EventID string     `json:"event_id"`
	Name    string     `json:"name"`
	Group   string     `json:"group"`
	Command string     `json:"cmd"`
	User    string     `json:"user"`
	Rules   []*JobRule `json:"rules"`
	Pause   bool       `json:"pause"`   // 可手工控制的状态
	Timeout int64      `json:"timeout"` // 任务执行时间超时设置，大于 0 时有效
	// 设置任务在单个节点上可以同时允许多少个
	// 针对两次任务执行间隔比任务执行时间要长的任务启用
	Parallels int64 `json:"parallels"`
	// 执行任务失败重试次数
	// 默认为 0，不重试
	Retry int `json:"retry"`
	// 执行任务失败重试时间间隔
	// 单位秒，如果不大于 0 则马上重试
	Interval int `json:"interval"`
	// 任务类型
	// 0: 普通任务
	// 1: 单机任务
	// 如果为单机任务，node 加载任务的时候 Parallels 设置 1
	Kind int `json:"kind"`
	// 平均执行时间，单位 ms
	AvgTime int64 `json:"avg_time"`
	// 执行失败发送通知
	FailNotify bool `json:"fail_notify"`
	// 发送通知地址
	To []string `json:"to"`

	// 执行任务的结点
	runOn string
	// 用于存储分隔后的任务
	cmd []string
	// 控制同时执行任务数
	Count *int64 `json:"-"`
}

//JobRule 任务规则
type JobRule struct {
	ID             string            `json:"id"`
	Timer          string            `json:"timer"`
	NodeIDs        []string          `json:"nids"`
	ExcludeNodeIDs []string          `json:"exclude_nids"`
	Labels         map[string]string `json:"labels"`
	Schedule       cron.Schedule     `json:"-"`
}

// 任务锁
type locker struct {
	kind  int
	ttl   int64
	lID   client.LeaseID
	timer *time.Timer
	done  chan struct{}
}

func (l *locker) keepAlive() {
	duration := time.Duration(l.ttl)*time.Second - 500*time.Millisecond
	l.timer = time.NewTimer(duration)
	for {
		select {
		case <-l.done:
			return
		case <-l.timer.C:
			_, err := store.DefalutClient.KeepAliveOnce(l.lID)
			if err != nil {
				logrus.Warnf("lock keep alive err: %s", err.Error())
				return
			}
			l.timer.Reset(duration)
		}
	}
}

func (l *locker) unlock() {
	if l.kind != KindAlone {
		return
	}

	close(l.done)
	l.timer.Stop()
	if _, err := store.DefalutClient.KeepAliveOnce(l.lID); err != nil {
		logrus.Warnf("unlock keep alive err: %s", err.Error())
	}
}

//Cmd 可执行任务
type Cmd struct {
	*Job
	*JobRule
}

//GetID GetID
func (c *Cmd) GetID() string {
	return c.Job.ID + c.JobRule.ID
}

//Run 执行
func (c *Cmd) Run() {
	// 同时执行任务数限制
	if c.Job.limit() {
		return
	}
	defer c.Job.unlimit()

	if c.Job.Kind != KindCommon {
		lk := c.lock()
		if lk == nil {
			return
		}
		defer lk.unlock()
	}

	if c.Job.Retry <= 0 {
		c.Job.Run("")
		return
	}

	for i := 0; i < c.Job.Retry; i++ {
		if c.Job.Run("") {
			return
		}
		if c.Job.Interval > 0 {
			time.Sleep(time.Duration(c.Job.Interval) * time.Second)
		}
	}
}

func (j *Job) limit() bool {
	if j.Parallels == 0 {
		return false
	}

	// 更精确的控制是加锁
	// 两次运行时间极为接近的任务才可能出现控制不精确的情况
	count := atomic.LoadInt64(j.Count)
	if j.Parallels <= count {
		j.Fail(time.Now(), fmt.Sprintf("job[%s] running on[%s] running:[%d]", j.Key(), j.runOn, count))
		return true
	}

	atomic.AddInt64(j.Count, 1)
	return false
}

func (j *Job) unlimit() {
	if j.Parallels == 0 {
		return
	}
	atomic.AddInt64(j.Count, -1)
}

//Init 初始化
func (j *Job) Init(n string) {
	var c int64
	j.Count, j.runOn = &c, n
}

//lockTTL
func (c *Cmd) lockTTL() int64 {
	now := time.Now()
	prev := c.JobRule.Schedule.Next(now)
	ttl := int64(c.JobRule.Schedule.Next(prev).Sub(prev) / time.Second)
	if ttl == 0 {
		return 0
	}

	if c.Job.Kind == KindInterval {
		ttl -= 2
		if ttl > conf.Config.LockTTL {
			ttl = conf.Config.LockTTL
		}
		if ttl < 1 {
			ttl = 1
		}
		return ttl
	}

	cost := c.Job.AvgTime / 1e3
	if c.Job.AvgTime/1e3-cost*1e3 > 0 {
		cost += 1
	}
	// 如果执行间隔时间不大于执行时间，把过期时间设置为执行时间的下限-1
	// 以便下次执行的时候，能获取到 lock
	if ttl >= cost {
		ttl -= cost
	}

	if ttl > conf.Config.LockTTL {
		ttl = conf.Config.LockTTL
	}

	// 支持的最小时间间隔 2s
	if ttl < 2 {
		ttl = 2
	}

	return ttl
}

func (c *Cmd) newLock() *locker {
	return &locker{
		kind: c.Job.Kind,
		ttl:  c.lockTTL(),
		done: make(chan struct{}),
	}
}

func (c *Cmd) lock() *locker {
	lk := c.newLock()
	// 非法的 rule
	if lk.ttl == 0 {
		return nil
	}

	resp, err := store.DefalutClient.Grant(lk.ttl)
	if err != nil {
		logrus.Infof("job[%s] didn't get a lock, err: %s", c.Job.Key(), err.Error())
		return nil
	}

	ok, err := store.DefalutClient.GetLock(c.Job.ID, resp.ID)
	if err != nil {
		logrus.Infof("job[%s] didn't get a lock, err: %s", c.Job.Key(), err.Error())
		return nil
	}

	if !ok {
		return nil
	}

	lk.lID = resp.ID
	if lk.kind == KindAlone {
		go lk.keepAlive()
	}
	return lk
}

//Valid 验证 timer 字段,创建Schedule
func (j *JobRule) Valid() error {
	// 注意 interface nil 的比较
	if j.Schedule != nil {
		return nil
	}

	if len(j.Timer) == 0 {
		return utils.ErrNilRule
	}
	sch, err := cron.Parse(j.Timer)
	if err != nil {
		return fmt.Errorf("invalid JobRule[%s], parse err: %s", j.Timer, err.Error())
	}
	j.Schedule = sch
	return nil
}

//included 当前节点是否符合规则
func (j *JobRule) included(node *model.HostNode) bool {
	//是否属于排除节点
	for _, excludeID := range j.ExcludeNodeIDs {
		if excludeID == node.ID {
			return false
		}
	}
	//是否属于允许节点
	for _, id := range j.NodeIDs {
		if id == node.ID {
			return true
		}
	}
	//是否匹配label
	for k, v := range j.Labels {
		if nodev := node.Labels[k]; nodev != v {
			return false
		}
	}
	return true
}

//GetJob get job
func GetJob(id string) (job *Job, err error) {
	job, _, err = GetJobAndRev(id)
	return
}

//GetJobAndRev get job
func GetJobAndRev(id string) (job *Job, rev int64, err error) {
	resp, err := store.DefalutClient.Get(CreateJobKey(id))
	if err != nil {
		return
	}

	if resp.Count == 0 {
		err = utils.ErrNotFound
		return
	}

	rev = resp.Kvs[0].ModRevision
	if err = json.Unmarshal(resp.Kvs[0].Value, &job); err != nil {
		return
	}

	job.splitCmd()
	return
}

//DeleteJob 删除job
func DeleteJob(id string) (resp *client.DeleteResponse, err error) {
	return store.DefalutClient.Delete(CreateJobKey(id))
}

//GetJobs 获取jobs
func GetJobs() (jobs map[string]*Job, err error) {
	if conf.Config.Cmd == "" {
		return nil, fmt.Errorf("job save path can not be empty")
	}
	resp, err := store.DefalutClient.Get(conf.Config.Cmd, client.WithPrefix())
	if err != nil {
		return
	}
	count := len(resp.Kvs)
	jobs = make(map[string]*Job, count)
	if count == 0 {
		return
	}
	for _, j := range resp.Kvs {
		job := new(Job)
		if e := json.Unmarshal(j.Value, job); e != nil {
			logrus.Warnf("job[%s] umarshal err: %s", string(j.Key), e.Error())
			continue
		}

		if err := job.Valid(); err != nil {
			logrus.Warnf("job[%s] is invalid: %s", string(j.Key), err.Error())
			continue
		}
		job.alone()
		jobs[job.ID] = job
	}
	return
}

//WatchJobs watch jobs
func WatchJobs() client.WatchChan {
	return store.DefalutClient.Watch(conf.Config.Cmd, client.WithPrefix())
}

//GetJobFromKv Create job from etcd value
func GetJobFromKv(kv *mvccpb.KeyValue) (job *Job, err error) {
	job = new(Job)
	if err = json.Unmarshal(kv.Value, job); err != nil {
		err = fmt.Errorf("job[%s] umarshal err: %s", string(kv.Key), err.Error())
		return
	}

	err = job.Valid()
	job.alone()
	return
}

func (j *Job) alone() {
	if j.Kind == KindAlone {
		j.Parallels = 1
	}
}

func (j *Job) splitCmd() {
	j.cmd = strings.Split(j.Command, " ")
}

func (j *Job) String() string {
	data, err := json.Marshal(j)
	if err != nil {
		return err.Error()
	}
	return string(data)
}

//CountRunning 获取结点正在执行任务的数量
func (j *Job) CountRunning() (int64, error) {
	resp, err := store.DefalutClient.Get(conf.Config.Proc+j.runOn+"/"+j.Group+"/"+j.ID, client.WithPrefix(), client.WithCountOnly())
	if err != nil {
		return 0, err
	}
	return resp.Count, nil
}

//getErrorLog 从错误输出获取日志
func getErrorLog(job *Job, read io.ReadCloser) {
	if job.EventID != "" {
		logger := event.GetManager().GetLogger(job.EventID)
		defer event.GetManager().ReleaseLogger(logger)
		defer read.Close()
		for {
			r := bufio.NewReader(read)
			line, _, err := r.ReadLine()
			if err != nil {
				return
			}
			lineStr := string(line)
			logger.Debug(lineStr, map[string]string{"jobId": job.ID, "status": "3"})
		}
	}
}

// Run 执行任务
func (j *Job) Run(nid string) bool {
	var (
		cmd         *exec.Cmd
		proc        *Process
		sysProcAttr *syscall.SysProcAttr
	)
	start := time.Now()
	cmd = exec.Command(j.cmd[0], j.cmd[1:]...)
	cmd.SysProcAttr = sysProcAttr
	stderr, _ := cmd.StderrPipe()
	var b bytes.Buffer
	cmd.Stdout = &b
	go getErrorLog(j, stderr)

	if err := cmd.Start(); err != nil {
		logrus.Warnf("job exec failed,details :%s", err.Error())
		j.Fail(start, fmt.Sprintf("%s\n%s", b.String(), err.Error()))
		return false
	}
	proc = &Process{
		ID:     strconv.Itoa(cmd.Process.Pid),
		JobID:  j.ID,
		NodeID: j.runOn,
		Time:   start,
	}
	proc.Start()
	defer proc.Stop()

	if err := cmd.Wait(); err != nil {
		j.Fail(start, fmt.Sprintf("%s\n%s", b.String(), err.Error()))
		return false
	}
	j.Success(start, b.String())

	return true
}

//RunWithRecovery 执行任务，并捕获异常
func (j *Job) RunWithRecovery() {
	defer func() {
		if r := recover(); r != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			logrus.Warnf("panic running job: %v\n%s", r, buf)
		}
	}()
	j.Run(j.runOn)
}

//RunBuildInWithRecovery run build
//should delete
func (j *Job) RunBuildInWithRecovery(nid string) {
	defer func() {
		if r := recover(); r != nil {
			const size = 64 << 10
			buf := make([]byte, size)
			buf = buf[:runtime.Stack(buf, false)]
			logrus.Warnf("panic running job: %v\n%s", r, buf)
		}
	}()
	j.Run(nid)
}

//GetIDFromKey 从 etcd 的 key 中取 id
func GetIDFromKey(key string) string {
	index := strings.LastIndex(key, "/")
	if index < 0 {
		return ""
	}
	if strings.Contains(key, "-") { //build in任务，为了给不同node做一个区分
		return strings.Split(key[index+1:], "-")[0]
	}

	return key[index+1:]
}

//CreateJobKey JobKey
func CreateJobKey(id string) string {
	return conf.Config.Cmd + "/" + id
}

//Key Key
func (j *Job) Key() string {
	return CreateJobKey(j.ID)
}

//Check
func (j *Job) Check() error {
	j.ID = strings.TrimSpace(j.ID)
	if !store.IsValidAsKeyPath(j.ID) {
		return utils.ErrIllegalJobId
	}

	j.Name = strings.TrimSpace(j.Name)
	if len(j.Name) == 0 {
		return utils.ErrEmptyJobName
	}

	j.Group = strings.TrimSpace(j.Group)
	if len(j.Group) == 0 {
		j.Group = DefaultJobGroup
	}

	if !store.IsValidAsKeyPath(j.Group) {
		return utils.ErrIllegalJobGroupName
	}

	j.User = strings.TrimSpace(j.User)

	for i := range j.Rules {
		id := strings.TrimSpace(j.Rules[i].ID)
		if id == "" || strings.HasPrefix(id, "NEW") {
			j.Rules[i].ID = uuid.NewV4().String()
		}
	}

	// 不修改 Command 的内容，简单判断是否为空
	if len(strings.TrimSpace(j.Command)) == 0 {
		return utils.ErrEmptyJobCommand
	}

	return j.Valid()
}

//Success 记录执行结果
func (j *Job) Success(t time.Time, out string) {
	CreateExecutionRecord(j, t, out, true)
}

func (j *Job) Fail(t time.Time, msg string) {
	j.Notify(t, msg)
	CreateExecutionRecord(j, t, msg, false)
}

func (j *Job) Notify(t time.Time, msg string) {
}

func (j *Job) Avg(t, et time.Time) {
	execTime := int64(et.Sub(t) / time.Millisecond)
	if j.AvgTime == 0 {
		j.AvgTime = execTime
		return
	}
	j.AvgTime = (j.AvgTime + execTime) / 2
}

//Cmds 根据执行策略 创建 cmd
func (j *Job) Cmds(node *model.HostNode) (cmds map[string]*Cmd) {
	cmds = make(map[string]*Cmd)
	if j.Pause {
		return
	}
	for _, r := range j.Rules {
		if r.included(node) {
			cmd := &Cmd{
				Job:     j,
				JobRule: r,
			}
			cmds[cmd.GetID()] = cmd
		}
	}

	return
}

//IsRunOn  是否在本节点执行
//只要有一个rule满足条件即可
func (j Job) IsRunOn(node *model.HostNode) bool {
	for _, r := range j.Rules {
		if r.included(node) {
			return true
		}
	}
	return false
}

//Valid 安全选项验证
func (j *Job) Valid() error {
	if len(j.cmd) == 0 {
		j.splitCmd()
	}

	if err := j.ValidRules(); err != nil {
		return err
	}
	return nil
}

//ResolveShell ResolveShell
func (j *Job) ResolveShell() error {
	logrus.Infof("resolving shell,job.cmd is %v", j.cmd)
	if len(j.cmd) == 0 {
		j.genReal()
	}
	if err := j.ValidRules(); err != nil {
		return err
	}
	return nil

}

func (j *Job) genReal() {
	if strings.Contains(j.ID, "online") {
		cmds := strings.Split(j.Command, ";#!")
		base := cmds[0]
		shel := cmds[1]
		shel = ";#!" + shel
		baseArgs := strings.Split(base, " ")
		needToSplit := baseArgs[2]
		args := strings.Split(needToSplit, "_*")

		finalArgs := ""
		for _, v := range args {
			finalArgs = finalArgs + " " + v
		}
		logrus.Infof("job's run args is %s", finalArgs)
		j.cmd = []string{baseArgs[0], baseArgs[1], finalArgs + shel}
	} else {
		cmds := strings.Split(j.Command, " ")
		finalArgs := ""
		args := strings.Split(cmds[2], "_*")
		for _, v := range args {
			finalArgs = finalArgs + " " + v
		}
		j.cmd = []string{cmds[0], cmds[1], finalArgs}
	}

}

//ValidRules ValidRules
func (j *Job) ValidRules() error {
	for _, r := range j.Rules {
		if err := r.Valid(); err != nil {
			return err
		}
	}
	return nil
}

//ShortName ShortName
func (j *Job) ShortName() string {
	if len(j.Name) <= 10 {
		return j.Name
	}

	names := []rune(j.Name)
	if len(names) <= 10 {
		return j.Name
	}

	return string(names[:10]) + "..."
}
