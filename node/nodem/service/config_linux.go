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

// +build linux
package service

import (
	"regexp"
	"fmt"
	"github.com/goodrain/rainbond/node/nodem/client"
	"strings"
	"github.com/Sirupsen/logrus"
)

var (
	etcdReg = regexp.MustCompile(`${(\w)}`)
)

func ToConfig(svc *Service, cluster client.ClusterClient) []byte {
	if svc.Start == "" {
		logrus.Error("service start command is empty.")
		return nil
	}
	if cluster == nil {
		logrus.Error("cluster client can not is nil, at to config.")
		return nil
	}

	s := Lines{}

	s.AddTitle("[Unit]")
	s.Add("Description", svc.Name)
	for _, d := range svc.Dependences {
		s.Add("After", d+".service")
		s.Add("Requires", d+".service")
	}

	s.AddTitle("[Service]")
	if svc.Type != "simple" {
		s.Add("Type", svc.Type)
		s.Add("RemainAfterExit", "yes")
	}
	s.Add("ExecStartPre", fmt.Sprintf(`bash -c "%s"`, svc.PreStart))
	s.Add("ExecStart", fmt.Sprintf(`bash -c "%s"`, svc.Start))
	s.Add("ExecStop", fmt.Sprintf(`bash -c "%s"`, svc.Stop))
	s.Add("Restart", svc.RestartPolicy)
	s.Add("RestartSec", svc.RestartSec)

	s.AddTitle("[Install]")
	s.Add("WantedBy", "multi-user.target")

	result := InjectConfig(s.Get(), cluster)

	return []byte(result)
}

func InjectConfig(content string, cluster client.ClusterClient) string {
	for _, parantheses := range etcdReg.FindAllString(content, -1) {
		group := etcdReg.FindStringSubmatch(parantheses)
		if group == nil || len(group) < 2 {
			logrus.Warnf("Not found group for ", parantheses)
			continue
		}
		line := cluster.GetConfig(group[1])
		if line == "" {
			logrus.Warnf("Not found config %s for inject config.", group[1])
			continue
		}
		content = strings.Replace(content, group[0], line, 1)
	}
	return content
}

type Lines struct {
	str string
}

func (l *Lines) AddTitle(line string) {
	l.str = fmt.Sprintf("%s\n%s", l, line)
}

func (l *Lines) Add(k, v string) {
	if v == "" {
		return
	}
	l.str = fmt.Sprintf("%s\n%s=%s", l, k, v)
}

func (l *Lines) Get() string {
	return l.str
}