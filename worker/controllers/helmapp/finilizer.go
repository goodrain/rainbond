package helmapp

import (
	"context"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	"github.com/sirupsen/logrus"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
)

type Finalizer struct {
	ctx        context.Context
	log        *logrus.Entry
	kubeClient clientset.Interface
	clientset  versioned.Interface
	queue      workqueue.Interface
	repoFile   string
	repoCache  string
}

// NewControlLoop -
func NewFinalizer(ctx context.Context,
	kubeClient clientset.Interface,
	clientset versioned.Interface,
	workQueue workqueue.Interface,
	repoFile string,
	repoCache string,
) *Finalizer {

	return &Finalizer{
		ctx:        ctx,
		log:        logrus.WithField("WHO", "Finalizer"),
		kubeClient: kubeClient,
		clientset:  clientset,
		queue:      workQueue,
		repoFile:   repoFile,
		repoCache:  repoCache,
	}
}

func (c *Finalizer) Run() {
	for {
		obj, shutdown := c.queue.Get()
		if shutdown {
			return
		}

		err := c.run(obj)
		if err != nil {
			c.log.Warningf("run: %v", err)
			continue
		}
		c.queue.Done(obj)
	}
}

func (c *Finalizer) run(obj interface{}) error {
	helmApp, ok := obj.(*v1alpha1.HelmApp)
	if !ok {
		return nil
	}

	logrus.Infof("start uninstall helm app: %s/%s", helmApp.Name, helmApp.Namespace)

	app, err := NewApp(c.ctx, c.kubeClient, c.clientset, helmApp, c.repoFile, c.repoCache)
	if err != nil {
		return err
	}

	return app.Uninstall()
}
