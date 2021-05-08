package helmapp

import (
	"context"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	"github.com/goodrain/rainbond/worker/controllers/helmapp/helm"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
)

type App struct {
	ctx            context.Context
	log            *logrus.Entry
	rainbondClient versioned.Interface
	recorder       record.EventRecorder

	helmApp         *v1alpha1.HelmApp
	originalHelmApp *v1alpha1.HelmApp

	name         string
	namespace    string
	templateName string
	version      string
	repoName     string
	repoURL      string
	overrides    []string
	revision     int
	chartDir     string

	helmCmd *helm.Helm
	repo    *helm.Repo
}

func (a *App) Chart() string {
	return a.repoName + "/" + a.templateName
}

// NewApp creates a new app.
func NewApp(ctx context.Context, kubeClient clientset.Interface, rainbondClient versioned.Interface, helmApp *v1alpha1.HelmApp, repoFile, repoCache string) (*App, error) {
	helmCmd, err := helm.NewHelm(helmApp.GetNamespace(), repoFile, repoCache)
	if err != nil {
		return nil, err
	}
	repo := helm.NewRepo(repoFile, repoCache)
	log := logrus.WithField("HelmAppController", "Reconcile").WithField("Namespace", helmApp.GetNamespace()).WithField("Name", helmApp.GetName())

	return &App{
		ctx:             ctx,
		log:             log,
		recorder:        createRecorder(kubeClient, helmApp.Name, helmApp.Namespace),
		rainbondClient:  rainbondClient,
		helmApp:         helmApp.DeepCopy(),
		originalHelmApp: helmApp,
		name:            helmApp.GetName(),
		namespace:       helmApp.GetNamespace(),
		templateName:    helmApp.Spec.TemplateName,
		repoName:        helmApp.Spec.AppStore.Name,
		repoURL:         helmApp.Spec.AppStore.URL,
		version:         helmApp.Spec.Version,
		revision:        helmApp.Spec.Revision,
		overrides:       helmApp.Spec.Overrides,
		helmCmd:         helmCmd,
		repo:            repo,
		chartDir:        path.Join("/tmp/helm/chart", helmApp.Namespace, helmApp.Name, helmApp.Spec.Version),
	}, nil
}

func createRecorder(kubeClient clientset.Interface, name, namespace string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logrus.Infof)

	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{
		Interface: v1core.New(kubeClient.CoreV1().RESTClient()).Events(namespace)})
	return eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: name})
}

// NeedSetup checks if necessary to setup default values for the helm app.
func (a *App) NeedSetup() bool {
	if a.helmApp.Spec.PreStatus == "" {
		return true
	}

	if a.helmApp.Status.Phase == "" {
		return true
	}

	for _, typ3 := range defaultConditionTypes {
		idx, _ := a.helmApp.Status.GetCondition(typ3)
		if idx == -1 {
			return true
		}
	}

	return false
}

// NeedDetect checks if necessary to detect the helm app.
func (a *App) NeedDetect() bool {
	conditionTypes := []v1alpha1.HelmAppConditionType{
		v1alpha1.HelmAppChartReady,
		v1alpha1.HelmAppPreInstalled,
	}
	for _, t := range conditionTypes {
		if !a.helmApp.Status.IsConditionTrue(t) {
			return true
		}
	}
	return false
}

// NeedUpdate check if the helmApp needed to update.
func (a *App) NeedUpdate() bool {
	if a.helmApp.Spec.PreStatus != v1alpha1.HelmAppPreStatusConfigured {
		return false
	}
	return !a.helmApp.OverridesEqual() || a.helmApp.Spec.Version != a.helmApp.Status.CurrentVersion
}

func (a *App) Setup() error {
	a.log.Info("setup the helm app")
	// setup default PreStatus
	if a.helmApp.Spec.PreStatus == "" {
		a.helmApp.Spec.PreStatus = v1alpha1.HelmAppPreStatusNotConfigured
	}

	// default phase is detecting
	if a.helmApp.Status.Phase == "" {
		a.helmApp.Status.Phase = v1alpha1.HelmAppStatusPhaseDetecting
	}

	// setup default conditions
	for _, typ3 := range defaultConditionTypes {
		_, condition := a.helmApp.Status.GetCondition(typ3)
		if condition == nil {
			a.helmApp.Status.UpdateConditionStatus(typ3, corev1.ConditionFalse)
		}
	}

	return a.Update()
}

// Update updates the helm app.
func (a *App) Update() error {
	// update status
	if err := a.UpdateStatus(); err != nil {
		return err
	}
	// use patch instead of update to void resource version conflict.
	return a.UpdateSpec()
}

// UpdateStatus updates the running status of the helm app.
func (a *App) UpdateRunningStatus() error {
	rel, err := a.Status()
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			return nil
		}
		return err
	}

	newStatus := v1alpha1.HelmAppStatusStatus(rel.Info.Status)
	if a.helmApp.Status.Status == newStatus {
		// no change
		return nil
	}

	status := NewStatus(a.ctx, a.helmApp, a.rainbondClient)

	return status.Update()
}

// UpdateStatus updates the status of the helm app.
func (a *App) UpdateStatus() error {
	status := NewStatus(a.ctx, a.helmApp, a.rainbondClient)
	return status.Update()
}

// UpdateSpec updates the helm app spec.
func (a *App) UpdateSpec() error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		ctx, cancel := context.WithTimeout(a.ctx, defaultTimeout)
		defer cancel()

		helmApp, err := a.rainbondClient.RainbondV1alpha1().HelmApps(a.helmApp.Namespace).Get(ctx, a.helmApp.Name, metav1.GetOptions{})
		if err != nil {
			return errors.Wrap(err, "get helm app before update")
		}

		a.helmApp.ResourceVersion = helmApp.ResourceVersion
		if _, err := a.rainbondClient.RainbondV1alpha1().HelmApps(a.helmApp.Namespace).Update(ctx, a.helmApp, metav1.UpdateOptions{}); err != nil {
			return errors.Wrap(err, "update helm app spec")
		}

		return nil
	})
}

// Detect detect the helm app.
func (a *App) Detect() error {
	detector := NewDetector(a.helmApp, a, a.repo)
	if err := detector.Detect(); err != nil {
		return errors.WithMessage(err, "detect helm app")
	}
	return a.UpdateStatus()
}

// LoadChart loads the chart from repository.
func (a *App) LoadChart() error {
	if err := a.repo.Add(a.repoName, a.repoURL, "", ""); err != nil {
		return err
	}

	_, err := a.helmCmd.Load(a.chart(), a.version)
	return err
}

func (a *App) chart() string {
	return a.repoName + "/" + a.templateName
}

func (a *App) PreInstall() error {
	if err := a.repo.Add(a.repoName, a.repoURL, "", ""); err != nil {
		return err
	}

	return a.helmCmd.PreInstall(a.name, a.Chart(), a.version)
}

func (a *App) Status() (*release.Release, error) {
	return a.helmCmd.Status(a.name)
}

func (a *App) InstallOrUpdate() error {
	if err := a.installOrUpdate(); err != nil {
		a.helmApp.Status.SetCondition(*v1alpha1.NewHelmAppCondition(
			v1alpha1.HelmAppInstalled, corev1.ConditionFalse, "InstallFailed", err.Error()))
		return a.UpdateStatus()
	}

	a.helmApp.Status.UpdateConditionStatus(v1alpha1.HelmAppInstalled, corev1.ConditionTrue)
	a.helmApp.Status.CurrentVersion = a.helmApp.Spec.Version
	a.helmApp.Status.Overrides = a.helmApp.Spec.Overrides
	return a.UpdateStatus()
}

func (a *App) installOrUpdate() error {
	if err := a.repo.Add(a.repoName, a.repoURL, "", ""); err != nil {
		return err
	}

	_, err := a.helmCmd.Status(a.name)
	if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
		return err
	}

	if errors.Is(err, driver.ErrReleaseNotFound) {
		logrus.Debugf("name: %s; namespace: %s; chart: %s; install helm app", a.name, a.namespace, a.Chart())
		if err := a.helmCmd.Install(a.name, a.Chart(), a.version, a.overrides); err != nil {
			return err
		}

		return nil
	}

	logrus.Debugf("name: %s; namespace: %s; chart: %s; upgrade helm app", a.name, a.namespace, a.Chart())
	return a.helmCmd.Upgrade(a.name, a.chart(), a.version, a.overrides)
}

func (a *App) ParseChart() (map[string]string, string, string, error) {
	readme, err := a.getReadme()
	if err != nil {
		return nil, "", "", err
	}

	files, err := a.getValues()
	if err != nil {
		return nil, "", "", err
	}

	questions, err := a.getQuestions()
	if err != nil {
		return nil, "", "", err
	}

	return files, readme, questions, nil
}

func (a *App) getValues() (map[string]string, error) {
	files := make(map[string]string)
	err := filepath.Walk(a.chartDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, "values.yaml") {
			return nil
		}

		valuesBytes, err := ioutil.ReadFile(p)
		if err != nil {
			return err
		}
		files[strings.Replace(p, a.chartDir, "", 1)] = base64.StdEncoding.EncodeToString(valuesBytes)

		return nil
	})

	return files, err
}

func (a *App) getReadme() (string, error) {
	return a.fileInRootChart("README.md")
}

func (a *App) getQuestions() (string, error) {
	return a.fileInRootChart("questions.yaml")
}

func (a *App) fileInRootChart(filename string) (string, error) {
	var file string
	err := filepath.Walk(a.chartDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if p == a.chartDir {
			return nil
		}
		if file != "" {
			return filepath.SkipDir
		}
		if !info.IsDir() {
			return nil
		}

		readmeFile := path.Join(p, filename)
		readmeBytes, err := ioutil.ReadFile(readmeFile)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		file = base64.StdEncoding.EncodeToString(readmeBytes)

		return nil
	})

	return file, err
}

func (a *App) Uninstall() error {
	return a.helmCmd.Uninstall(a.name)
}
