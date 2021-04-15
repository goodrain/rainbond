package helmapp

import (
	"time"

	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
)

// Controller -
type Controller struct {
	clientset   versioned.Interface
	storer      Storer
	stopCh      chan struct{}
	controlLoop *ControlLoop
}

func NewController(stopCh chan struct{}, restcfg *rest.Config, resyncPeriod time.Duration,
	repoFile, repoCache string) *Controller {
	queue := workqueue.New()
	clientset := versioned.NewForConfigOrDie(restcfg)
	storer := NewStorer(clientset, resyncPeriod, queue)

	controlLoop := NewControlLoop(clientset, storer, queue, repoFile, repoCache)

	return &Controller{
		storer:      storer,
		stopCh:      stopCh,
		controlLoop: controlLoop,
	}
}

func (c *Controller) Start() error {
	go c.storer.Run(c.stopCh)

	c.controlLoop.Run()

	return nil
}
