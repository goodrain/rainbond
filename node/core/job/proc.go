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
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	conf "github.com/goodrain/rainbond/cmd/node/option"

	"github.com/Sirupsen/logrus"
)

// 当前执行中的任务信息
// key: /cronsun/proc/node/group/jobId/pid
// value: 开始执行时间
// key 会自动过期，防止进程意外退出后没有清除相关 key，过期时间可配置
type Process struct {
	ID     string    `json:"id"` // pid
	JobID  string    `json:"jobId"`
	Group  string    `json:"group"`
	NodeID string    `json:"nodeId"`
	Time   time.Time `json:"time"` // 开始执行时间

	running int32
	hasPut  int32
	wg      sync.WaitGroup
	done    chan struct{}
}

func GetProcFromKey(key string) (proc *Process, err error) {
	ss := strings.Split(key, "/")
	var sslen = len(ss)
	if sslen < 5 {
		err = fmt.Errorf("invalid proc key [%s]", key)
		return
	}

	proc = &Process{
		ID:     ss[sslen-1],
		JobID:  ss[sslen-2],
		Group:  ss[sslen-3],
		NodeID: ss[sslen-4],
	}
	return
}

func (p *Process) Key() string {
	return conf.Config.Proc + p.NodeID + "/" + p.Group + "/" + p.JobID + "/" + p.ID
}

func (p *Process) Val() string {
	return p.Time.Format(time.RFC3339)
}

// put 出错也进行 del 操作
// 有可能某种原因，put 命令已经发送到 etcd server
// 目前已知的 deadline 会出现此情况
func (p *Process) put() (err error) {
	// if atomic.LoadInt32(&p.running) != 1 {
	// 	return
	// }

	// if !atomic.CompareAndSwapInt32(&p.hasPut, 0, 1) {
	// 	return
	// }

	// id := lID.get()
	// if id < 0 {
	// 	if _, err = store.DefalutClient.Put(p.Key(), p.Val()); err != nil {
	// 		return
	// 	}
	// }

	// _, err = store.DefalutClient.Put(p.Key(), p.Val(), client.WithLease(id))
	// return
	return
}

func (p *Process) del() error {
	// if atomic.LoadInt32(&p.hasPut) != 1 {
	// 	return nil
	// }

	// _, err := store.DefalutClient.Delete(p.Key())
	// return err
	return nil
}

func (p *Process) Start() {
	if p == nil {
		return
	}
	if !atomic.CompareAndSwapInt32(&p.running, 0, 1) {
		return
	}
	if conf.Config.ProcReq == 0 {
		if err := p.put(); err != nil {
			logrus.Warnf("proc put[%s] err: %s", p.Key(), err.Error())
		}
		return
	}
	p.done = make(chan struct{})
	p.wg.Add(1)
	go func() {
		select {
		case <-p.done:
		case <-time.After(time.Duration(conf.Config.ProcReq) * time.Second):
			if err := p.put(); err != nil {
				logrus.Warnf("proc put[%s] err: %s", p.Key(), err.Error())
			}
		}
		p.wg.Done()
	}()
}

func (p *Process) Stop() {
	if p == nil {
		return
	}

	if !atomic.CompareAndSwapInt32(&p.running, 1, 0) {
		return
	}

	if p.done != nil {
		close(p.done)
	}
	p.wg.Wait()

	if err := p.del(); err != nil {
		logrus.Warnf("proc del[%s] err: %s", p.Key(), err.Error())
	}
}
