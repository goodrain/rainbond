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

package master

import (
	"fmt"

	"github.com/goodrain/rainbond/worker/appm/store"

	"github.com/goodrain/rainbond/cmd/worker/option"
	"github.com/goodrain/rainbond/util/leader"
)

//Controller app runtime master controller
type Controller struct {
	conf  option.Config
	store store.Storer
}

//NewMasterController new master controller
func NewMasterController(conf option.Config, store store.Storer) *Controller {
	return &Controller{
		conf:  conf,
		store: store,
	}
}

//Start start
func (m *Controller) Start() error {
	start := func(stop <-chan struct{}) {
		<-stop
	}
	// Leader election was requested.
	if m.conf.LeaderElectionNamespace == "" {
		return fmt.Errorf("-leader-election-namespace must not be empty")
	}
	if m.conf.LeaderElectionIdentity == "" {
		m.conf.LeaderElectionIdentity = m.conf.NodeName
	}
	if m.conf.LeaderElectionIdentity == "" {
		return fmt.Errorf("-leader-election-identity must not be empty")
	}
	// Name of config map with leader election lock
	lockName := "rainbond-appruntime-worker-leader"

	leader.RunAsLeader(m.conf.KubeClient, m.conf.LeaderElectionNamespace, m.conf.LeaderElectionIdentity, lockName, start, func() {})
	return nil
}
