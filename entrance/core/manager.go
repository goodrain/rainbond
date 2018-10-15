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

package core

import (
	"os"
	"strconv"

	"github.com/goodrain/rainbond/cmd/entrance/option"
	"github.com/goodrain/rainbond/entrance/cluster"
	"github.com/goodrain/rainbond/entrance/core/event"
	"github.com/goodrain/rainbond/entrance/core/object"
	"github.com/goodrain/rainbond/entrance/plugin"
	"github.com/goodrain/rainbond/entrance/store"

	"github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"

	"time"

	"fmt"

	"golang.org/x/net/context"
)

//Event source change event
type Event struct {
	Method EventMethod
	Source object.Object
}

//EventMethod event method
type EventMethod string

//ADDEventMethod add method
const ADDEventMethod EventMethod = "ADD"

//UPDATEEventMethod add method
const UPDATEEventMethod EventMethod = "UPDATE"

//DELETEEventMethod add method
const DELETEEventMethod EventMethod = "DELETE"

//Manager core manager
type Manager interface {
	EventChan() chan<- Event
	Start() error
	Stop() error
	Scrape(ch chan<- prometheus.Metric, namespace, exporter string) error
}

var defaultHTTPSVS = &object.VirtualServiceObject{
	Index:           10000001,
	Name:            "HTTPS.VS",
	Enabled:         true,
	Protocol:        "http",
	Port:            getHTTPSListenPort(),
	DefaultPoolName: "discard",
	Rules:           []string{"custom", "httpsproxy"},
	Note:            "system https vs",
}

func getHTTPSListenPort() int32 {
	if os.Getenv("DEFAULT_HTTPS_PORT") != "" {
		if port, err := strconv.Atoi(os.Getenv("DEFAULT_HTTPS_PORT")); err == nil {
			return int32(port)
		}
	}
	return 10443
}

//NewManager create core manager
func NewManager(server *option.ACPLBServer, pluginManager *plugin.Manager, storeManager *store.Manager, cluster *cluster.Manager) Manager {
	ctx, c := context.WithCancel(context.Background())
	eventManager := event.NewManager(server.Config)
	m := &manager{
		ctx:           ctx,
		cancel:        c,
		server:        server,
		eventChan:     make(chan Event, 20),
		pluginManager: pluginManager,
		storeManager:  storeManager,
		cluster:       cluster,
		eventManager:  eventManager,
	}
	logrus.Info("core manager create.")
	return m
}

type manager struct {
	eventChan       chan Event
	ctx             context.Context
	cancel          context.CancelFunc
	server          *option.ACPLBServer
	pluginManager   *plugin.Manager
	storeManager    *store.Manager
	cluster         *cluster.Manager
	pluginErrorSize float64
	eventManager    *event.Manager
}

func (m *manager) EventChan() chan<- Event {
	return m.eventChan
}

func (m *manager) Start() error {
	go m.handleEvent()
	return nil
}

func (m *manager) Stop() error {

	m.cancel()
	return nil
}

//handleEvent 单线程顺序处理
//目前唯一性保证处在资源级。
//同一个应用的多个资源在不同实例处理时可能存在顺序错误
func (m *manager) handleEvent() {
	logrus.Info("core manager start handle event...")
	for {
		select {
		case <-m.ctx.Done():
			return
		case event := <-m.eventChan:
			switch event.Method {
			case ADDEventMethod:
				m.add(event.Source)
			case UPDATEEventMethod:
				m.update(event.Source)
			case DELETEEventMethod:
				m.delete(event.Source)
			}
		}
	}
}
func (m *manager) getPlugin(name string, opt map[string]string) (p plugin.Plugin, err error) {
	if name == "" {
		return m.pluginManager.GetDefaultPlugin(m.storeManager)
	}
	p, err = m.pluginManager.GetPlugin(name, opt, m.storeManager)
	if err != nil {
		logrus.Errorf("get %s plugin error.%s will use default plugin", name, err.Error())
		p, err = m.pluginManager.GetDefaultPlugin(m.storeManager)
	}
	return
}

func (m *manager) add(source object.Object) {
	ok, err := m.storeManager.AddSource(source)
	if err != nil {
		logrus.Errorf("Add %s to store error.%s", source.GetName(), err.Error())
		//TODO: 判断是否已经存储，获取到操作权后失败。如果是，怎么协调集群其他点进行重试。
		return
	}
	if ok { //获取到操作权，进行操作
		logrus.Debugf("Get handle add permissions for %s", source.GetName())
		switch source.(type) {
		case *object.NodeObject:
			node := source.(*object.NodeObject)
			if !node.Ready {
				logrus.Debugf(node.NodeName, " node is not ready,don't update to lb")
				return
			}
		}
		m.handleAdd(source)
	}
}

func (m *manager) delete(source object.Object) {
	switch source.(type) {
	case *object.PoolObject:
		pool := source.(*object.PoolObject)
		plugin, err := m.getPlugin(pool.PluginName, pool.PluginOpts)
		if err != nil {
			logrus.Errorf("get default plugin error.%s pool don't add to lb", err.Error())
			return
		}
		if plugin.GetName() == "zeus" {
			//zeus do not handle pool delete
			return
		}
		nodes, err := m.storeManager.GetNodeByPool(pool.GetName())
		if err != nil {
			logrus.Errorf("get node list by pool name(%s) error.%s,do not delete pool", pool.GetName(), err.Error())
			return
		}
		if len(nodes) != 0 {
			return
		}
	}
	ok, err := m.storeManager.DeleteSource(source)
	if err != nil {
		logrus.Errorf("Update %s to store error.%s", source.GetName(), err.Error())
		//TODO 判断是否已经存储，获取到操作权后失败。如果是，怎么协调集群其他点进行重试。
		return
	}
	if ok { //获取到操作权，进行操作
		switch source.(type) {
		case *object.NodeObject:
			node := source.(*object.NodeObject)
			logrus.Debugf("delete node host:%v", node.Host)
		}
		logrus.Debugf("Get handle delete permissions for %s", source.GetName())
		m.handleDelete(source)
	}
}

func (m *manager) update(source object.Object) {
	isOnline, ok, err := m.storeManager.UpdateSource(source)
	if err != nil {
		logrus.Errorf("Update %s to store error.%s", source.GetName(), err.Error())
		//TODO 判断是否已经存储，获取到操作权后失败。如果是，怎么协调集群其他点进行重试。
		return
	}
	if ok { //获取到操作权，进行操作
		logrus.Debugf("Get handle update permissions for %s, %d", source.GetName(), source.GetIndex())
		switch source.(type) {
		case *object.NodeObject:
			node := source.(*object.NodeObject)
			logrus.Debugf("updateupdate Ready is %v, isOnline is %v, host is %v", node.Ready, isOnline, node.Host)
			if !node.Ready && isOnline {
				//if pool have one node,If this node is not-ready,could not online it.
				//If the only one node is a real failure,The effect is the same for offline and online
				//But if the node is not real failure.It shouldn't be offline.
				nodes, _ := m.storeManager.GetNodeByPool(node.PoolName)
				if len(nodes) > 1 {
					logrus.Info(node.NodeName, " node is not ready and pool have multiple nodes is online, should offline it.")
					err := m.handleDelete(node)
					if err == nil {
						err := m.storeManager.UpdateSourceOnline(node, false)
						if err != nil {
							logrus.Errorf("update a node %s online status is false error.%s", node.NodeName, err.Error())
						}
					}
				}
			} else if node.Ready {
				logrus.Info(node.NodeName, " node is ready, should add it to lb.")
				m.handleUpdate(source)
			} else {
				logrus.Debugf("%s don't need update .ignore it", source.GetName())
			}
		}
	}
}

func (m *manager) handleAdd(source object.Object) {
	switch source.(type) {
	case *object.PoolObject:
		//操作pool信息，加分布式锁，集群不能同时操作同一个pool
		pool := source.(*object.PoolObject)
		plugin, err := m.getPlugin(pool.PluginName, pool.PluginOpts)
		if err != nil {
			logrus.Errorf("get default plugin error.%s pool don't add to lb", err.Error())
		}
		lock := m.storeManager.CreateMutex("/lock/" + pool.Name)
		err = lock.Lock()
		if err != nil {
			logrus.Error("handle pool should lock.but create errror.", err.Error())
			return
		}
		err = m.ExecPool(plugin.AddPool, pool)
		if err != nil {
			logrus.Errorf("add pool %s to lb error.%s", pool.Name, err.Error())
		} else {
			logrus.Infof("add a pool %s", pool.Name)
		}
		lock.Unlock()
	case *object.VirtualServiceObject:
		vs := source.(*object.VirtualServiceObject)
		plugin, err := m.getPlugin(vs.PluginName, vs.PluginOpts)
		if err != nil {
			logrus.Errorf("get default plugin error.%s vs don't add to lb", err.Error())
		}
		err = m.ExecVS(plugin.AddVirtualService, vs)
		if err != nil {
			logrus.Errorf("add vs %s to lb error.%s", vs.Name, err.Error())
		} else {
			logrus.Infof("add a vs %s for pool %s", vs.Name, vs.DefaultPoolName)
			m.eventManager.Info(source.GetEventID(), "success", "负载均衡虚拟服务已添加")
		}
	case *object.RuleObject:
		rule := source.(*object.RuleObject)
		plugin, err := m.getPlugin(rule.PluginName, rule.PluginOpts)
		if err != nil {
			logrus.Errorf("get default plugin error.%s rule don't add to lb", err.Error())
		}
		err = m.ExecRule(plugin.AddRule, rule)
		if err != nil {
			logrus.Errorf("add rule %s to lb error.%s", rule.Name, err.Error())
		} else {
			logrus.Infof("add rule about domain %s ", rule.DomainName)
			m.eventManager.Debug(source.GetEventID(), "success", fmt.Sprintf("负载均衡域名（%s）规则已添加", rule.DomainName))
			if rule.HTTPS {
				logrus.Infof("start update default ssl vs from lb due to the add https rule")
				plugin.UpdateVirtualService(defaultHTTPSVS)
			}
		}
	case *object.NodeObject:
		node := source.(*object.NodeObject)
		plugin, err := m.getPlugin(node.PluginName, node.PluginOpts)
		if err != nil {
			logrus.Errorf("get default plugin error.%s node don't add to lb", err.Error())
		}
		lock := m.storeManager.CreateMutex("/lock/" + node.PoolName)
		err = lock.Lock()
		if err != nil {
			logrus.Error("handle pool should lock.but create errror.", err.Error())
			return
		}
		err = m.ExecNode(plugin.AddNode, node)
		if err != nil {
			logrus.Errorf("add rule %s to lb error.%s", node.NodeName, err.Error())
		} else {
			logrus.Infof("add a node %s (%s:%d) to pool %s", node.NodeName, node.Host, node.Port, node.PoolName)
			err := m.storeManager.UpdateSourceOnline(node, true)
			if err != nil {
				logrus.Errorf("update a node %s online status is true error.%v", node.NodeName, err.Error())
			}
			m.eventManager.Info(source.GetEventID(), "success", fmt.Sprintf("负载均衡节点（%s:%d）已添加", node.Host, node.Port))
		}
		lock.Unlock()
	case *object.DomainObject:
		domain := source.(*object.DomainObject)
		plugin, err := m.getPlugin(domain.PluginName, domain.PluginOpts)
		if err != nil {
			logrus.Errorf("get default plugin error.%s domain don't add to lb", err.Error())
		}
		err = m.ExecDomain(plugin.AddDomain, domain)
		if err != nil {
			logrus.Errorf("add domain %s to lb error.%s", domain.Name, err.Error())
		} else {
			logrus.Infof("add a domain %s", domain.Name)
			m.eventManager.Debug(source.GetEventID(), "success", fmt.Sprintf("负载均衡域名（%s）已添加", domain.Domain))
		}
	case *object.Certificate:
		ca := source.(*object.Certificate)
		plugin, err := m.getPlugin(ca.PluginName, ca.PluginOpts)
		if err != nil {
			logrus.Errorf("get default plugin error.%s domain don't add to lb", err.Error())
		}
		err = m.ExecCertificate(plugin.AddCertificate, ca)
		if err != nil {
			logrus.Errorf("update domain %s to lb error.%s", ca.Name, err.Error())
		}
	}
}

func (m *manager) handleUpdate(source object.Object) {
	switch source.(type) {
	case *object.PoolObject:
		pool := source.(*object.PoolObject)
		plugin, err := m.getPlugin(pool.PluginName, pool.PluginOpts)
		if err != nil {
			logrus.Errorf("get default plugin error.%s pool don't add to lb", err.Error())
		}
		lock := m.storeManager.CreateMutex("/lock/" + pool.Name)
		err = lock.Lock()
		if err != nil {
			logrus.Error("handle update pool should lock.but create errror.", err.Error())
			return
		}
		err = m.ExecPool(plugin.UpdatePool, pool)
		if err != nil {
			logrus.Errorf("update pool %s to lb error.%s", pool.Name, err.Error())
		}
		// TODO:
		// IF error is not nil, will save state to etcd and other instance can continue to perform
		lock.Unlock()
	case *object.VirtualServiceObject:
		vs := source.(*object.VirtualServiceObject)
		plugin, err := m.getPlugin(vs.PluginName, vs.PluginOpts)
		if err != nil {
			logrus.Errorf("update default plugin error.%s vs don't add to lb", err.Error())
		}
		err = m.ExecVS(plugin.UpdateVirtualService, vs)
		if err != nil {
			logrus.Errorf("add vs %s to lb error.%s", vs.Name, err.Error())
		}
	case *object.RuleObject:
		rule := source.(*object.RuleObject)
		plugin, err := m.getPlugin(rule.PluginName, rule.PluginOpts)
		if err != nil {
			logrus.Errorf("get default plugin error.%s rule don't add to lb", err.Error())
		}
		err = m.ExecRule(plugin.UpdateRule, rule)
		if err != nil {
			logrus.Errorf("update rule %s to lb error.%s", rule.Name, err.Error())
			return
		}
		logrus.Infof("update rule %s from lb success", rule.Name)
		if rule.HTTPS {
			logrus.Infof("start update default ssl vs from lb due to the update https rule")
			plugin.UpdateVirtualService(defaultHTTPSVS)
		}

	case *object.NodeObject:
		node := source.(*object.NodeObject)
		plugin, err := m.getPlugin(node.PluginName, node.PluginOpts)
		if err != nil {
			logrus.Errorf("get default plugin error.%s node don't add to lb", err.Error())
		}
		lock := m.storeManager.CreateMutex("/lock/" + node.PoolName)
		err = lock.Lock()
		if err != nil {
			logrus.Error("handle update node should lock.but create errror.", err.Error())
			return
		}
		err = m.ExecNode(plugin.UpdateNode, node)
		if err != nil {
			logrus.Errorf("update node %s to lb error.%s", node.NodeName, err.Error())
		} else {
			logrus.Infof("update a node %s(%d) (%s:%d) to pool %s", node.NodeName, node.Index, node.Host, node.Port, node.PoolName)
			err := m.storeManager.UpdateSourceOnline(node, true)
			if err != nil {
				logrus.Errorf("update a node %s online status is true error.%s", node.NodeName, err.Error())
			}
			m.eventManager.Info(source.GetEventID(), "success", fmt.Sprintf("负载均衡节点（%s:%d）已添加", node.Host, node.Port))
		}
		lock.Unlock()
	case *object.DomainObject:
		domain := source.(*object.DomainObject)
		plugin, err := m.getPlugin(domain.PluginName, domain.PluginOpts)
		if err != nil {
			logrus.Errorf("get default plugin error.%s domain don't add to lb", err.Error())
		}
		err = m.ExecDomain(plugin.UpdateDomain, domain)
		if err != nil {
			logrus.Errorf("update domain %s to lb error.%s", domain.Name, err.Error())
		}
	}
}

func (m *manager) handleDelete(source object.Object) error {
	switch source.(type) {
	case *object.PoolObject:
		pool := source.(*object.PoolObject)
		plugin, err := m.getPlugin(pool.PluginName, pool.PluginOpts)
		if err != nil {
			logrus.Errorf("get default plugin error.%s pool don't add to lb", err.Error())
			return err
		}
		err = m.ExecPool(plugin.DeletePool, pool)
		if err != nil {
			logrus.Errorf("delete pool %s from lb error.%s", pool.Name, err.Error())
			return err
		}
	case *object.VirtualServiceObject:
		vs := source.(*object.VirtualServiceObject)
		plugin, err := m.getPlugin(vs.PluginName, vs.PluginOpts)
		if err != nil {
			logrus.Errorf("update default plugin error.%s vs don't add to lb", err.Error())
			return err
		}
		err = m.ExecVS(plugin.DeleteVirtualService, vs)
		if err != nil {
			logrus.Errorf("delete vs %s from lb error.%s", vs.Name, err.Error())
			return err
		}
	case *object.RuleObject:
		rule := source.(*object.RuleObject)
		plugin, err := m.getPlugin(rule.PluginName, rule.PluginOpts)
		if err != nil {
			logrus.Errorf("get default plugin error.%s rule don't add to lb", err.Error())
			return err
		}
		err = m.ExecRule(plugin.DeleteRule, rule)
		if err != nil {
			logrus.Errorf("delete rule %s from lb error.%s", rule.Name, err.Error())
			return err
		}
		logrus.Infof("delete rule %s from lb success", rule.Name)
		if rule.HTTPS {
			logrus.Infof("start update default ssl vs from lb due to the delete https rule")
			plugin.UpdateVirtualService(defaultHTTPSVS)
		}
	case *object.NodeObject:
		node := source.(*object.NodeObject)
		plugin, err := m.getPlugin(node.PluginName, node.PluginOpts)
		if err != nil {
			logrus.Errorf("get default plugin error.%s node don't add to lb", err.Error())
			return err
		}
		err = m.ExecNode(plugin.DeleteNode, node)
		if err != nil {
			logrus.Errorf("delete node %s from lb error.%s", node.NodeName, err.Error())
			return err
		}
		m.eventManager.Info(source.GetEventID(), "success", fmt.Sprintf("负载均衡节点（%s:%d）已下线", node.Host, node.Port))
		logrus.Infof("delete node %s from pool %s", node.NodeName, node.PoolName)
	case *object.DomainObject:
		domain := source.(*object.DomainObject)
		plugin, err := m.getPlugin(domain.PluginName, domain.PluginOpts)
		if err != nil {
			logrus.Errorf("get default plugin error.%s domain don't add to lb", err.Error())
			return err
		}
		err = m.ExecDomain(plugin.DeleteDomain, domain)
		if err != nil {
			logrus.Errorf("delete domain %s from lb error.%s", domain.Name, err.Error())
			return err
		}
	case *object.Certificate:
		ca := source.(*object.Certificate)
		plugin, err := m.getPlugin(ca.PluginName, ca.PluginOpts)
		if err != nil {
			logrus.Errorf("get default plugin error.%s ca don't delete from lb", err.Error())
		}
		err = m.ExecCertificate(plugin.DeleteCertificate, ca)
		if err != nil {
			logrus.Errorf("delete ca %s from lb error.%s", ca.Name, err.Error())
		}
	}
	return nil
}

//Scrape prometheus metric scrape
//TODO:
func (m *manager) Scrape(ch chan<- prometheus.Metric, namespace, exporter string) error {
	scrapeTime := time.Now()
	//step 1: monitor lb plugin status
	pluginDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "default_plugin_up"),
		"the lb plugin status.",
		[]string{"plugin_name"}, nil,
	)
	plugin, err := m.pluginManager.GetDefaultPlugin(m.storeManager)
	if err != nil {
		logrus.Error(err)
		ch <- prometheus.MustNewConstMetric(pluginDesc, prometheus.GaugeValue, 0, plugin.GetName())
	} else {
		if ok := plugin.GetPluginStatus(); ok {
			ch <- prometheus.MustNewConstMetric(pluginDesc, prometheus.GaugeValue, 1, plugin.GetName())
		} else {
			ch <- prometheus.MustNewConstMetric(pluginDesc, prometheus.GaugeValue, 1, plugin.GetName())
		}
	}

	//step 2: monitor core manager exec plugin err size
	errSizeDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "coremanager_pluginerr_size"),
		"the lb plugin status.",
		nil, nil,
	)
	ch <- prometheus.MustNewConstMetric(errSizeDesc, prometheus.CounterValue, m.pluginErrorSize)

	//step last: scrape time
	scrapeDurationDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, exporter, "collector_duration_seconds"),
		"Collector time duration.",
		nil, nil,
	)
	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, time.Since(scrapeTime).Seconds())
	return nil
}

//ExecPool Pool方法执行器
func (m *manager) ExecPool(fun func(...*object.PoolObject) error, pools ...*object.PoolObject) error {
	var err error
	for i := 0; i < 3; i++ {
		err = fun(pools...)
		if err != nil {
			m.pluginErrorSize++
			time.Sleep(time.Millisecond * 10)
		} else {
			break
		}
	}
	return err
}

//ExecNode Node方法执行器
func (m *manager) ExecNode(fun func(...*object.NodeObject) error, pools ...*object.NodeObject) error {
	var err error
	for i := 0; i < 3; i++ {
		err = fun(pools...)
		if err != nil {
			m.pluginErrorSize++
			time.Sleep(time.Millisecond * 10)
		} else {
			break
		}
	}
	return err
}

//ExecVS VS方法执行器
func (m *manager) ExecVS(fun func(...*object.VirtualServiceObject) error, pools ...*object.VirtualServiceObject) error {
	var err error
	for i := 0; i < 3; i++ {
		err = fun(pools...)
		if err != nil {
			m.pluginErrorSize++
			time.Sleep(time.Millisecond * 10)
		} else {
			break
		}
	}
	return err
}

//ExecRule rule方法执行器
func (m *manager) ExecRule(fun func(...*object.RuleObject) error, pools ...*object.RuleObject) error {
	var err error
	for i := 0; i < 3; i++ {
		err = fun(pools...)
		if err != nil {
			logrus.Errorf("exec rule error %s", err)
			m.pluginErrorSize++
		} else {
			break
		}
	}
	return err
}

//ExecDomain domain方法执行器
func (m *manager) ExecDomain(fun func(...*object.DomainObject) error, pools ...*object.DomainObject) error {
	var err error
	for i := 0; i < 3; i++ {
		err = fun(pools...)
		if err != nil {
			m.pluginErrorSize++
			time.Sleep(time.Millisecond * 10)
		} else {
			break
		}
	}
	return err
}

//ExecDomain domain方法执行器
func (m *manager) ExecCertificate(fun func(...*object.Certificate) error, cas ...*object.Certificate) error {
	var err error
	for i := 0; i < 3; i++ {
		err = fun(cas...)
		if err != nil {
			m.pluginErrorSize++
			time.Sleep(time.Millisecond * 10)
		} else {
			break
		}
	}
	return err
}
