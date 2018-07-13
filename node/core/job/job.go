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

package job

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	client "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	conf "github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/node/core/store"
	"github.com/goodrain/rainbond/node/utils"
	"github.com/goodrain/rainbond/util"
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

//Event JobEvent
type Event struct {
	EventType string `json:"event_type"`
	Job       Job    `json:"job"`
}

//Job 需要执行的任务
type Job struct {
	ID      string   `json:"id"`
	TaskID  string   `json:"taskID"`
	EventID string   `json:"event_id"`
	NodeID  string   `json:"node_id"`
	Hash    string   `json:"hash"`
	Name    string   `json:"name"`
	Command string   `json:"cmd"`
	Stdin   string   `json:"stdin"`
	Envs    []string `json:"envs"`
	User    string   `json:"user"`
	//rules 为nil 即当前任务是一次任务
	Rules   *Rule `json:"rule"`
	Pause   bool  `json:"pause"`   // 可手工控制的状态
	Timeout int64 `json:"timeout"` // 任务执行时间超时设置，大于 0 时有效
	// 执行任务失败重试次数
	// 默认为 0，不重试
	Retry int `json:"retry"`
	// 执行任务失败重试时间间隔
	// 单位秒，如果不大于 0 则马上重试
	Interval int `json:"interval"`
	// 任务类型
	// 0: 单次任务
	// 1: 循环任务
	Kind int `json:"kind"`
	// 平均执行时间，单位 ms
	AvgTime int64 `json:"avg_time"`
	// 用于存储分隔后的任务
	cmd []string
	// 控制同时执行任务数
	Count     *int64 `json:"-"`
	Scheduler *Scheduler
	RunStatus *RunStatus
}

//Scheduler 调度信息
type Scheduler struct {
	NodeID          string    `json:"node_id"`
	SchedulerTime   time.Time `json:"scheduler_time"`
	CanRun          bool      `json:"can_run"`
	Message         string    `json:"message"`
	SchedulerStatus string    `json:"scheduler_status"`
}

//RunStatus job run status
type RunStatus struct {
	Status    string    `json:"status"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	RecordID  string    `json:"record_id"`
}

//Rule 任务规则
type Rule struct {
	ID       string            `json:"id"`
	Mode     RuleMode          `json:"mode"` //once,
	Timer    string            `json:"timer"`
	Labels   map[string]string `json:"labels"`
	Schedule cron.Schedule     `json:"-"`
}

//RuleMode RuleMode
type RuleMode string

//OnlyOnce 只能一次
var OnlyOnce RuleMode = "onlyonce"

//ManyOnce 多次运行
var ManyOnce RuleMode = "manyonce"

//Cycle 循环运行
var Cycle RuleMode = "cycle"

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
	*Rule
}

//GetID GetID
func (c *Cmd) GetID() string {
	return c.Job.ID + c.Rule.ID
}

//Run 执行
func (c *Cmd) Run() {
	if c.Job.Retry <= 0 {
		c.Job.Run("")
		return
	}

	for i := 0; i < c.Job.Retry; i++ {
		if c.Job.Run("") {
			return
		}
		logrus.Warnf("job %s run error ,will retry", c.Job.ID)
		if c.Job.Interval > 0 {
			time.Sleep(time.Duration(c.Job.Interval) * time.Second)
		}
	}
}

//lockTTL
func (c *Cmd) lockTTL() int64 {
	now := time.Now()
	prev := c.Rule.Schedule.Next(now)
	ttl := int64(c.Rule.Schedule.Next(prev).Sub(prev) / time.Second)
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
func (j *Rule) Valid() error {
	// 注意 interface nil 的比较
	if j.Schedule != nil {
		return nil
	}
	if j.Mode != OnlyOnce && j.Mode != ManyOnce && j.Mode != Cycle {
		return fmt.Errorf("job rule mode(%s) can not be support", j.Mode)
	}
	if j.Mode == Cycle && len(j.Timer) <= 0 {
		return fmt.Errorf("job rule mode(%s) timer can not be empty", Cycle)
	}
	if j.Mode == Cycle && len(j.Timer) > 0 {
		sch, err := cron.Parse(j.Timer)
		if err != nil {
			return fmt.Errorf("invalid JobRule[%s], parse err: %s", j.Timer, err.Error())
		}
		j.Schedule = sch
	}
	return nil
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

//GetJobs 获取当前节点jobs
func GetJobs(nodeID string) (jobs map[string]*Job, err error) {
	if conf.Config.JobPath == "" {
		return nil, fmt.Errorf("job save path can not be empty")
	}
	resp, err := store.DefalutClient.Get(conf.Config.JobPath, client.WithPrefix())
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
		if !job.IsRunOn(nodeID) {
			continue
		}
		jobs[job.ID] = job
	}
	return
}

//WatchJobs watch jobs
func WatchJobs() client.WatchChan {
	return store.DefalutClient.Watch(conf.Config.JobPath, client.WithPrefix())
}

//PutJob 添加获取更新job
func PutJob(j *Job) error {
	_, err := store.DefalutClient.Put(conf.Config.JobPath+"/"+j.Hash, j.String())
	if err != nil {
		return err
	}
	return nil
}

//DeleteJob delete job
func DeleteJob(hash string) error {
	_, err := store.DefalutClient.Delete(conf.Config.JobPath + "/" + hash)
	if err != nil {
		return err
	}
	return nil
}

//GetJobFromKv Create job from etcd value
func GetJobFromKv(kv *mvccpb.KeyValue) (job *Job, err error) {
	job = new(Job)
	if err = json.Unmarshal(kv.Value, job); err != nil {
		err = fmt.Errorf("job[%s] umarshal err: %s", string(kv.Key), err.Error())
		return
	}
	err = job.Valid()
	return
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

//Decode Decode
func (j *Job) Decode(data []byte) error {
	if err := json.Unmarshal(data, j); err != nil {
		err = fmt.Errorf("job decode err: %s", err.Error())
		return err
	}
	if err := j.Valid(); err != nil {
		return err
	}
	return nil
}

//CountRunning 获取结点正在执行任务的数量
func (j *Job) CountRunning() (int64, error) {
	resp, err := store.DefalutClient.Get(conf.Config.Proc+j.NodeID+"/"+j.ID, client.WithPrefix(), client.WithCountOnly())
	if err != nil {
		return 0, err
	}
	return resp.Count, nil
}

//handleJobLog 从标准输出获取日志
func handleJobLog(job *Job, read io.ReadCloser) {
	var logpath = "/var/log/event"
	util.CheckAndCreateDir(logpath)
	if job.EventID != "" {
		logger := event.GetManager().GetLogger(job.EventID)
		defer event.GetManager().ReleaseLogger(logger)
		defer read.Close()
		logfile, err := util.OpenOrCreateFile(logpath + "/" + job.EventID + ".log")
		if err != nil {
			logrus.Errorf("open file %s error,%s", logpath+"/"+job.EventID+".log", err.Error())
		}
		if logfile != nil {
			defer logfile.Close()
		}
		r := bufio.NewReader(read)
		for {
			line, _, err := r.ReadLine()
			if err != nil {
				return
			}
			lineStr := string(line)
			logger.Debug(lineStr, map[string]string{"jobId": job.ID, "status": "3"})
			if logfile != nil {
				// 查找文件末尾的偏移量
				n, _ := logfile.Seek(0, os.SEEK_END)
				// 从末尾的偏移量开始写入内容
				_, err = logfile.WriteAt(append(line, []byte("\n")...), n)
			}
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
	//注入环境变量
	cmd.Env = append(os.Environ(), j.Envs...)
	//注入标准输入
	if j.Stdin != "" {
		cmd.Stdin = bytes.NewBuffer([]byte(j.Stdin))
	}
	//从执行任务的标准输出获取日志，错误输出获取最终结果
	stdout, _ := cmd.StdoutPipe()
	var b bytes.Buffer
	cmd.Stderr = &b
	go handleJobLog(j, stdout)

	if err := cmd.Start(); err != nil {
		logrus.Warnf("job exec failed,details :%s", err.Error())
		j.Fail(start, err.Error()+"\n"+b.String())
		return false
	}
	proc = &Process{
		ID:     strconv.Itoa(cmd.Process.Pid),
		JobID:  j.ID,
		NodeID: j.NodeID,
		Time:   start,
	}
	proc.Start()
	defer proc.Stop()

	if err := cmd.Wait(); err != nil {
		j.Fail(start, err.Error()+"\n"+b.String())
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
	j.Run(j.NodeID)
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
	return conf.Config.JobPath + "/" + id
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

	j.User = strings.TrimSpace(j.User)

	if j.Rules != nil {
		id := strings.TrimSpace(j.Rules.ID)
		if id == "" || strings.HasPrefix(id, "NEW") {
			j.Rules.ID = uuid.NewV4().String()
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
func (j *Job) Cmds() (cmds map[string]*Cmd) {
	cmds = make(map[string]*Cmd)
	if j.Pause {
		return
	}
	if j.Rules != nil {
		cmd := &Cmd{
			Job:  j,
			Rule: j.Rules,
		}
		cmds[cmd.GetID()] = cmd
	}
	return
}

//IsRunOn  是否在本节点执行
func (j Job) IsRunOn(nodeID string) bool {
	if j.Scheduler == nil {
		return false
	}
	if j.Scheduler.NodeID != nodeID {
		return false
	}
	if !j.Scheduler.CanRun {
		return false
	}
	//已有执行状态
	if j.RunStatus != nil {
		return false
	}
	return true
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
	if j.Rules == nil {
		return fmt.Errorf("job rule can not be nil")
	}
	if err := j.Rules.Valid(); err != nil {
		return err
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
