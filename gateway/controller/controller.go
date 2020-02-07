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

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/goodrain/rainbond/gateway/cluster"

	"github.com/Sirupsen/logrus"
	client "github.com/coreos/etcd/clientv3"
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/cmd/gateway/option"
	"github.com/goodrain/rainbond/gateway/controller/openresty"
	"github.com/goodrain/rainbond/gateway/metric"
	"github.com/goodrain/rainbond/gateway/store"
	v1 "github.com/goodrain/rainbond/gateway/v1"
	etcdutil "github.com/goodrain/rainbond/util/etcd"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/ingress-nginx/task"
)

// rainbond endpoints map
var rbdemap = make(map[string]struct{})

func init() {
	rbdemap["APISERVER_ENDPOINTS"] = struct{}{}
	rbdemap["HUB_ENDPOINTS"] = struct{}{}
	rbdemap["REPO_ENDPOINTS"] = struct{}{}
}

// GWController -
type GWController struct {
	GWS   GWServicer
	store store.Storer

	syncQueue       *task.Queue
	syncRateLimiter flowcontrol.RateLimiter
	isShuttingDown  bool

	// stopLock is used to enforce that only a single call to Stop send at
	// a given time. We allow stopping through an HTTP endpoint and
	// allowing concurrent stoppers leads to stack traces.
	stopLock *sync.Mutex

	ocfg *option.Config
	rcfg *v1.Config // running configuration
	rrhp []*v1.Pool // running rainbond http pools
	rrtp []*v1.Pool // running rainbond tcp or udp pools

	stopCh   chan struct{}
	updateCh *channels.RingChannel

	EtcdCli *client.Client
	ctx     context.Context

	metricCollector metric.Collector
}

// Start starts Gateway
func (gwc *GWController) Start(errCh chan error) error {
	if gwc.ocfg.EnableRbdEndpoints {
		go gwc.initRbdEndpoints(errCh)
	}

	// start plugin(eg: nginx, zeus and etc)
	if err := gwc.GWS.Start(errCh); err != nil {
		return err
	}
	// start informer
	gwc.store.Run(gwc.stopCh)

	// start task queue
	go gwc.syncQueue.Run(1*time.Second, gwc.stopCh)

	// force initial sync
	gwc.syncQueue.EnqueueTask(task.GetDummyObject("initial-sync"))

	go gwc.handleEvent()

	return nil
}

// Close stops Gateway
func (gwc *GWController) Close() error {
	gwc.isShuttingDown = true
	if gwc.EtcdCli != nil {
		gwc.EtcdCli.Close()
	}
	gwc.stopLock.Lock()
	defer gwc.stopLock.Unlock()

	if gwc.syncQueue.IsShuttingDown() {
		return fmt.Errorf("shutdown already in progress")
	}

	logrus.Infof("Shutting down controller queues")
	close(gwc.stopCh) // stop the loop in *GWController#Start()
	go gwc.syncQueue.Shutdown()

	return gwc.GWS.Stop()
}

func (gwc *GWController) handleEvent() {
	for {
		select {
		case event := <-gwc.updateCh.Out(): // received k8s events
			if gwc.isShuttingDown {
				break
			}
			if evt, ok := event.(store.Event); ok {
				gwc.syncQueue.EnqueueSkippableTask(evt.Obj)
			} else {
				logrus.Warningf("Unexpected event type received %T", event)
			}
		case <-gwc.stopCh:
			break
		}
	}
}

func (gwc *GWController) syncGateway(key interface{}) error {
	gwc.syncRateLimiter.Accept()

	if gwc.syncQueue.IsShuttingDown() {
		return nil
	}
	l7sv, l4sv := gwc.store.ListVirtualService()
	httpPools, tcpPools := gwc.store.ListPool()
	currentConfig := &v1.Config{
		HTTPPools: httpPools,
		TCPPools:  tcpPools,
		L7VS:      l7sv,
		L4VS:      l4sv,
	}
	// refresh http tcp and udp pools dynamically
	httpPools = append(httpPools, gwc.rrhp...)
	tcpPools = append(tcpPools, gwc.rrtp...)
	if err := gwc.GWS.UpdatePools(httpPools, tcpPools); err != nil {
		logrus.Warningf("error updating pools: %v", err)
	}
	if gwc.rcfg.Equals(currentConfig) {
		logrus.Debug("No need to update running configuration.")
		return nil
	}
	logrus.Infof("update nginx server config file.")
	err := gwc.GWS.PersistConfig(currentConfig)
	if err != nil {
		// TODO: if nginx is not ready, then stop gateway
		logrus.Errorf("Fail to persist Nginx config: %v\n", err)
		return nil
	}

	//set metric
	remove, hosts := getHosts(gwc.rcfg, currentConfig)
	gwc.metricCollector.SetHosts(hosts)
	gwc.metricCollector.RemoveHostMetric(remove)
	gwc.metricCollector.SetServerNum(len(httpPools), len(tcpPools))

	gwc.rcfg = currentConfig
	return nil
}

//NewGWController new Gateway controller
func NewGWController(ctx context.Context, clientset kubernetes.Interface, cfg *option.Config, mc metric.Collector, node *cluster.NodeManager) (*GWController, error) {
	gwc := &GWController{
		updateCh:        channels.NewRingChannel(1024),
		syncRateLimiter: flowcontrol.NewTokenBucketRateLimiter(cfg.SyncRateLimit, 1),
		stopLock:        &sync.Mutex{},
		stopCh:          make(chan struct{}),
		ocfg:            cfg,
		ctx:             ctx,
		metricCollector: mc,
	}

	if cfg.EnableRbdEndpoints {
		// create etcd client
		etcdClientArgs := &etcdutil.ClientArgs{
			Endpoints:   cfg.EtcdEndpoint,
			CaFile:      cfg.EtcdCaFile,
			CertFile:    cfg.EtcdCertFile,
			KeyFile:     cfg.EtcdKeyFile,
			DialTimeout: time.Duration(cfg.EtcdTimeout) * time.Second,
		}
		cli, err := etcdutil.NewClient(ctx, etcdClientArgs)
		if err != nil {
			return nil, err
		}
		gwc.EtcdCli = cli
	}
	gwc.GWS = openresty.CreateOpenrestyService(cfg, &gwc.isShuttingDown)

	gwc.store = store.New(
		clientset,
		gwc.updateCh,
		cfg, node)
	gwc.syncQueue = task.NewTaskQueue(gwc.syncGateway)

	return gwc, nil
}

// initRbdEndpoints inits rainbond endpoints
func (gwc *GWController) initRbdEndpoints(errCh chan<- error) {
	gwc.GWS.WaitPluginReady()

	// get endpoints for etcd
	rbdEdps, i := gwc.listRbdEndpoints()
	gwc.updateRbdPools(rbdEdps)

	gwc.watchRbdEndpoints(i)
}

// updateRbdPools updates rainbond pools
func (gwc *GWController) updateRbdPools(edps map[string][]string) {
	if ok, _ := gwc.syncRbdPools(edps); ok {
		if err := gwc.syncGateway(nil); err != nil {
			logrus.Errorf("sync gateway rule failure %s after update rainbond pool", err.Error())
		}
		logrus.Debugf("update rainbond pools")
	}
}

// syncRbdPools returns rainbond pools
func (gwc *GWController) syncRbdPools(edps map[string][]string) (bool, error) {
	var hpools []*v1.Pool // http pools
	var tpools []*v1.Pool // tcp pools
	if gwc.ocfg.EnableKApiServer {
		pools := convIntoRbdPools(edps["APISERVER_ENDPOINTS"], "kube_apiserver")
		if pools != nil && len(pools) > 0 {
			for _, pool := range pools {
				pool.LeastConn = true
				for _, node := range pool.Nodes {
					node.MaxFails = 2
					node.FailTimeout = "30s"
				}
			}
			tpools = append(tpools, pools...)
		} else {
			logrus.Debugf("there is no endpoints for %s", "kube-apiserver")
		}
	}
	if gwc.ocfg.EnableLangGrMe {
		pools := convIntoRbdPools(edps["REPO_ENDPOINTS"], "lang")
		if pools != nil && len(pools) > 0 {
			hpools = append(hpools, pools...)
		} else {
			logrus.Debugf("there is no endpoints for %s", "lang.goodrain.me")
		}
	}
	if gwc.ocfg.EnableMVNGrMe {
		pools := convIntoRbdPools(edps["REPO_ENDPOINTS"], "maven")
		if pools != nil && len(pools) > 0 {
			hpools = append(hpools, pools...)
		} else {
			logrus.Debugf("there is no endpoints for %s", "maven.goodrain.me")
		}
	}
	if gwc.ocfg.EnableGrMe {
		pools := convIntoRbdPools(edps["HUB_ENDPOINTS"], "registry")
		if pools != nil && len(pools) > 0 {
			for _, p := range pools {
				p.UpstreamHashBy = "$remote_addr"
			}
			hpools = append(hpools, pools...)
		} else {
			logrus.Debugf("there is no endpoints for %s", "maven.goodrain.me")
		}
	}
	defer func() {
		gwc.rrhp = hpools
		gwc.rrtp = tpools
	}()
	if !poolsEqual(gwc.rrhp, hpools) || !poolsEqual(gwc.rrtp, tpools) {
		return true, nil
	}
	return false, nil
}

func poolsEqual(a []*v1.Pool, b []*v1.Pool) bool {
	if len(a) != len(b) {
		return false
	}
	for _, ap := range a {
		flag := false
		for _, bp := range b {
			if ap.Equals(bp) {
				flag = true
				break
			}
		}
		if !flag {
			return false
		}
	}
	return true
}

// listRbdEndpoints lists rainbond endpoints form etcd
func (gwc *GWController) listRbdEndpoints() (map[string][]string, int64) {
	// get endpoints for etcd
	resp, err := gwc.EtcdCli.Get(gwc.ctx, gwc.ocfg.RbdEndpointsKey, client.WithPrefix())
	if err != nil {
		logrus.Errorf("get rainbond service endpoint from etcd error %s", err.Error())
		return nil, 0
	}
	rbdEdps := make(map[string][]string)
	for _, kv := range resp.Kvs {
		key := strings.Replace(string(kv.Key), gwc.ocfg.RbdEndpointsKey, "", -1)
		s := strings.Split(key, "/")
		if len(s) < 1 {
			continue
		}
		key = s[0]
		// skip unexpected key
		if _, ok := rbdemap[key]; !ok {
			continue
		}
		var data []string
		val := strings.Replace(string(kv.Value), "http://", "", -1)
		if err := json.Unmarshal([]byte(val), &data); err != nil {
			logrus.Warningf("get rainbond service endpoint from etcd error %s", err.Error())
			continue
		}

		var d []string
		for _, dat := range data {
			s := strings.Split(dat, ":")
			if len(s) != 2 || strings.Replace(s[0], " ", "", -1) == "" {
				logrus.Warningf("wrong endpoint: %s", dat)
				continue
			}
			if _, err := strconv.Atoi(s[1]); err != nil {
				logrus.Warningf("wrong endpoint: %s: %v", dat, err)
				continue
			}
			d = append(d, dat)
		}
		rbdEdps[key] = append(rbdEdps[key], d...)
	}
	if resp.Header != nil {
		return rbdEdps, resp.Header.Revision
	}
	return rbdEdps, 0
}

// watchRbdEndpoints watches the change of Rainbond endpoints
func (gwc *GWController) watchRbdEndpoints(version int64) {
	logrus.Infof("Start watching Rainbond servers. Watch key: %s", gwc.ocfg.RbdEndpointsKey)
	rch := gwc.EtcdCli.Watch(gwc.ctx, gwc.ocfg.RbdEndpointsKey, client.WithPrefix(), client.WithRev(version+1))
	for wresp := range rch {
		for _, ev := range wresp.Events {
			key := strings.Replace(string(ev.Kv.Key), gwc.ocfg.RbdEndpointsKey, "", -1)
			if strings.HasPrefix(key, "REPO_ENDPOINTS") ||
				strings.HasPrefix(key, "HUB_ENDPOINTS") ||
				strings.HasPrefix(key, "APISERVER_ENDPOINTS") {
				logrus.Debugf("%s %q : %q\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
				//only need update one
				edps, _ := gwc.listRbdEndpoints()
				gwc.updateRbdPools(edps)
				break
			}
		}
	}
}

// convIntoRbdPools converts data, contains rainbond endpoints, into rainbond pools
func convIntoRbdPools(data []string, names ...string) []*v1.Pool {
	var nodes []*v1.Node
	if data != nil && len(data) > 0 {
		for _, d := range data {
			s := strings.Split(d, ":")
			p, err := strconv.Atoi(s[1])
			if err != nil {
				logrus.Warningf("Can't convert string(%s) to int", s[1])
				continue
			}
			n := &v1.Node{
				Host:   s[0],
				Port:   int32(p),
				Weight: 1,
			}
			nodes = append(nodes, n)
		}
	}
	var pools []*v1.Pool
	// make sure every pool has nodes
	if nodes != nil && len(nodes) > 0 {
		for _, name := range names {
			pool := &v1.Pool{
				Meta: v1.Meta{
					Name:      name,
					Namespace: "rainbond",
				},
				LoadBalancingType: v1.RoundRobin,
			}
			pool.Nodes = nodes
			pools = append(pools, pool)
		}
	}
	return pools
}

// getHosts returns a list of the hostsnames and tobe remove hostname
// that are not associated anymore to the NGINX configuration.
func getHosts(rucfg, newcfg *v1.Config) (remove []string, current sets.String) {
	old := sets.NewString()
	new := sets.NewString()
	if rucfg != nil {
		for _, s := range rucfg.L7VS {
			if !old.Has(s.ServerName) {
				old.Insert(s.ServerName)
			}
		}
	}
	if newcfg != nil {
		for _, s := range newcfg.L7VS {
			if !new.Has(s.ServerName) {
				new.Insert(s.ServerName)
			}
		}
	}
	return old.Difference(new).List(), new
}
