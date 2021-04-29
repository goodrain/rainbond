package helmapp

import (
	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	"github.com/goodrain/rainbond/worker/controllers/helmapp/helm"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/util/workqueue"
)

type Finalizer struct {
	clientset versioned.Interface
	queue     workqueue.Interface
	repoFile  string
	repoCache string
}

// NewControlLoop -
func NewFinalizer(clientset versioned.Interface,
	workqueue workqueue.Interface,
	repoFile string,
	repoCache string,
) *Finalizer {

	return &Finalizer{
		clientset: clientset,
		queue:     workqueue,
		repoFile:  repoFile,
		repoCache: repoCache,
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
			logrus.Warningf("[HelmAppFinalizer] run finalizer: %v", err)
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

	appStore := helmApp.Spec.AppStore
	// TODO: too much args
	app, err := helm.NewApp(helmApp.Name, helmApp.Namespace,
		helmApp.Spec.TemplateName, helmApp.Spec.Version, helmApp.Spec.Revision,
		helmApp.Spec.Overrides,
		helmApp.Spec.FullName(), appStore.URL, c.repoFile, c.repoCache)
	if err != nil {
		return err
	}

	return app.Uninstall()
}
