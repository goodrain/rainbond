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

	RunningConfig *v1.Config
	optionConfig  option.Config

	stopCh   chan struct{}
	updateCh *channels.RingChannel
	errCh    chan error // errCh is used to detect errors with the NGINX processes
}

func (gwc *GWController) syncGateway(key interface{}) error {
	if gwc.syncQueue.IsShuttingDown() {
		return nil
	}

	gwc.store.InitSecret()

	currentConfig := &v1.Config{}
	currentConfig.Pools = gwc.store.ListPool()
	currentConfig.VirtualServices = gwc.store.ListVirtualService()

	if gwc.RunningConfig.Equals(currentConfig) {
		logrus.Info("No need to update running configuration.")
		return nil
	}

	gwc.RunningConfig = currentConfig // TODO

	err := gwc.GWS.PersistConfig(gwc.RunningConfig)
	if err != nil {
		logrus.Errorf("Fail to persist Nginx config: %v\n", err)
	}

	return nil
}

func (gwc *GWController) Start() {
	gwc.store.Run(gwc.stopCh)

	gws := &openresty.OpenrestyService{}
	err := gws.Start()
	if err != nil {
		logrus.Fatalf("Can not start gateway plugin: %v", err)
		return
	}

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

	// 创建Ingress的syncQueue，每往syncQueue插入一个Ingress对象，就会调用syncIngress一次
	// gwc.syncIngress方法会收集组装NGINX配置文件所需的所有东西，并在有必要重新加载时, 将结果数据结构传递给backend（OnUpdate）。
	gwc.syncQueue = task.NewTaskQueue(gwc.syncGateway)

	return gwc
}
