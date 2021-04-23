package helmapp

import (
	"context"
	"strings"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	"github.com/goodrain/rainbond/worker/controllers/helmapp/helm"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
)

type ControlLoop struct {
	clientset versioned.Interface
	storer    Storer
	workqueue workqueue.Interface
	repo      *helm.Repo
	repoFile  string
	repoCache string
}

// NewControlLoop -
func NewControlLoop(clientset versioned.Interface,
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
		repoFile:  repoFile,
		repoCache: repoCache,
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
		// ignore the error, informer will push the same time into queue later.
		logrus.Warningf("[HelmAppController] [ControlLoop] [Reconcile]: %v", err)
		return
	}
}

func (c *ControlLoop) Reconcile(helmApp *v1alpha1.HelmApp) error {
	logrus.Debugf("HelmApp Received: %s; phase: %s", k8sutil.ObjKey(helmApp), helmApp.Status.Phase)

	appStore := helmApp.Spec.AppStore
	app, err := helm.NewApp(helmApp.Name, helmApp.Namespace,
		helmApp.Spec.TemplateName, helmApp.Spec.Version, helmApp.Spec.Values,
		appStore.FullName(), appStore.URL, c.repoFile, c.repoCache)

	if err != nil {
		return err
	}

	status, continu3 := NewStatus(helmApp)

	defer func() {
		helmApp.Status = status.GetHelmAppStatus()
		s, _ := app.Status()
		helmApp.Status.Status = v1alpha1.HelmAppStatusStatus(s)
		// TODO: handle the error
		c.updateStatus(helmApp)
	}()

	if !continu3 {
		return nil
	}

	detector := NewDetector(helmApp, status, app, c.repo)
	if err := detector.Detect(); err != nil {
		// TODO: create event
		return err
	}

	if needUpdate(helmApp) {
		if err := app.InstallOrUpdate(); err != nil {
			status.SetCondition(*v1alpha1.NewHelmAppCondition(
				v1alpha1.HelmAppInstalled, corev1.ConditionFalse, "InstallFailed", err.Error()))
			return err
		}
		status.UpdateConditionStatus(v1alpha1.HelmAppInstalled, corev1.ConditionTrue)
		status.CurrentValues = helmApp.Spec.Values
	}

	return nil
}

func needUpdate(helmApp *v1alpha1.HelmApp) bool {
	return helmApp.Spec.Values != helmApp.Status.CurrentValues
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
