package helmapp

import (
	"time"

	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/util/workqueue"
)

// Controller -
type Controller struct {
	storer      Storer
	stopCh      chan struct{}
	controlLoop *ControlLoop
	finalizer   *Finalizer
}

func NewController(stopCh chan struct{}, clientset versioned.Interface, resyncPeriod time.Duration,
	repoFile, repoCache string) *Controller {
	workQueue := workqueue.New()
	finalizerQueue := workqueue.New()
	storer := NewStorer(clientset, resyncPeriod, workQueue, finalizerQueue)

	controlLoop := NewControlLoop(clientset, storer, workQueue, repoFile, repoCache)
	finalizer := NewFinalizer(clientset, finalizerQueue, repoFile, repoCache)

	return &Controller{
		storer:      storer,
		stopCh:      stopCh,
		controlLoop: controlLoop,
		finalizer:   finalizer,
	}
}

func (c *Controller) Start() {
	logrus.Info("start helm app controller")
	go c.storer.Run(c.stopCh)
	go c.controlLoop.Run()
	c.finalizer.Run()
}
