package controller

import (
	"fmt"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/eapache/channels"
	"github.com/golang/glog"
	"github.com/goodrain/rainbond/cmd/gateway/option"
	"github.com/goodrain/rainbond/gateway/controller/openresty"
	"github.com/goodrain/rainbond/gateway/store"
	"github.com/goodrain/rainbond/gateway/v1"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/ingress-nginx/task"
)

const (
	// TryTimes -
	TryTimes = 2
)

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

	optionConfig     option.Config
	RunningConfig    *v1.Config
	RunningHttpPools []*v1.Pool

	stopCh   chan struct{}
	updateCh *channels.RingChannel
	errCh    chan error
}

func (gwc *GWController) syncGateway(key interface{}) error {
	if gwc.syncQueue.IsShuttingDown() {
		return nil
	}

	l7sv, l4sv := gwc.store.ListVirtualService()
	httpPools, tcpPools := gwc.store.ListPool()
	currentConfig := &v1.Config{
		HttpPools: httpPools,
		TCPPools:  tcpPools,
		L7VS:      l7sv,
		L4VS:      l4sv,
	}

	if gwc.RunningConfig.Equals(currentConfig) {
		logrus.Info("No need to update running configuration.")
		// refresh http pools dynamically
		gwc.refreshPools(httpPools)
		return nil
	}

	gwc.RunningConfig = currentConfig

	err := gwc.GWS.PersistConfig(gwc.RunningConfig)
	if err != nil {
		logrus.Errorf("Fail to persist Nginx config: %v\n", err)
	} else {
		// refresh http pools dynamically
		logrus.Debug("no error")
		gwc.refreshPools(httpPools)
		gwc.RunningHttpPools = httpPools
	}

	return nil
}

// Start starts Gateway
func (gwc *GWController) Start() error {
	gwc.store.Run(gwc.stopCh)

	errCh := make(chan error)
	gwc.GWS.Starts(errCh)

	go gwc.syncQueue.Run(1*time.Second, gwc.stopCh)
	// force initial sync
	gwc.syncQueue.EnqueueTask(task.GetDummyObject("initial-sync"))

	go gwc.handleEvent(errCh)

	return nil
}

func (gwc *GWController) handleEvent(errCh chan error) {
	for {
		select {
		case err := <-errCh:
			if err != nil {
				logrus.Debugf("Unexpected error: %v", err)
			}
			// TODO: 20181122 huangrh
		case event := <-gwc.updateCh.Out():
			if gwc.isShuttingDown {
				break
			}
			if evt, ok := event.(store.Event); ok {
				logrus.Infof("Event %v received - object %v", evt.Type, evt.Obj)
				gwc.syncQueue.EnqueueSkippableTask(evt.Obj)
			} else {
				glog.Warningf("Unexpected event type received %T", event)
			}
		case <-gwc.stopCh:
			break
		}
	}
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

// refreshPools refresh pools dynamically.
func (gwc *GWController) refreshPools(pools []*v1.Pool) {
	logrus.Debug("refresh pools.")
	gwc.GWS.WaitPluginReady()

	delPools, updPools := gwc.getDelUpdPools(pools)
	for i := 0; i < TryTimes; i++ {
		err := gwc.GWS.UpdatePools(updPools)
		if err == nil {
			break
		}
	}
	for i := 0; i < TryTimes; i++ {
		err := gwc.GWS.DeletePools(delPools)
		if err == nil {
			break
		}
	}
}

// getDelUpdPools returns delPools which need to delete and updPools which needs to update.
func (gwc *GWController) getDelUpdPools(updPools []*v1.Pool) ([]*v1.Pool, []*v1.Pool) {
	// updPools need to delete
	var delPools []*v1.Pool
	for _, rPool := range gwc.RunningHttpPools {
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
func NewGWController(config *option.Config, errCh chan error) *GWController {
	logrus.Debug("NewGWController...")
	gwc := &GWController{
		updateCh: channels.NewRingChannel(1024),
		errCh:    errCh,
		stopLock: &sync.Mutex{},
		stopCh:   make(chan struct{}),
	}

	gws := openresty.CreateOpenrestyService(config, &gwc.isShuttingDown)
	gwc.GWS = gws

	clientSet, err := NewClientSet(config.K8SConfPath)
	if err != nil {
		logrus.Error("can't create kubernetes's client.")
	}

	gwc.store = store.New(
		clientSet,
		gwc.updateCh)

	gwc.syncQueue = task.NewTaskQueue(gwc.syncGateway)

	return gwc
}
