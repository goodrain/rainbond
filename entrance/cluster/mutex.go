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

package cluster

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/client"
)

const (
	defaultTTL   = 60
	defaultTry   = 3
	deleteAction = "delete"
	expireAction = "expire"
)

// A Mutex is a mutual exclusion lock which is distributed across a cluster.
type Mutex struct {
	key    string
	id     string // The identity of the caller
	client client.Client
	kapi   client.KeysAPI
	ctx    context.Context
	ttl    time.Duration
	mutex  *sync.Mutex
}

// New creates a Mutex with the given key which must be the same
// across the cluster nodes.
func New(key string, ttl int, c client.Client) *Mutex {

	hostname, err := os.Hostname()
	if err != nil {
		return nil
	}

	if len(key) == 0 {
		return nil
	}

	if key[0] != '/' {
		key = "/" + key
	}

	if ttl < 1 {
		ttl = defaultTTL
	}

	return &Mutex{
		key:    key,
		id:     fmt.Sprintf("%v-%v-%v", hostname, os.Getpid(), time.Now().Format("20060102-15:04:05.999999999")),
		client: c,
		kapi:   client.NewKeysAPI(c),
		ctx:    context.TODO(),
		ttl:    time.Second * time.Duration(ttl),
		mutex:  new(sync.Mutex),
	}
}

// Lock locks m.
// If the lock is already in use, the calling goroutine
// blocks until the mutex is available.
func (m *Mutex) Lock() (err error) {
	m.mutex.Lock()
	for try := 1; try <= defaultTry; try++ {
		if m.lock() == nil {
			return nil
		}
		if try < defaultTry {
			logrus.Debugf("Try to lock node %v again", m.key, err)
		}
	}
	return err
}

func (m *Mutex) lock() (err error) {
	setOptions := &client.SetOptions{
		PrevExist: client.PrevNoExist,
		TTL:       m.ttl,
	}
	resp, err := m.kapi.Set(m.ctx, m.key, m.id, setOptions)
	if err == nil {
		return nil
	}
	e, ok := err.(client.Error)
	if !ok {
		return err
	}
	if e.Code != client.ErrorCodeNodeExist {
		return err
	}
	// Get the already node's value.
	resp, err = m.kapi.Get(m.ctx, m.key, nil)
	if err != nil {
		return err
	}
	watcherOptions := &client.WatcherOptions{
		AfterIndex: resp.Index,
		Recursive:  false,
	}
	watcher := m.kapi.Watcher(m.key, watcherOptions)
	for {
		resp, err = watcher.Next(m.ctx)
		if err != nil {
			return err
		}
		if resp.Action == deleteAction || resp.Action == expireAction {
			return nil
		}
	}

}

// Unlock unlocks m.
// It is a run-time error if m is not locked on entry to Unlock.
//
// A locked Mutex is not associated with a particular goroutine.
// It is allowed for one goroutine to lock a Mutex and then
// arrange for another goroutine to unlock it.
func (m *Mutex) Unlock() (err error) {
	defer m.mutex.Unlock()
	for i := 1; i <= defaultTry; i++ {
		_, err = m.kapi.Delete(m.ctx, m.key, nil)
		if err == nil {
			return nil
		}
		e, ok := err.(client.Error)
		if ok && e.Code == client.ErrorCodeKeyNotFound {
			return nil
		}
	}
	return err
}
