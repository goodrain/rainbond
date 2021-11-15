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
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	endpointapi "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	envoyv2 "github.com/goodrain/rainbond/node/core/envoy/v2"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

//DependServiceHealthController Detect the health of the dependent service
//Health based conditions：
//------- lds: discover all dependent services
//------- cds: discover all dependent services
//------- sds: every service has at least one Ready instance
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
}

//NewDependServiceHealthController create a controller
func NewDependServiceHealthController() (*DependServiceHealthController, error) {
	clusterID := os.Getenv("ENVOY_NODE_ID")
	if clusterID == "" {
		clusterID = fmt.Sprintf("%s_%s_%s", os.Getenv("NAMESPACE"), os.Getenv("PLUGIN_ID"), os.Getenv("SERVICE_NAME"))
	}
	dsc := DependServiceHealthController{
		interval:  time.Second * 5,
		clusterID: clusterID,
	}
	dsc.checkFunc = append(dsc.checkFunc, dsc.checkListener)
	dsc.checkFunc = append(dsc.checkFunc, dsc.checkClusters)
	dsc.checkFunc = append(dsc.checkFunc, dsc.checkEDS)
	xDSHost := os.Getenv("XDS_HOST_IP")
	xDSHostPort := os.Getenv("XDS_HOST_PORT")
	if xDSHostPort == "" {
		xDSHostPort = "6101"
	}
	cli, err := grpc.Dial(fmt.Sprintf("%s:%s", xDSHost, xDSHostPort), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	dsc.endpointClient = v2.NewEndpointDiscoveryServiceClient(cli)
	dsc.clusterClient = v2.NewClusterDiscoveryServiceClient(cli)
	dsc.dependServiceNames = strings.Split(os.Getenv("STARTUP_SEQUENCE_DEPENDENCIES"), ",")
	return &dsc, nil
}

//Check check all conditions
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

func (d *DependServiceHealthController) checkListener() bool {
	return true
}

func (d *DependServiceHealthController) checkClusters() bool {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	res, err := d.clusterClient.FetchClusters(ctx, &v2.DiscoveryRequest{
		Node: &core.Node{
			Cluster: d.clusterID,
			Id:      d.clusterID,
		},
	})
	if err != nil {
		logrus.Errorf("discover depend services cluster failure %s", err.Error())
		return false
	}
	clusters := envoyv2.ParseClustersResource(res.Resources)
	d.ignoreCheckEndpointsClusterName = nil
	for _, cluster := range clusters {
		if cluster.GetType() == v2.Cluster_LOGICAL_DNS {
			d.ignoreCheckEndpointsClusterName = append(d.ignoreCheckEndpointsClusterName, cluster.Name)
		}
	}
	d.clusters = clusters
	return true
}

func (d *DependServiceHealthController) checkEDS() bool {
	logrus.Infof("start checking eds; dependent service cluster names: %s", d.dependServiceNames)
	if len(d.clusters) == len(d.ignoreCheckEndpointsClusterName) {
		logrus.Info("all dependent services is domain third service.")
		return true
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	res, err := d.endpointClient.FetchEndpoints(ctx, &v2.DiscoveryRequest{
		Node: &core.Node{
			Cluster: d.clusterID,
			Id:      d.clusterID,
		},
	})
	if err != nil {
		logrus.Errorf("discover depend services endpoint failure %s", err.Error())
		return false
	}
	clusterLoadAssignments := envoyv2.ParseLocalityLbEndpointsResource(res.Resources)
	readyClusters := make(map[string]bool, len(clusterLoadAssignments))
	for _, cla := range clusterLoadAssignments {
		// clusterName := fmt.Sprintf("%s_%s_%s_%d", namespace, serviceAlias, destServiceAlias, service.Spec.Ports[0].Port)
		serviceName := ""
		clusterNameInfo := strings.Split(cla.GetClusterName(), "_")
		if len(clusterNameInfo) == 4 {
			serviceName = clusterNameInfo[2]
		}
		if serviceName == "" {
			continue
		}
		if ready, exist := readyClusters[serviceName]; exist && ready {
			continue
		}

		ready := func() bool {
			if util.StringArrayContains(d.ignoreCheckEndpointsClusterName, cla.ClusterName) {
				return true
			}
			if len(cla.Endpoints) > 0 && len(cla.Endpoints[0].LbEndpoints) > 0 {
				// first LbEndpoints healthy is not nil. so endpoint is not notreadyaddress
				if host, ok := cla.Endpoints[0].LbEndpoints[0].HostIdentifier.(*endpointapi.LbEndpoint_Endpoint); ok {
					if host.Endpoint != nil && host.Endpoint.HealthCheckConfig != nil {
						logrus.Infof("depend service (%s) start complete", cla.ClusterName)
						return true
					}
				}
			}
			return false
		}()
		logrus.Infof("cluster name: %s; ready: %v", serviceName, ready)
		readyClusters[serviceName] = ready
	}
	for _, ignoreCheckEndpointsClusterName := range d.ignoreCheckEndpointsClusterName {
		clusterNameInfo := strings.Split(ignoreCheckEndpointsClusterName, "_")
		if len(clusterNameInfo) == 4 {
			readyClusters[clusterNameInfo[2]] = true
		}
	}
	for _, cn := range d.dependServiceNames {
		if cn != "" {
			if ready := readyClusters[cn]; !ready {
				logrus.Infof("%s not ready.", cn)
				return false
			}
		}
	}
	logrus.Info("all dependent services have been started.")

	return true
}
