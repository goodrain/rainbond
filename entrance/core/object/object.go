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

package object

import (
	"fmt"
	"strings"

	"github.com/goodrain/rainbond/util"
)

//Object source object
type Object interface {
	GetName() string
	GetIndex() int64
	GetEventID() string
	GetHash() (string, error)
}

type PoolObject struct {
	ServiceID      string //租户原名.service别名 labels
	ServiceVersion string //version labels
	Index          int64
	Name           string
	Note           string //说明
	PluginName     string
	PluginOpts     map[string]string
	Namespace      string
	EventID        string
}

func (p *PoolObject) GetName() string {
	return p.Name
}
func (p *PoolObject) GetIndex() int64 {
	return p.Index
}
func (p *PoolObject) GetEventID() string {
	return p.EventID
}
func (p *PoolObject) GetHash() (string, error) {
	return util.CreateHashString(p.ServiceVersion + p.ServiceID + p.Name + p.Note + p.PluginName + p.Namespace)
}

type NodeObject struct {
	Index      int64
	Host       string
	Port       int32
	Protocol   string
	State      string //Active Draining Disabled
	PoolName   string //属于哪个pool
	NodeName   string
	Ready      bool //是否已经Ready
	PluginName string
	PluginOpts map[string]string
	Weight     int
	Namespace  string
	EventID    string
}

func (p *NodeObject) GetName() string {
	return p.NodeName
}
func (p *NodeObject) GetIndex() int64 {
	return p.Index
}
func (p *NodeObject) GetEventID() string {
	return p.EventID
}
func (p *NodeObject) GetHash() (string, error) {
	return util.CreateHashString(p.Host + fmt.Sprintf("%d%t%d", p.Port, p.Ready, p.Weight) + p.Protocol + p.State + p.PoolName + p.NodeName + p.Namespace + p.PluginName)
}

type RuleObject struct {
	Name            string //不重复的命名规则
	Index           int64
	DomainName      string //与domain中的name对应
	PoolName        string
	HTTPS           bool
	TransferHTTP    bool //转移 http到https
	CertificateName string
	PluginName      string
	PluginOpts      map[string]string
	Namespace       string
	EventID         string
}

func (p *RuleObject) GetName() string {
	return p.Name
}
func (p *RuleObject) GetIndex() int64 {
	return p.Index
}

//GetEventID get eventid
func (p *RuleObject) GetEventID() string {
	return p.EventID
}

//Copy copy
func (p *RuleObject) Copy() *RuleObject {
	var r RuleObject
	r.CertificateName = p.CertificateName
	r.DomainName = p.DomainName
	r.EventID = p.EventID
	r.Index = p.Index + 1
	r.Name = p.Name
	r.Namespace = p.Namespace
	r.PluginName = p.PluginName
	r.PluginOpts = p.PluginOpts
	r.PoolName = p.PoolName
	return &r
}

//GetHash get hash
func (p *RuleObject) GetHash() (string, error) {
	return util.CreateHashString(fmt.Sprintf("%t%t", p.HTTPS, p.TransferHTTP) + p.Name + p.DomainName + p.PoolName + p.CertificateName + p.Namespace + p.PluginName)
}

type Certificate struct {
	Name        string
	Index       int64
	Certificate string
	PrivateKey  string
	EventID     string
	PluginName  string
	PluginOpts  map[string]string
}

func (p *Certificate) GetName() string {
	return p.Name
}

func (p *Certificate) GetIndex() int64 {
	return p.Index
}

func (p *Certificate) GetEventID() string {
	return p.EventID
}

func (p *Certificate) GetHash() (string, error) {
	return util.CreateHashString(p.Name + p.Certificate + p.PrivateKey + p.PluginName)
}

type DomainObject struct {
	Name       string //不重复的命名 不同资源之间可以一样
	Domain     string
	Protocol   string
	Index      int64
	PluginName string
	PluginOpts map[string]string
	EventID    string
}

func (p *DomainObject) GetName() string {
	return p.Name
}
func (p *DomainObject) GetIndex() int64 {
	return p.Index
}
func (p *DomainObject) GetEventID() string {
	return p.EventID
}

func (p *DomainObject) GetHash() (string, error) {
	return util.CreateHashString(p.Name + p.Domain + p.Protocol + p.PluginName)
}

type VirtualServiceObject struct {
	Index           int64
	Name            string //不重复的命名
	Enabled         bool
	Protocol        string //默认为 stream
	Port            int32
	Listening       []string //if Listening is nil,will listen all
	Note            string   //说明
	DefaultPoolName string
	Rules           []string //默认无
	PluginName      string
	PluginOpts      map[string]string
	Namespace       string
	EventID         string
}

func (p *VirtualServiceObject) GetName() string {
	return p.Name
}
func (p *VirtualServiceObject) GetIndex() int64 {
	return p.Index
}
func (p *VirtualServiceObject) GetEventID() string {
	return p.EventID
}
func (p *VirtualServiceObject) GetHash() (string, error) {
	return util.CreateHashString(p.Name + p.Protocol + p.PluginName + p.Note + p.DefaultPoolName + strings.Join(p.Rules, "") + fmt.Sprintf("%t%d", p.Enabled, p.Port) + strings.Join(p.Listening, "") + p.PluginName + p.Namespace)
}
