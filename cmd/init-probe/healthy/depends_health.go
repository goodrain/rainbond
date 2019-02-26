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

package healthy

import (
	"time"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"

	"github.com/Sirupsen/logrus"
)

//DependServiceHealthController Detect the health of the dependent service
//Health based conditions：
//------- lds: discover all dependent services
//------- cds: discover all dependent services
//------- sds: every service has at least one Ready instance
type DependServiceHealthController struct {
	listeners            []v2.Listener
	clusters             []v2.Cluster
	sdsHost              []v2.ClusterLoadAssignment
	interval             time.Duration
	envoyDiscoverVersion string //only support v2
	checkFunc            []func() bool
}

//NewDependServiceHealthController create a controller
func NewDependServiceHealthController(serviceName string) *DependServiceHealthController {
	var dsc DependServiceHealthController
	dsc.checkFunc = append(dsc.checkFunc, dsc.checkListener)
	dsc.checkFunc = append(dsc.checkFunc, dsc.checkClusters)
	dsc.checkFunc = append(dsc.checkFunc, dsc.checkSDS)
	return &dsc
}

//Check check all conditions
func (d *DependServiceHealthController) Check() {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	check := func() bool {
		for _, check := range d.checkFunc {
			if !check() {
				return false
			}
		}
		return true
	}
	for {
		if check() {
			logrus.Info("Depend services all check passed,will start service")
		}
		select {
		case <-ticker.C:
		}
	}
}

func (d *DependServiceHealthController) checkListener() bool {
	if d.listeners != nil {
		return true
	}

	return false
}

func (d *DependServiceHealthController) checkClusters() bool {
	if d.clusters != nil {
		return true
	}
	return false
}

func (d *DependServiceHealthController) checkSDS() bool {
	if d.sdsHost != nil && len(d.sdsHost) >= len(d.clusters) {
		return true
	}
	return false
}
