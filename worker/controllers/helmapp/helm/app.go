package helm

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/goodrain/rainbond/util/commonutil"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/yaml"
)

type App struct {
	templateName string
	repo         string
	name         string
	namespace    string
	version      string
	chartDir     string

	encodedValues string

	helm *Helm
}

func (a *App) Chart() string {
	return a.repo + "/" + a.templateName
}

func NewApp(name, namespace, templateName, repo string, version, values, repoFile, repoCache string) (*App, error) {
	configFlags := genericclioptions.NewConfigFlags(true)
	configFlags.Namespace = commonutil.String(namespace)
	kubeClient := kube.New(configFlags)

	helm, err := NewHelm(kubeClient, configFlags, repoFile, repoCache)
	if err != nil {
		return nil, err
	}

	return &App{
		name:          name,
		namespace:     namespace,
		templateName:  templateName,
		repo:          repo,
		version:       version,
		encodedValues: values,
		helm:          helm,
		chartDir:      path.Join("/tmp/helm/chart", namespace, name, version),
	}, nil
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
	if err := a.helm.PreInstall(a.templateName, a.namespace, a.Chart(), &buf); err != nil {
		return err
	}
	logrus.Infof("pre install: %s", buf.String())
	return nil
}

func (a *App) InstallOrUpdate() error {
	err := a.helm.Status(a.name)
	if errors.Is(err, driver.ErrReleaseNotFound) {
		b, err := base64.StdEncoding.DecodeString(a.encodedValues)
		if err != nil {
			return errors.Wrap(err, "decode values")
		}

		values := map[string]interface{}{}
		if err := yaml.Unmarshal(b, &values); err != nil {
			return errors.Wrap(err, "parse values")
		}

		var buf bytes.Buffer
		if err := a.helm.Install(a.templateName, a.namespace, a.Chart(), values, &buf); err != nil {
			return err
		}
		logrus.Infof("install: %s", buf.String())
		return nil
	}
	return nil
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
