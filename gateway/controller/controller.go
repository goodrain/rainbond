package controller

import (
	"github.com/Sirupsen/logrus"
	"github.com/eapache/channels"
	"github.com/golang/glog"
	"github.com/goodrain/rainbond/cmd/gateway/option"
	"github.com/goodrain/rainbond/gateway/controller/openresty"
	"github.com/goodrain/rainbond/gateway/store"
	"github.com/goodrain/rainbond/gateway/v1"
	"k8s.io/ingress-nginx/task"
	"time"
)

type GWController struct {
	GWS            GWServicer
	store          store.Storer // TODO 为什么不能是*store.Storer
	syncQueue      *task.Queue
	isShuttingDown bool

	optionConfig  option.Config
	RunningConfig *v1.Config
	RunningHttpPools  []*v1.Pool

	stopCh   chan struct{}
	updateCh *channels.RingChannel
	errCh    chan error // errCh is used to detect errors with the NGINX processes
}

func (gwc *GWController) syncGateway(key interface{}) error {
	if gwc.syncQueue.IsShuttingDown() {
		return nil
	}

	l7sv, l4sv := gwc.store.ListVirtualService()
	httpPools, tcpPools := gwc.store.ListPool()
	currentConfig := &v1.Config{
		HttpPools:httpPools,
		TCPPools:tcpPools,
		L7VS: l7sv,
		L4VS: l4sv,
	}

	if gwc.RunningConfig.Equals(currentConfig) {
		if !gwc.poolsIsEqual(httpPools) {
			// TODO: 还需要把不存在的upstream删除
			openresty.UpdateUpstreams(httpPools)
			gwc.RunningHttpPools = httpPools
		}
		logrus.Info("No need to update running configuration.")
		return nil
	}

	gwc.RunningConfig = currentConfig // TODO

	err := gwc.GWS.PersistConfig(gwc.RunningConfig)
	// update http pools dynamically
	// TODO: check if the nginx is ready.
	openresty.UpdateUpstreams(httpPools)
	gwc.RunningHttpPools = httpPools
	if err != nil {
		logrus.Errorf("Fail to persist Nginx config: %v\n", err)
	}

	return nil
}

func (gwc *GWController) Start() {
	gwc.store.Run(gwc.stopCh)

	//gws := &openresty.OpenrestyService{}
	////err := gws.Start()
	//if err != nil {
	//	logrus.Fatalf("Can not start gateway plugin: %v", err)
	//	return
	//}

	// 处理task.Queue中的task
	// 每秒同步1次, 直到<-stopCh为真
	go gwc.syncQueue.Run(1*time.Second, gwc.stopCh)
	// force initial sync
	gwc.syncQueue.EnqueueTask(task.GetDummyObject("initial-sync"))

	for {
		select {
		case event := <-gwc.updateCh.Out(): // 将ringChannel的output通道接收到event放到task.Queue中
			if gwc.isShuttingDown {
				break
			}
			if evt, ok := event.(store.Event); ok {
				logrus.Infof("Event %v received - object %v", evt.Type, evt.Obj)
				if evt.Type == store.ConfigurationEvent {
					// TODO: is this necessary? Consider removing this special case
					gwc.syncQueue.EnqueueTask(task.GetDummyObject("configmap-change"))
					continue
				}
				gwc.syncQueue.EnqueueSkippableTask(evt.Obj)
			} else {
				glog.Warningf("Unexpected event type received %T", event)
			}
		case <-gwc.stopCh:
			break
		}
	}
}

func (gwc *GWController) poolsIsEqual(currentPools []*v1.Pool) bool {
	if len(gwc.RunningHttpPools) != len(currentPools) {
		return false
	}
	for _, rp := range gwc.RunningHttpPools {
		flag := false
		for _, cp := range currentPools {
			if rp.Equals(cp) {
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

func NewGWController() *GWController {
	logrus.Debug("NewGWController...")
	gwc := &GWController{
		updateCh: channels.NewRingChannel(1024),
		errCh:    make(chan error),
	}

	gws := &openresty.OpenrestyService{}
	gwc.GWS = gws

	clientSet, err := NewClientSet("/Users/abe/Documents/admin.kubeconfig")
	if err != nil {
		// TODO
	}

	gwc.store = store.New(clientSet,
		"gateway",
		gwc.updateCh)

	gwc.syncQueue = task.NewTaskQueue(gwc.syncGateway)

	return gwc
}
