package helm

import (
	"io"
	"io/ioutil"
	"unsafe"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type Helm struct {
	cfg       *action.Configuration
	settings  *cli.EnvSettings
	namespace string

	repoFile  string
	repoCache string
}

// NewHelm creates a new helm.
func NewHelm(kubeClient kube.Interface, configFlags *genericclioptions.ConfigFlags, namespace, repoFile, repoCache string) (*Helm, error) {
	cfg := &action.Configuration{
		KubeClient: kubeClient,
		Log: func(s string, i ...interface{}) {
			logrus.Debugf(s, i)
		},
		RESTClientGetter: configFlags,
	}
	helmDriver := ""
	settings := cli.New()
	// set namespace
	namespacePtr := (*string)(unsafe.Pointer(settings))
	*namespacePtr = namespace
	settings.RepositoryConfig = repoFile
	settings.RepositoryCache = repoCache
	// initializes the action configuration
	if err := cfg.Init(settings.RESTClientGetter(), settings.Namespace(), helmDriver, func(format string, v ...interface{}) {
		logrus.Debugf(format, v)
	}); err != nil {
		return nil, errors.Wrap(err, "init config")
	}
	return &Helm{
		cfg:       cfg,
		settings:  settings,
		namespace: namespace,
		repoFile:  repoFile,
		repoCache: repoCache,
	}, nil
}

func (h *Helm) PreInstall(name, chart string, out io.Writer) error {
	_, err := h.install(name, chart, nil, true, out)
	return err
}

func (h *Helm) Install(name, chart string, vals map[string]interface{}) error {
	_, err := h.install(name, chart, vals, false, ioutil.Discard)
	return err
}

func (h *Helm) Manifests(name, chart string, vals map[string]interface{}, out io.Writer) (string, error) {
	rel, err := h.install(name, chart, vals, true, out)
	if err != nil {
		return "", err
	}
	return rel.Manifest, nil
}

func (h *Helm) install(name, chart string, vals map[string]interface{}, dryRun bool, out io.Writer) (*release.Release, error) {
	client := action.NewInstall(h.cfg)
	client.ReleaseName = name
	client.Namespace = h.namespace
	client.DryRun = dryRun

	cp, err := client.ChartPathOptions.LocateChart(chart, h.settings)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("CHART PATH: %s\n", cp)

	p := getter.All(h.settings)

	// Check chart dependencies to make sure all are present in /charts
	chartRequested, err := loader.Load(cp)
	if err != nil {
		return nil, err
	}

	if err := checkIfInstallable(chartRequested); err != nil {
		return nil, err
	}

	if chartRequested.Metadata.Deprecated {
		logrus.Warningf("This chart is deprecated")
	}

	if req := chartRequested.Metadata.Dependencies; req != nil {
		// If CheckDependencies returns an error, we have unfulfilled dependencies.
		// As of Helm 2.4.0, this is treated as a stopping condition:
		// https://github.com/helm/helm/issues/2209
		if err := action.CheckDependencies(chartRequested, req); err != nil {
			if client.DependencyUpdate {
				man := &downloader.Manager{
					Out:              out,
					ChartPath:        cp,
					Keyring:          client.ChartPathOptions.Keyring,
					SkipUpdate:       false,
					Getters:          p,
					RepositoryConfig: h.settings.RepositoryConfig,
					RepositoryCache:  h.settings.RepositoryCache,
					Debug:            h.settings.Debug,
				}
				if err := man.Update(); err != nil {
					return nil, err
				}
				// Reload the chart with the updated Chart.lock file.
				if chartRequested, err = loader.Load(cp); err != nil {
					return nil, errors.Wrap(err, "failed reloading chart after repoName update")
				}
			} else {
				return nil, err
			}
		}
	}

	return client.Run(chartRequested, vals)
}

func (h *Helm) Upgrade(name string, chart string, vals map[string]interface{}) error {
	client := action.NewUpgrade(h.cfg)
	client.Namespace = h.namespace

	chartPath, err := client.ChartPathOptions.LocateChart(chart, h.settings)
	if err != nil {
		return err
	}

	// Check chart dependencies to make sure all are present in /charts
	ch, err := loader.Load(chartPath)
	if err != nil {
		return err
	}
	if req := ch.Metadata.Dependencies; req != nil {
		if err := action.CheckDependencies(ch, req); err != nil {
			return err
		}
	}

	if ch.Metadata.Deprecated {
		logrus.Warningf("This chart is deprecated")
	}

	upgrade := action.NewUpgrade(h.cfg)
	upgrade.Namespace = h.namespace
	_, err = upgrade.Run(name, ch, vals)
	return err
}

func (h *Helm) Status(name string) (*release.Release, error) {
	// helm status RELEASE_NAME [flags]
	client := action.NewStatus(h.cfg)
	rel, err := client.Run(name)
	return rel, errors.Wrap(err, "helm status")
}

func (h *Helm) Uninstall(name string) error {
	uninstall := action.NewUninstall(h.cfg)
	_, err := uninstall.Run(name)
	return err
}

// checkIfInstallable validates if a chart can be installed
//
// Application chart type is only installable
func checkIfInstallable(ch *chart.Chart) error {
	switch ch.Metadata.Type {
	case "", "application":
		return nil
	}
	return errors.Errorf("%s charts are not installable", ch.Metadata.Type)
}
