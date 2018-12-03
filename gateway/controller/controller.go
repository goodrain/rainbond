package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	client "github.com/coreos/etcd/clientv3"
	"github.com/eapache/channels"
	"github.com/goodrain/rainbond/cmd/gateway/option"
	"github.com/goodrain/rainbond/gateway/controller/openresty"
	"github.com/goodrain/rainbond/gateway/store"
	"github.com/goodrain/rainbond/gateway/v1"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/ingress-nginx/task"
)

// rainbond endpoints map
var rbdemap = make(map[string]struct{})

func init() {
	rbdemap["APISERVER_ENDPOINTS"] = struct{}{}
	rbdemap["APP_UI_ENDPOINTS"] = struct{}{}
	rbdemap["HUB_ENDPOINTS"] = struct{}{}
	rbdemap["REPO_ENDPOINTS"] = struct{}{}
}

// GWController -
type GWController struct {
	GWS   GWServicer
	store store.Storer

	syncQueue       *task.Queue
	syncRateLimiter flowcontrol.RateLimiter // TODO: use it
	isShuttingDown  bool

	// stopLock is used to enforce that only a single call to Stop send at
	// a given time. We allow stopping through an HTTP endpoint and
	// allowing concurrent stoppers leads to stack traces.
	stopLock *sync.Mutex

	ocfg *option.Config
	rcfg *v1.Config // running configuration
	rhp  []*v1.Pool // running http pools

	stopCh   chan struct{}
	updateCh *channels.RingChannel

	EtcdCli *client.Client
	ctx     context.Context
}

// Start starts Gateway
func (gwc *GWController) Start(errCh chan error) error {
	// start plugin(eg: nginx, zeus and etc)
	gwc.GWS.Start(errCh)
	// start informer
	gwc.store.Run(gwc.stopCh)

	if gwc.ocfg.EnableRbdEndpoints {
		go gwc.initRbdEndpoints(errCh)
	}

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
	gwc.EtcdCli.Close()
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
		// TODO: 20181122 huangrh
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

	if gwc.rcfg.Equals(currentConfig) {
		logrus.Info("No need to update running configuration.")
		// refresh http pools dynamically
		gwc.refreshPools(httpPools)
		return nil
	}

	gwc.rcfg = currentConfig

	err := gwc.GWS.PersistConfig(gwc.rcfg)
	if err != nil {
		// TODO: if nginx is not ready, then stop gateway
		logrus.Errorf("Fail to persist Nginx config: %v\n", err)
	} else {
		// refresh http pools dynamically
		gwc.refreshPools(httpPools)
		gwc.rhp = httpPools
	}

	return nil
}

// refreshPools refresh pools dynamically.
func (gwc *GWController) refreshPools(pools []*v1.Pool) {
	gwc.GWS.UpdatePools(pools)
}

// getDelUpdPools returns delPools which need to delete and updPools which needs to update.
func (gwc *GWController) getDelUpdPools(updPools []*v1.Pool) ([]*v1.Pool, []*v1.Pool) {
	// updPools need to delete
	var delPools []*v1.Pool
	for _, rPool := range gwc.rhp {
		flag := false
		for i, pool := range updPools {
			if rPool.Equals(pool) {
				flag = true
				// delete a pool that has no changed
				updPools = append(updPools[:i], updPools[i+1:]...)
				break
			}
		}
		if !flag {
			delPools = append(delPools, rPool)
		}
	}

	return delPools, updPools
}

//NewGWController new Gateway controller
func NewGWController(ctx context.Context, cfg *option.Config) (*GWController, error) {
	gwc := &GWController{
		updateCh: channels.NewRingChannel(1024),
		stopLock: &sync.Mutex{},
		stopCh:   make(chan struct{}),
		ocfg:     cfg,
		ctx:      ctx,
	}

	if cfg.EnableRbdEndpoints {
		// create etcd client
		cli, err := client.New(client.Config{
			Endpoints:   cfg.EtcdEndPoints,
			DialTimeout: time.Duration(cfg.EtcdTimeout) * time.Second,
		})
		if err != nil {
			return nil, err
		}
		gwc.EtcdCli = cli
	}
	gwc.GWS = openresty.CreateOpenrestyService(cfg, &gwc.isShuttingDown)
	clientSet, err := NewClientSet(cfg.K8SConfPath)
	if err != nil {
		logrus.Error("can't create kubernetes's client.")
		return nil, err
	}
	gwc.store = store.New(
		clientSet,
		gwc.updateCh,
		cfg)
	gwc.syncQueue = task.NewTaskQueue(gwc.syncGateway)
	return gwc, nil
}

// initRbdEndpoints inits rainbond endpoints
func (gwc *GWController) initRbdEndpoints(errCh chan<- error) {
	gwc.GWS.WaitPluginReady()
	// get endpoints for etcd
	gwc.watchRbdEndpoints(gwc.listEndpoints())
}
func (gwc *GWController) listEndpoints() int64 {
	// get endpoints for etcd
	resp, err := gwc.EtcdCli.Get(gwc.ctx, gwc.ocfg.RbdEndpointsKey, client.WithPrefix())
	if err != nil {
		logrus.Errorf("get rainbond service endpoint from etcd error %s", err.Error())
		return 0
	}
	var pools []*v1.Pool
	for _, kv := range resp.Kvs {
		//logrus.Debugf("key: %s; value: %s\n", string(kv.Key), string(kv.Value))
		key := strings.Replace(string(kv.Key), gwc.ocfg.RbdEndpointsKey, "", -1)
		// skip unexpected key
		if _, ok := rbdemap[key]; !ok {
			continue
		}
		var data []string
		val := strings.Replace(string(kv.Value), "http://", "", -1)
		if err := json.Unmarshal([]byte(val), &data); err != nil {
			logrus.Errorf("get rainbond service endpoint from etcd error %s", err.Error())
			continue
		}
		switch key {
		case "REPO_ENDPOINTS":
			lpools := getPool(data, "lang", "maven")
			if lpools[0].Nodes == nil || len(lpools[0].Nodes) == 0 {
				logrus.Debug("there is no endpoints for REPO_ENDPOINTS")
				continue
			}
			pools = append(pools, lpools...)
		case "HUB_ENDPOINTS":
			lpools := getPool(data, "registry")
			if lpools[0].Nodes == nil || len(lpools[0].Nodes) == 0 {
				logrus.Debug("there is no endpoints for REPO_ENDPOINTS")
				continue
			}
			pools = append(pools, lpools...)
		}
	}
	//merge app pool
	logrus.Debugf("rainbond endpoings: %v", pools)
	pools = append(pools, gwc.rhp...)
	if err := gwc.GWS.UpdatePools(pools); err != nil {
		logrus.Errorf("update pools failure %s", err.Error())
	}
	if resp.Header != nil {
		return resp.Header.Revision
	}
	return 0
}

// watchRbdEndpoints watches the change of Rainbond endpoints
func (gwc *GWController) watchRbdEndpoints(version int64) {
	logrus.Infof("Start watching Rainbond servers. Watch key: %s", gwc.ocfg.RbdEndpointsKey)
	rch := gwc.EtcdCli.Watch(gwc.ctx, gwc.ocfg.RbdEndpointsKey, client.WithPrefix(), client.WithRev(version+1))
	for wresp := range rch {
		for _, ev := range wresp.Events {
			// APISERVER_ENDPOINTS(external), APP_UI_ENDPOINTS, HUB_ENDPOINTS, REPO_ENDPOINTS
			key := strings.Replace(string(ev.Kv.Key), gwc.ocfg.RbdEndpointsKey, "", -1)
			if key == "REPO_ENDPOINTS" || key == "HUB_ENDPOINTS" {
				logrus.Debugf("%s %q : %q\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
				//only need update one
				gwc.listEndpoints()
				break
			}
		}
	}
}

func getPool(data []string, names ...string) []*v1.Pool {
	var nodes []*v1.Node
	if data != nil || len(data) > 0 {
		for _, d := range data {
			s := strings.Split(d, ":")
			p, err := strconv.Atoi(s[1])
			if err != nil {
				logrus.Warningf("Can't convert string(%s) to int", s[1])
				continue
			}
			n := &v1.Node{
				Host: s[0],
				Port: int32(p),
			}
			nodes = append(nodes, n)
		}
	}

	var pools []*v1.Pool
	for _, name := range names {
		pool := &v1.Pool{
			Meta: v1.Meta{
				Name: name,
			},
			LoadBalancingType: v1.RoundRobin,
		}
		pool.Nodes = nodes
		pools = append(pools, pool)
	}

	return pools
}
