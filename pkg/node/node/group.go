
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
	"github.com/goodrain/rainbond/pkg/node/core"
)

type Groups map[string]*core.Group

type jobLink struct {
	gname string
	// rule id
	rules map[string]bool
}

// map[group id]map[job id]*jobLink
// 用于 group 发生变化的时候修改相应的 job
type link map[string]map[string]*jobLink

func newLink(size int) link {
	return make(link, size)
}

func (l link) add(gid, jid, rid, gname string) {
	js, ok := l[gid]
	if !ok {
		js = make(map[string]*jobLink, 4)
		l[gid] = js
	}

	j, ok := js[jid]
	if !ok {
		j = &jobLink{
			gname: gname,
			rules: make(map[string]bool),
		}
		js[jid] = j
	}

	j.rules[rid] = true
}

func (l link) addJob(job *core.Job) {
	for _, r := range job.Rules {
		for _, gid := range r.GroupIDs {
			l.add(gid, job.ID, r.ID, job.Group)
		}
	}
}

func (l link) del(gid, jid, rid string) {
	js, ok := l[gid]
	if !ok {
		return
	}

	j, ok := js[jid]
	if !ok {
		return
	}

	delete(j.rules, rid)
	if len(j.rules) == 0 {
		delete(js, jid)
	}
}

func (l link) delJob(job *core.Job) {
	for _, r := range job.Rules {
		for _, gid := range r.GroupIDs {
			l.delGroupJob(gid, job.ID)
		}
	}
}

func (l link) delGroupJob(gid, jid string) {
	js, ok := l[gid]
	if !ok {
		return
	}

	delete(js, jid)
}

func (l link) delGroup(gid string) {
	delete(l, gid)
}
