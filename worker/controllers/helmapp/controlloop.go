package helmapp

import (
	"strings"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/util/workqueue"
)

type ControlLoop struct {
	storer    Storer
	workqueue workqueue.Interface
}

// NewControlLoop -
func NewControlLoop(storer Storer,
	workqueue workqueue.Interface,
) *ControlLoop {
	return &ControlLoop{
		storer:    storer,
		workqueue: workqueue,
	}
}

func (c *ControlLoop) Run() {
	for {
		obj, shutdown := c.workqueue.Get()
		if shutdown {
			return
		}

		c.run(obj)
	}
}

func (c *ControlLoop) run(obj interface{}) {
	key, ok := obj.(string)
	if !ok {
		return
	}
	defer c.workqueue.Done(obj)
	name, ns := nameNamespace(key)

	helmApp, err := c.storer.GetHelmApp(ns, name)
	if err != nil {
		logrus.Warningf("[HelmAppController] [ControlLoop] get helm app(%s): %v", key, err)
		return
	}

	if err := c.Reconcile(helmApp); err != nil {
		// ignore the error, informer will push the same time into workqueue later.
		logrus.Warningf("[HelmAppController] [ControlLoop] [Reconcile]: %v", err)
		return
	}
}

func (c *ControlLoop) Reconcile(helmApp *v1alpha1.HelmApp) error {
	logrus.Debugf("HelmApp Received: %s", k8sutil.ObjKey(helmApp))
	return nil
}

// nameNamespace -
func nameNamespace(key string) (string, string) {
	strs := strings.Split(key, "/")
	return strs[0], strs[1]
}
