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
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/sirupsen/logrus"
)

// DependServiceHealthController Detect the health of the dependent service
// Health based conditions：
// ------- lds: discover all dependent services
// ------- cds: discover all dependent services
// ------- sds: every service has at least one Ready instance
type DependServiceHealthController struct {
	listeners                       []v2.Listener
	clusters                        []v2.Cluster
	sdsHost                         []v2.ClusterLoadAssignment
	interval                        time.Duration
	envoyDiscoverVersion            string //only support v2
	checkFunc                       []func() bool
	endpointClient                  v2.EndpointDiscoveryServiceClient
	clusterClient                   v2.ClusterDiscoveryServiceClient
	dependServiceCount              int
	clusterID                       string
	dependServiceNames              []string
	ignoreCheckEndpointsClusterName []string
	dependentComponents             []DependentComponents
}

// DependentComponents -
type DependentComponents struct {
	K8sServiceName string `json:"k8s_service_name"`
	Port           int    `json:"port"`
	Protocol       string `json:"protocol"`
}

//NewDecouplingDependServiceHealthController create a decoupling controller
func NewDecouplingDependServiceHealthController() (*DependServiceHealthController, error) {
	dsc := DependServiceHealthController{
		interval: time.Second * 5,
	}
	dsc.checkFunc = append(dsc.checkFunc, dsc.checkDependentComponentsPorts)
	dependentComponents := os.Getenv("DependentComponents")
	err := json.Unmarshal([]byte(dependentComponents), &dsc.dependentComponents)
	if err != nil {
		return nil, err
	}
	return &dsc, nil
}

// Check check all conditions
func (d *DependServiceHealthController) Check() {
	logrus.Info("start denpenent health check.")
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
			logrus.Info("Depend services all check passed, will start service")
			return
		}
		select {
		case <-ticker.C:
		}
	}
}

func (d *DependServiceHealthController) checkDependentComponentsPorts() bool {
	for _, dependentComponent := range d.dependentComponents {
		logrus.Infof("start check service %v port %v", dependentComponent.K8sServiceName, dependentComponent.Port)
		var conn net.Conn
		var err error
		address := fmt.Sprintf(dependentComponent.K8sServiceName+":%v", dependentComponent.Port)
		if dependentComponent.Protocol == "udp" {
			conn, err = net.DialTimeout("udp", address, 3*time.Second)
		} else {
			conn, err = net.DialTimeout("tcp", address, 3*time.Second)
		}
		if err != nil {
			logrus.Errorf("service %v port %v connection failed %v", dependentComponent.K8sServiceName, dependentComponent.Port, err)
			return false
		}
		if conn == nil {
			logrus.Errorf("service %v port %v connection failed", dependentComponent.K8sServiceName, dependentComponent.Port)
			return false
		}
	}
	return true
}
