package helm

import (
	"io"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/kube"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type Helm struct {
	cfg *action.Configuration

	repoFile  string
	repoCache string
}

// NewHelm creates a new helm.
func NewHelm(kubeClient kube.Interface, configFlags *genericclioptions.ConfigFlags, repoFile, repoCache string) (*Helm, error) {
	cfg := &action.Configuration{
		KubeClient: kubeClient,
		Log: func(s string, i ...interface{}) {
			logrus.Debugf(s, i)
		},
		RESTClientGetter: configFlags,
	}
	helmDriver := ""
	settings := cli.New()
	if err := cfg.Init(settings.RESTClientGetter(), settings.Namespace(), helmDriver, func(format string, v ...interface{}) {
		logrus.Debugf(format, v)
	}); err != nil {
		return nil, errors.Wrap(err, "init config")
	}
	return &Helm{
		cfg:       cfg,
		repoFile:  repoFile,
		repoCache: repoCache,
	}, nil
}

func (h *Helm) PreInstall(name, namespace, chart string, out io.Writer) error {
	return h.install(name, namespace, chart, nil, true, out)
}

func (h *Helm) Install(name, namespace, chart string, vals map[string]interface{}, out io.Writer) error {
	return h.install(name, namespace, chart, vals, true, out)
}

func (h *Helm) install(name, namespace, chart string, vals map[string]interface{}, dryRun bool, out io.Writer) error {
	client := action.NewInstall(h.cfg)
	client.ReleaseName = name
	client.Namespace = namespace
	client.DryRun = dryRun

	settings := cli.New()
	settings.RepositoryCache = h.repoCache
	settings.RepositoryConfig = h.repoFile

	cp, err := client.ChartPathOptions.LocateChart(chart, settings)
	if err != nil {
		return err
	}

	logrus.Debugf("CHART PATH: %s\n", cp)

	p := getter.All(settings)

	// Check chart dependencies to make sure all are present in /charts
	chartRequested, err := loader.Load(cp)
	if err != nil {
		return err
	}

	if err := checkIfInstallable(chartRequested); err != nil {
		return err
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
					RepositoryConfig: settings.RepositoryConfig,
					RepositoryCache:  settings.RepositoryCache,
					Debug:            settings.Debug,
				}
				if err := man.Update(); err != nil {
					return err
				}
				// Reload the chart with the updated Chart.lock file.
				if chartRequested, err = loader.Load(cp); err != nil {
					return errors.Wrap(err, "failed reloading chart after repo update")
				}
			} else {
				return err
			}
		}
	}

	_, err = client.Run(chartRequested, vals)
	return err
}

func (h *Helm) Status(name string) error {
	// helm status RELEASE_NAME [flags]
	client := action.NewStatus(h.cfg)
	_, err := client.Run(name)
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
