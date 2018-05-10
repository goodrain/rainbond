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

package plugin

import (
	"errors"
	"strings"
	"sync"

	"github.com/goodrain/rainbond/cmd/entrance/option"
	"github.com/goodrain/rainbond/entrance/core/object"
	"github.com/goodrain/rainbond/entrance/store"

	"github.com/Sirupsen/logrus"

	"golang.org/x/net/context"
)

//Manager lb plugin manager
type Manager struct {
	plugins              map[string]Plugin
	lock                 sync.Mutex
	ctx                  context.Context
	cancel               context.CancelFunc
	config               option.Config
	defaultPluginOptions map[string]string
}

//Context plugin context
type Context struct {
	Store  store.ReadStore
	Option map[string]string
	Ctx    context.Context
}

//Creater lb plugin creater
type Creater func(Context) (Plugin, error)

//Checker the opts
type Checker func(Context) error

//LBPluginFactory plugin factory
type LBPluginFactory struct {
	registry      map[string]Creater
	optionChecker map[string]Checker
	lock          sync.Mutex
}

func (p *LBPluginFactory) regist(name string, c Creater) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if _, ok := p.registry[name]; !ok {
		p.registry[name] = c
	}
}

func (p *LBPluginFactory) registChecker(name string, c Checker) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if _, ok := p.optionChecker[name]; !ok {
		p.optionChecker[name] = c
	}
}
func (p *LBPluginFactory) get(name string) (Creater, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if c, ok := p.registry[name]; ok {
		return c, nil
	}
	return nil, errors.New("no plugin creater regist")
}

func (p *LBPluginFactory) getChecker(name string) (Checker, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	if c, ok := p.optionChecker[name]; ok {
		return c, nil
	}
	return nil, errors.New("no plugin creater regist")
}

//getAll get all lb plugin
func (p *LBPluginFactory) getAll() ([]Creater, error) {
	p.lock.Lock()
	defer p.lock.Unlock()
	var cc []Creater
	for _, c := range p.registry {
		cc = append(cc, c)
	}
	if len(cc) > 0 {
		return cc, nil
	}
	return nil, errors.New("no plugin creater regist")
}

var factory = LBPluginFactory{registry: make(map[string]Creater), optionChecker: make(map[string]Checker)}

//RegistPlugin regist a plugin
func RegistPlugin(name string, c Creater) {
	logrus.Infof("regist a plugin %s", name)
	factory.regist(name, c)
}

//RegistPluginOptionCheck regist the options check
func RegistPluginOptionCheck(name string, c Checker) {
	factory.registChecker(name, c)
}

//GetDefaultPlugin get default plugin
func (m *Manager) GetDefaultPlugin(store store.ReadStore) (Plugin, error) {
	return m.GetPlugin(m.config.DefaultPluginName, m.defaultPluginOptions, store)
}

//GetPlugin get  plugin from name
//if name is not exist, will create it
func (m *Manager) GetPlugin(name string, opts map[string]string, store store.ReadStore) (Plugin, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if p, ok := m.plugins[name]; ok {
		return p, nil
	}
	c, err := factory.get(name)
	if err != nil {
		return nil, err
	}
	checker, err := factory.getChecker(name)
	if err != nil {
		return nil, err
	}
	context := Context{
		Option: opts,
		Ctx:    m.ctx,
		Store:  store,
	}
	if err := checker(context); err != nil {
		return nil, errors.New("plugin options is invalid." + err.Error())
	}
	p, err := c(context)
	if err != nil {
		return nil, err
	}
	m.plugins[name] = p
	return p, nil
}

//NewPluginManager new lb plugin manager
func NewPluginManager(config option.Config) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	var opts = make(map[string]string)
	for _, v := range config.DefaultPluginOpts {
		if strings.Contains(v, "=") {
			kv := strings.Split(v, "=")
			opts[kv[0]] = kv[1]
		}
	}
	p := &Manager{
		ctx:                  ctx,
		cancel:               cancel,
		plugins:              make(map[string]Plugin),
		config:               config,
		defaultPluginOptions: opts,
	}
	logrus.Info("plugin manager create.")
	return p, nil
}

//Stop stop all plugin
func (m *Manager) Stop() {
	m.cancel()
	logrus.Info("plugin manager stop.")
	for _, p := range m.plugins {
		p.Stop()
	}
}

//Plugin plugin interface
//设计注意事项
//1.需要先创建pool，再添加node
//2.需要先创建pool,再添加vs
//3.如果删除pool，不需要再删除node
//4.操作pool或者node时需要使用pool分布式锁
type Plugin interface {
	AddPool(pools ...*object.PoolObject) error
	UpdatePool(pools ...*object.PoolObject) error
	DeletePool(pools ...*object.PoolObject) error
	GetPool(name string) *object.PoolObject

	UpdateNode(nodes ...*object.NodeObject) error
	DeleteNode(nodes ...*object.NodeObject) error
	AddNode(nodes ...*object.NodeObject) error
	GetNode(name string) *object.NodeObject

	UpdateRule(rules ...*object.RuleObject) error
	DeleteRule(rules ...*object.RuleObject) error
	AddRule(rules ...*object.RuleObject) error
	GetRule(name string) *object.RuleObject

	AddDomain(domains ...*object.DomainObject) error
	UpdateDomain(domains ...*object.DomainObject) error
	DeleteDomain(domains ...*object.DomainObject) error
	GetDomain(name string) *object.DomainObject

	GetName() string
	Stop() error

	AddVirtualService(services ...*object.VirtualServiceObject) error
	UpdateVirtualService(services ...*object.VirtualServiceObject) error
	DeleteVirtualService(services ...*object.VirtualServiceObject) error
	GetVirtualService(name string) *object.VirtualServiceObject
	//GetPluginStatus 获取插件状态，用于监控
	GetPluginStatus() bool

	AddCertificate(cas ...*object.Certificate) error
	DeleteCertificate(cas ...*object.Certificate) error
}
