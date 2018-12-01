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
	errCh    chan error

	EtcdCli *client.Client
	ctx     context.Context
}

// Start starts Gateway
func (gwc *GWController) Start() error {
	// start informer
	gwc.store.Run(gwc.stopCh)

	if gwc.ocfg.EnableRbdEndpoints {
		go gwc.initRbdEndpoints()
		go gwc.watchRbdEndpoints()
	}

	// start plugin(eg: nginx, zeus and etc)
	errCh := make(chan error)
	gwc.GWS.Start(errCh)

	// start task queue
	go gwc.syncQueue.Run(1*time.Second, gwc.stopCh)

	// force initial sync
	gwc.syncQueue.EnqueueTask(task.GetDummyObject("initial-sync"))

	go gwc.handleEvent(errCh)

	return nil
}

// Stop stops Gateway
func (gwc *GWController) Stop() error {
	gwc.isShuttingDown = true

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

func (gwc *GWController) handleEvent(errCh chan error) {
	for {
		select {
		case err := <-errCh:
			if err != nil {
				logrus.Debugf("Unexpected error: %v", err)
			}
			// TODO: 20181122 huangrh
		case event := <-gwc.updateCh.Out(): // received k8s events
			if gwc.isShuttingDown {
				break
			}
			if evt, ok := event.(store.Event); ok {
				logrus.Debugf("Event %v received - object %v", evt.Type, evt.Obj)
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
	gwc.GWS.WaitPluginReady()

	delPools, updPools := gwc.getDelUpdPools(pools)
	// delete delPools first, then update updPools
	tryTimes := 3
	for i := 0; i < tryTimes; i++ {
		err := gwc.GWS.DeletePools(delPools)
		if err == nil {
			break
		}
	}
	for i := 0; i < tryTimes; i++ {
		err := gwc.GWS.UpdatePools(updPools)
		if err == nil {
			break
		}
	}
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
func NewGWController(ctx context.Context, cfg *option.Config, errCh chan error, ) *GWController {
	gwc := &GWController{
		updateCh: channels.NewRingChannel(1024),
		errCh:    errCh,
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
			logrus.Error(err)
			errCh <- err
		}
		gwc.EtcdCli = cli
	}

	gwc.GWS = openresty.CreateOpenrestyService(cfg, &gwc.isShuttingDown)

	clientSet, err := NewClientSet(cfg.K8SConfPath)
	if err != nil {
		logrus.Error("can't create kubernetes's client.")
		errCh <- err
	}

	gwc.store = store.New(
		clientSet,
		gwc.updateCh,
		cfg)

	gwc.syncQueue = task.NewTaskQueue(gwc.syncGateway)

	return gwc
}

// initRbdEndpoints inits rainbond endpoints
func (gwc *GWController) initRbdEndpoints() {
	gwc.GWS.WaitPluginReady()

	// get endpoints for etcd
	resp, err := gwc.EtcdCli.Get(gwc.ctx, gwc.ocfg.RbdEndpointsKey, client.WithPrefix())
	if err != nil {
		// error occurred -> stop gateway
		gwc.errCh <- err
	}
	for _, kv := range resp.Kvs {
		logrus.Debugf("key: %s; value: %s\n", string(kv.Key), string(kv.Value))
		key := strings.Replace(string(kv.Key), gwc.ocfg.RbdEndpointsKey, "", -1)
		// skip unexpected key
		if _, ok := rbdemap[key]; !ok {
			continue
		}
		var data []string
		val := strings.Replace(string(kv.Value), "http://", "", -1)
		if err := json.Unmarshal([]byte(val), &data); err != nil {
			// error occurred -> stop gateway
			gwc.errCh <- err
		}
		switch key {
		case "REPO_ENDPOINTS":
			pools := getPool(data, "lang", "maven")
			if pools[0].Nodes == nil || len(pools[0].Nodes) == 0 {
				gwc.errCh <- fmt.Errorf("there is no endpoints for REPO_ENDPOINTS")
			}
			err := gwc.GWS.UpdatePools(pools)
			if err != nil {
				logrus.Warningf("Unexpected error whiling updating pools: %v", err)
				gwc.errCh <- fmt.Errorf("Unexpected error whiling updating pools: %v", err)
			}
		case "HUB_ENDPOINTS":
			pools := getPool(data, "registry")
			if pools[0].Nodes == nil || len(pools[0].Nodes) == 0 {
				gwc.errCh <- fmt.Errorf("there is no endpoints for REPO_ENDPOINTS")
			}
			err := gwc.GWS.UpdatePools(pools)
			if err != nil {
				logrus.Warningf("Unexpected error whiling updating pools: %v", err)
				gwc.errCh <- fmt.Errorf("Unexpected error whiling updating pools: %v", err)
			}
		}
	}
}

// watchRbdEndpoints watches the change of Rainbond endpoints
func (gwc *GWController) watchRbdEndpoints() {
	gwc.GWS.WaitPluginReady()
	logrus.Infof("Start watching Rainbond servers. Watch key: %s", gwc.ocfg.RbdEndpointsKey)
	rch := gwc.EtcdCli.Watch(gwc.ctx, gwc.ocfg.RbdEndpointsKey, client.WithPrefix())
	for wresp := range rch {
		for _, ev := range wresp.Events {
			// APISERVER_ENDPOINTS(external), APP_UI_ENDPOINTS, HUB_ENDPOINTS, REPO_ENDPOINTS
			logrus.Debugf("%s %q : %q\n", ev.Type, ev.Kv.Key, ev.Kv.Value)
			gwc.updDelPools(ev)
		}
	}
}

// updDelPools updates or deletes pools of rainbond endpoints
func (gwc *GWController) updDelPools(ev *client.Event) {
	key := strings.Replace(string(ev.Kv.Key), gwc.ocfg.RbdEndpointsKey, "", -1)
	if ev.IsCreate() || ev.IsModify() {
		var data []string
		val := strings.Replace(string(ev.Kv.Value), "http://", "", -1)
		if err := json.Unmarshal([]byte(val), &data); err != nil {
			logrus.Warningf("Unexpected error while unmarshaling string(%s) to slice: %v", val, err)
			return
		}
		switch key {
		case "REPO_ENDPOINTS":
			pools := getPool(data, "lang", "maven")
			err := gwc.GWS.UpdatePools(pools)
			if err != nil {
				logrus.Warningf("Unexpected error whiling updating pools: %v", err)
			}
		case "HUB_ENDPOINTS":
			pools := getPool(data, "registry")
			err := gwc.GWS.UpdatePools(pools)
			if err != nil {
				logrus.Warningf("Unexpected error whiling updating pools: %v", err)
			}
		}

	} else {
		switch key {
		case "REPO_ENDPOINTS":
			pools := getPool(nil, "lang", "maven")
			err := gwc.GWS.DeletePools(pools)
			if err != nil {
				logrus.Warningf("Unexpected error whiling deleting pools: %v", err)
			}
		case "HUB_ENDPOINTS":
			pools := getPool(nil, "registry")
			err := gwc.GWS.DeletePools(pools)
			if err != nil {
				logrus.Warningf("Unexpected error whiling deleting pools: %v", err)
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
		}
		pool.Nodes = nodes
		pools = append(pools, pool)
	}

	return pools
}
