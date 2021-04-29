package helmapp

import (
	"context"
	"errors"
	"strings"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	"github.com/goodrain/rainbond/worker/controllers/helmapp/helm"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
)

type ControlLoop struct {
	clientset versioned.Interface
	storer    Storer
	workQueue workqueue.Interface
	repo      *helm.Repo
	repoFile  string
	repoCache string
}

// NewControlLoop -
func NewControlLoop(clientset versioned.Interface,
	storer Storer,
	workQueue workqueue.Interface,
	repoFile string,
	repoCache string,
) *ControlLoop {
	repo := helm.NewRepo(repoFile, repoCache)

	return &ControlLoop{
		clientset: clientset,
		storer:    storer,
		workQueue: workQueue,
		repo:      repo,
		repoFile:  repoFile,
		repoCache: repoCache,
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

func (c *ControlLoop) Reconcile(helmApp *v1alpha1.HelmApp) error {
	logrus.Debugf("HelmApp Received: %s; phase: %s", k8sutil.ObjKey(helmApp), helmApp.Status.Phase)

	appStore := helmApp.Spec.AppStore
	app, err := helm.NewApp(helmApp.Name, helmApp.Namespace,
		helmApp.Spec.TemplateName, helmApp.Spec.Version, helmApp.Spec.Revision, helmApp.Spec.Overrides,
		helmApp.Spec.FullName(), appStore.URL, c.repoFile, c.repoCache)

	if err != nil {
		return err
	}

	status, continu3 := NewStatus(helmApp)

	defer func() {
		helmApp.Status = status.GetHelmAppStatus()
		appStatus, err := app.Status()
		if err != nil {
			if !errors.Is(err, driver.ErrReleaseNotFound) {
				logrus.Warningf("get app status: %v", err)
			}
		} else {
			helmApp.Status.Status = v1alpha1.HelmAppStatusStatus(appStatus.Info.Status)
			helmApp.Status.CurrentRevision = appStatus.Version
		}
		// TODO: handle the error
		if err := c.updateStatus(helmApp); err != nil {
			logrus.Warningf("update app status: %v", err)
		}
	}()

	// update condition quickly
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
		status.CurrentVersion = helmApp.Spec.Version
		status.Overrides = helmApp.Spec.Overrides
	}

	if needRollback(helmApp) {
		if err := app.Rollback(); err != nil {
			logrus.Warningf("app: %s; namespace: %s; rollback helm app: %v", helmApp.Name, helmApp.Namespace, err)
		} else {
			status.TargetRevision = helmApp.Spec.Revision
		}
	}

	return nil
}

// check if the helmApp needed to be update
func needUpdate(helmApp *v1alpha1.HelmApp) bool {
	if helmApp.Spec.PreStatus != "Configured" {
		return false
	}
	return !helmApp.OverridesEqual() || helmApp.Spec.Version != helmApp.Status.CurrentVersion
}

// check if the helmApp needed to be rollback
func needRollback(helmApp *v1alpha1.HelmApp) bool {
	return helmApp.Spec.Revision != 0 &&
		helmApp.Spec.Revision != helmApp.Status.TargetRevision
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
