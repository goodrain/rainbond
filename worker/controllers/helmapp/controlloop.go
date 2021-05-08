package helmapp

import (
	"context"
	"strings"
	"time"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	"github.com/goodrain/rainbond/worker/controllers/helmapp/helm"
	"github.com/sirupsen/logrus"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"
)

const (
	defaultTimeout = 3 * time.Second
)

var defaultConditionTypes = []v1alpha1.HelmAppConditionType{
	v1alpha1.HelmAppChartReady,
	v1alpha1.HelmAppPreInstalled,
	v1alpha1.HelmAppInstalled,
}

type ControlLoop struct {
	ctx        context.Context
	kubeClient clientset.Interface
	clientset  versioned.Interface
	storer     Storer
	workQueue  workqueue.Interface
	repo       *helm.Repo
	repoFile   string
	repoCache  string
}

// NewControlLoop -
func NewControlLoop(ctx context.Context,
	kubeClient clientset.Interface,
	clientset versioned.Interface,
	storer Storer,
	workQueue workqueue.Interface,
	repoFile string,
	repoCache string,
) *ControlLoop {
	repo := helm.NewRepo(repoFile, repoCache)

	return &ControlLoop{
		ctx:        ctx,
		kubeClient: kubeClient,
		clientset:  clientset,
		storer:     storer,
		workQueue:  workQueue,
		repo:       repo,
		repoFile:   repoFile,
		repoCache:  repoCache,
	}
}

func (c *ControlLoop) Run() {
	for {
		obj, shutdown := c.workQueue.Get()
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
	defer c.workQueue.Done(obj)
	name, ns := nameNamespace(key)

	helmApp, err := c.storer.GetHelmApp(ns, name)
	if err != nil {
		logrus.Warningf("[HelmAppController] [ControlLoop] get helm app(%s): %v", key, err)
		return
	}

	if err := c.Reconcile(helmApp); err != nil {
		// ignore the error, informer will push the same time into queue later.
		logrus.Warningf("[HelmAppController] [ControlLoop] [Reconcile]: %v", err)
		return
	}
}

// nameNamespace -
func nameNamespace(key string) (string, string) {
	strs := strings.Split(key, "/")
	return strs[0], strs[1]
}

func (c *ControlLoop) Reconcile(helmApp *v1alpha1.HelmApp) error {
	app, err := NewApp(c.ctx, c.kubeClient, c.clientset, helmApp, c.repoFile, c.repoCache)
	if err != nil {
		return err
	}

	app.log.Debug("start reconcile")

	// update running status
	defer app.UpdateRunningStatus()

	if app.NeedSetup() {
		return app.Setup()
	}

	if app.NeedDetect() {
		return app.Detect()
	}

	if app.NeedUpdate() {
		return app.InstallOrUpdate()
	}

	return nil
}
