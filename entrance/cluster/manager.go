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
	"github.com/goodrain/rainbond/cmd/entrance/option"
	"os"

	"github.com/Sirupsen/logrus"
	"golang.org/x/net/context"
)

type Manager struct {
	Prefix string
	Name   string
	ctx    context.Context
	cancel context.CancelFunc
}

func NewManager(c option.Config) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	manager := &Manager{
		Prefix: c.EtcdPrefix,
		Name:   c.ClusterName,
		ctx:    ctx,
		cancel: cancel,
	}
	if manager.Name == "" {
		manager.Name, _ = os.Hostname()
	}
	logrus.Info("cluster manager create.")
	return manager, nil
}

func (m *Manager) GetName() string {
	return m.Name
}

func (m *Manager) GetPrefix() string {
	return m.Prefix
}
