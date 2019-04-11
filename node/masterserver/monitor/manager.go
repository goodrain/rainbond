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

package monitor

import (
	"github.com/goodrain/rainbond/node/masterserver/node"

	"github.com/prometheus/client_golang/prometheus"
)

//Manager Manager
type Manager interface {
	Start(errchan chan error) error
	Stop() error
	GetRegistry() *prometheus.Registry
}

type manager struct {
	clusterExporterRestry *prometheus.Registry
	cluster               *node.Cluster
}

//CreateManager CreateManager
func CreateManager(cluster *node.Cluster) (Manager, error) {
	clusterRegistry := prometheus.NewRegistry()
	manage := &manager{
		clusterExporterRestry: clusterRegistry,
		cluster:               cluster,
	}
	return manage, nil
}

func (m *manager) Start(errchan chan error) error {
	return m.clusterExporterRestry.Register(m.cluster)
}

func (m *manager) Stop() error {
	return nil
}

func (m *manager) GetRegistry() *prometheus.Registry {
	return m.clusterExporterRestry
}
