package helm

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
)

type App struct {
	templateName string
	repo         string
	name         string
	namespace    string
	version      string
	chartDir     string

	helm *Helm
}

func (a *App) Chart() string {
	return a.repo + "/" + a.templateName
}

// TODO: use appName and templateName
func NewApp(name, namespace, templateName, repo string, version string, helm *Helm) *App {
	return &App{
		name:         name,
		namespace:    namespace,
		templateName: templateName,
		repo:         repo,
		version:      version,
		helm:         helm,
		chartDir:     path.Join("/tmp/helm/chart", namespace, name, version),
	}
}

func (a *App) Pull() error {
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
	return a.repo + "/" + a.templateName
}

func (a *App) PreInstall() error {
	var buf bytes.Buffer
	return a.helm.PreInstall(a.templateName, a.namespace, a.Chart(), &buf)
}

func (a *App) ParseChart() (string, error) {
	var values string
	err := filepath.Walk(a.chartDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.Contains(path, "values.yaml") {
			return nil
		}

		file, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		values = base64.StdEncoding.EncodeToString(file)

		return nil
	})
	return values, err
}
