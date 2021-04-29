package helm

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
)

type App struct {
	templateName string
	repoName     string
	repoURL      string
	name         string
	namespace    string
	version      string
	chartDir     string
	revision     int
	overrides    []string

	helm *Helm
	repo *Repo
}

func (a *App) Chart() string {
	return a.repoName + "/" + a.templateName
}

func NewApp(name, namespace, templateName, version string, revision int, overrides []string, repoName, repoURL, repoFile, repoCache string) (*App, error) {
	helm, err := NewHelm(namespace, repoFile, repoCache)
	if err != nil {
		return nil, err
	}
	repo := NewRepo(repoFile, repoCache)

	return &App{
		name:         name,
		namespace:    namespace,
		templateName: templateName,
		repoName:     repoName,
		repoURL:      repoURL,
		version:      version,
		revision:     revision,
		overrides:    overrides,
		helm:         helm,
		repo:         repo,
		chartDir:     path.Join("/tmp/helm/chart", namespace, name, version),
	}, nil
}

func (a *App) Pull() error {
	if err := a.repo.Add(a.repoName, a.repoURL, "", ""); err != nil {
		return err
	}

	client := action.NewPull()
	settings := cli.New()
	settings.RepositoryConfig = a.helm.repoFile
	settings.RepositoryCache = a.helm.repoCache
	client.Settings = settings
	client.DestDir = a.chartDir
	client.Version = a.version
	client.Untar = true

	if err := os.RemoveAll(a.chartDir); err != nil {
		return errors.WithMessage(err, "clean up chart dir")
	}

	output, err := client.Run(a.chart())
	if err != nil {
		return err
	}
	logrus.Info(output)
	return nil
}

func (a *App) chart() string {
	return a.repoName + "/" + a.templateName
}

func (a *App) PreInstall() error {
	if err := a.repo.Add(a.repoName, a.repoURL, "", ""); err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := a.helm.PreInstall(a.name, a.Chart(), a.version, &buf); err != nil {
		return err
	}
	logrus.Infof("pre install: %s", buf.String())
	return nil
}

func (a *App) Status() (*release.Release, error) {
	rel, err := a.helm.Status(a.name)
	if err != nil {
		return nil, err
	}
	return rel, nil
}

func (a *App) InstallOrUpdate() error {
	if err := a.repo.Add(a.repoName, a.repoURL, "", ""); err != nil {
		return err
	}

	_, err := a.helm.Status(a.name)
	if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
		return err
	}

	if errors.Is(err, driver.ErrReleaseNotFound) {
		logrus.Debugf("name: %s; namespace: %s; chart: %s; install helm app", a.name, a.namespace, a.Chart())
		if err := a.helm.Install(a.name, a.Chart(), a.version, a.overrides); err != nil {
			return err
		}

		return nil
	}

	logrus.Debugf("name: %s; namespace: %s; chart: %s; upgrade helm app", a.name, a.namespace, a.Chart())
	return a.helm.Upgrade(a.name, a.chart(), a.version, a.overrides)
}

func (a *App) ParseChart() (string, string, error) {
	var values string
	var readme string
	err := filepath.Walk(a.chartDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if p == a.chartDir {
			return nil
		}
		if values != "" || readme != "" {
			return filepath.SkipDir
		}
		if !info.IsDir() {
			return nil
		}

		valuesFile := path.Join(p, "values.yaml")
		valuesBytes, err := ioutil.ReadFile(valuesFile)
		if err != nil {
			return err
		}
		values = base64.StdEncoding.EncodeToString(valuesBytes)

		readmeFile := path.Join(p, "README.md")
		readmeBytes, err := ioutil.ReadFile(readmeFile)
		if err != nil {
			return err
		}
		readme = base64.StdEncoding.EncodeToString(readmeBytes)

		return nil
	})

	return values, readme, err
}

func (a *App) Uninstall() error {
	return a.helm.Uninstall(a.name)
}

func (a *App) Rollback() error {
	if err := a.repo.Add(a.repoName, a.repoURL, "", ""); err != nil {
		return err
	}

	return a.helm.Rollback(a.name, a.revision)
}
