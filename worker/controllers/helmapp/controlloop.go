package helmapp

import (
	"context"
	"strings"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	"github.com/goodrain/rainbond/worker/controllers/helmapp/helm"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/util/workqueue"
)

type ControlLoop struct {
	clientset versioned.Interface
	storer    Storer
	workqueue workqueue.Interface
	repo      *helm.Repo
	helm      *helm.Helm
}

// NewControlLoop -
func NewControlLoop(kubeClient kube.Interface,
	clientset versioned.Interface,
	configFlags *genericclioptions.ConfigFlags,
	storer Storer,
	workqueue workqueue.Interface,
	repoFile string,
	repoCache string,
) *ControlLoop {
	repo := helm.NewRepo(repoFile, repoCache)

	return &ControlLoop{
		clientset: clientset,
		storer:    storer,
		workqueue: workqueue,
		repo:      repo,
		helm:      helm.NewHelm(kubeClient, configFlags, repoFile, repoCache),
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

	status := NewStatus(helmApp.Status)

	defer func() {
		helmApp.Status = status.HelmAppStatus
		c.updateStatus(helmApp)
	}()

	detector := NewDetector(helmApp, status, c.helm, c.repo)
	err := detector.Detect()
	if err != nil {
		// TODO: create event
		return err
	}

	return nil
}

func (c *ControlLoop) updateStatus(helmApp *v1alpha1.HelmApp) error {
	// TODO: context
	if _, err := c.clientset.RainbondV1alpha1().HelmApps(helmApp.Namespace).
		UpdateStatus(context.Background(), helmApp, metav1.UpdateOptions{}); err != nil {
		// TODO: create event
		return err
	}
	return nil
}

// nameNamespace -
func nameNamespace(key string) (string, string) {
	strs := strings.Split(key, "/")
	return strs[0], strs[1]
}
