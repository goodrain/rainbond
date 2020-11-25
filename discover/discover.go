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

package discover

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goodrain/rainbond/util/watch"

	"github.com/goodrain/rainbond/discover/config"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/mvcc/mvccpb"
	etcdutil "github.com/goodrain/rainbond/util/etcd"
	"github.com/sirupsen/logrus"
)

//CallbackUpdate 每次返还变化
type CallbackUpdate interface {
	//TODO:
	//weight自动发现更改实现暂时不 Ready
	UpdateEndpoints(operation config.Operation, endpoints ...*config.Endpoint)
	//when watch occurred error,will exec this method
	Error(error)
}

//Callback 每次返回全部节点
type Callback interface {
	UpdateEndpoints(endpoints ...*config.Endpoint)
	//when watch occurred error,will exec this method
	Error(error)
}

//Discover 后端服务自动发现
type Discover interface {
	AddProject(name string, callback Callback)
	AddUpdateProject(name string, callback CallbackUpdate)
	Stop()
}

//GetDiscover 获取服务发现管理器
func GetDiscover(opt config.DiscoverConfig) (Discover, error) {
	if opt.Ctx == nil {
		opt.Ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(opt.Ctx)
	client, err := etcdutil.NewClient(ctx, opt.EtcdClientArgs)
	if err != nil {
		cancel()
		return nil, err
	}
	watcher := watch.New(client, "")
	etcdD := &etcdDiscover{
		projects: make(map[string]CallbackUpdate),
		ctx:      ctx,
		cancel:   cancel,
		watcher:  watcher,
		prefix:   "/traefik",
	}
	return etcdD, nil
}

type etcdDiscover struct {
	projects map[string]CallbackUpdate
	lock     sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	watcher  watch.Watch
	prefix   string
}
type defaultCallBackUpdate struct {
	endpoints map[string]*config.Endpoint
	callback  Callback
	lock      sync.Mutex
}

func (d *defaultCallBackUpdate) UpdateEndpoints(operation config.Operation, endpoints ...*config.Endpoint) {
	d.lock.Lock()
	defer d.lock.Unlock()
	switch operation {
	case config.ADD:
		for _, e := range endpoints {
			if old, ok := d.endpoints[e.Name]; !ok {
				d.endpoints[e.Name] = e
			} else {
				if e.Mode == 0 {
					old.URL = e.URL
				}
				if e.Mode == 1 {
					old.Weight = e.Weight
				}
				if e.Mode == 2 {
					old.URL = e.URL
					old.Weight = e.Weight
				}
			}
		}
	case config.SYNC:
		for _, e := range endpoints {
			if old, ok := d.endpoints[e.Name]; !ok {
				d.endpoints[e.Name] = e
			} else {
				if e.Mode == 0 {
					old.URL = e.URL
				}
				if e.Mode == 1 {
					old.Weight = e.Weight
				}
				if e.Mode == 2 {
					old.URL = e.URL
					old.Weight = e.Weight
				}
			}
		}
	case config.DELETE:
		for _, e := range endpoints {
			if e.Mode == 0 {
				if old, ok := d.endpoints[e.Name]; ok {
					old.URL = ""
				}
			}
			if e.Mode == 1 {
				if old, ok := d.endpoints[e.Name]; ok {
					old.Weight = 0
				}
			}
			if e.Mode == 2 {
				if _, ok := d.endpoints[e.Name]; ok {
					delete(d.endpoints, e.Name)
				}
			}
		}
	case config.UPDATE:
		for _, e := range endpoints {
			if e.Mode == 0 {
				if old, ok := d.endpoints[e.Name]; ok {
					old.URL = e.URL
				}
			}
			if e.Mode == 1 {
				if old, ok := d.endpoints[e.Name]; ok {
					old.Weight = e.Weight
				}
			}
			if e.Mode == 2 {
				if old, ok := d.endpoints[e.Name]; ok {
					old.URL = e.URL
					old.Weight = e.Weight
				}
			}
		}
	}
	var re []*config.Endpoint
	for _, v := range d.endpoints {
		if v.URL != "" {
			re = append(re, v)
		}
	}
	d.callback.UpdateEndpoints(re...)
}

func (d *defaultCallBackUpdate) Error(err error) {
	d.callback.Error(err)
}

func (e *etcdDiscover) AddProject(name string, callback Callback) {
	e.lock.Lock()
	defer e.lock.Unlock()
	if _, ok := e.projects[name]; !ok {
		cal := &defaultCallBackUpdate{
			callback:  callback,
			endpoints: make(map[string]*config.Endpoint),
		}
		e.projects[name] = cal
		go e.discover(name, cal)
	}
}

func (e *etcdDiscover) AddUpdateProject(name string, callback CallbackUpdate) {
	e.lock.Lock()
	defer e.lock.Unlock()
	if _, ok := e.projects[name]; !ok {
		e.projects[name] = callback
		go e.discover(name, callback)
	}
}

func (e *etcdDiscover) Stop() {
	e.cancel()
}

func (e *etcdDiscover) removeProject(name string) {
	e.lock.Lock()
	defer e.lock.Unlock()
	if _, ok := e.projects[name]; ok {
		delete(e.projects, name)
	}
}

func (e *etcdDiscover) discover(name string, callback CallbackUpdate) {
	watchChan, err := e.watcher.WatchList(e.ctx, fmt.Sprintf("%s/backends/%s/servers", e.prefix, name), "")
	if err != nil {
		callback.Error(err)
		e.removeProject(name)
		return
	}
	defer watchChan.Stop()
	for event := range watchChan.ResultChan() {
		if event.Source == nil {
			continue
		}
		var end *config.Endpoint
		if strings.HasSuffix(event.GetKey(), "/url") { //服务地址变化
			kstep := strings.Split(event.GetKey(), "/")
			if len(kstep) > 2 {
				serverName := kstep[len(kstep)-2]
				serverURL := event.GetValueString()
				end = &config.Endpoint{Name: serverName, URL: serverURL, Mode: 0}
			}
		}
		if strings.HasSuffix(event.GetKey(), "/weight") { //获取服务地址
			kstep := strings.Split(event.GetKey(), "/")
			if len(kstep) > 2 {
				serverName := kstep[len(kstep)-2]
				serverWeight := event.GetValueString()
				weight, _ := strconv.Atoi(serverWeight)
				end = &config.Endpoint{Name: serverName, Weight: weight, Mode: 1}
			}
		}
		switch event.Type {
		case watch.Added:
			if end != nil {
				callback.UpdateEndpoints(config.ADD, end)
			}
		case watch.Modified:
			if end != nil {
				callback.UpdateEndpoints(config.UPDATE, end)
			}
		case watch.Deleted:
			if end != nil {
				callback.UpdateEndpoints(config.DELETE, end)
			}
		case watch.Error:
			callback.Error(event.Error)
			logrus.Errorf("monitor discover get watch error: %s, remove this watch target first, and then sleep 10 sec, we will re-watch it", event.Error.Error())
			e.removeProject(name)
			time.Sleep(10 * time.Second)
			e.AddUpdateProject(name, callback)
			return
		}
	}
}

func makeEndpointForKvs(kvs []*mvccpb.KeyValue) (res []*config.Endpoint) {
	var ends = make(map[string]*config.Endpoint)
	for _, kv := range kvs {
		if strings.HasSuffix(string(kv.Key), "/url") { //获取服务地址
			kstep := strings.Split(string(kv.Key), "/")
			if len(kstep) > 2 {
				serverName := kstep[len(kstep)-2]
				serverURL := string(kv.Value)
				if en, ok := ends[serverName]; ok {
					en.URL = serverURL
				} else {
					ends[serverName] = &config.Endpoint{Name: serverName, URL: serverURL}
				}
			}
		}
		if strings.HasSuffix(string(kv.Key), "/weight") { //获取服务权重
			kstep := strings.Split(string(kv.Key), "/")
			if len(kstep) > 2 {
				serverName := kstep[len(kstep)-2]
				serverWeight := string(kv.Value)
				if en, ok := ends[serverName]; ok {
					var err error
					en.Weight, err = strconv.Atoi(serverWeight)
					if err != nil {
						logrus.Error("get server weight error.", err.Error())
					}
				} else {
					weight, err := strconv.Atoi(serverWeight)
					if err != nil {
						logrus.Error("get server weight error.", err.Error())
					}
					ends[serverName] = &config.Endpoint{Name: serverName, Weight: weight}
				}
			}
		}
	}
	for _, v := range ends {
		res = append(res, v)
	}
	return
}
