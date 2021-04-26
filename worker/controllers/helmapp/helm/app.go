package helm

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sigs.k8s.io/yaml"

	"github.com/goodrain/rainbond/util/commonutil"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
)

type App struct {
	templateName string
	repoName     string
	repoURL      string
	name         string
	namespace    string
	version      string
	chartDir     string

	encodedValues string

	helm *Helm
	repo *Repo

	builder *resource.Builder
}

func (a *App) Chart() string {
	return a.repoName + "/" + a.templateName
}

func NewApp(name, namespace, templateName, version, values, repoName, repoURL, repoFile, repoCache string) (*App, error) {
	configFlags := genericclioptions.NewConfigFlags(true)
	configFlags.Namespace = commonutil.String(namespace)
	kubeClient := kube.New(configFlags)

	helm, err := NewHelm(kubeClient, configFlags, namespace, repoFile, repoCache)
	if err != nil {
		return nil, err
	}
	repo := NewRepo(repoFile, repoCache)

	return &App{
		name:          name,
		namespace:     namespace,
		templateName:  templateName,
		repoName:      repoName,
		repoURL:       repoURL,
		version:       version,
		encodedValues: values,
		helm:          helm,
		repo:          repo,
		chartDir:      path.Join("/tmp/helm/chart", namespace, name, version),
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
	if err := a.helm.PreInstall(a.name, a.Chart(), &buf); err != nil {
		return err
	}
	logrus.Infof("pre install: %s", buf.String())
	return nil
}

func (a *App) Status() (string, error) {
	release, err := a.helm.Status(a.name)
	if err != nil {
		return "", err
	}
	return string(release.Info.Status), nil
}

func (a *App) InstallOrUpdate() error {
	if err := a.repo.Add(a.repoName, a.repoURL, "", ""); err != nil {
		return err
	}

	b, err := base64.StdEncoding.DecodeString(a.encodedValues)
	if err != nil {
		return errors.Wrap(err, "decode values")
	}
	values := map[string]interface{}{}
	if err := yaml.Unmarshal(b, &values); err != nil {
		return errors.Wrap(err, "parse values")
	}

	_, err = a.helm.Status(a.name)
	if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
		return err
	}

	if errors.Is(err, driver.ErrReleaseNotFound) {
		logrus.Debugf("name: %s; namespace: %s; chart: %s; install helm app", a.name, a.namespace, a.Chart())
		if err := a.helm.Install(a.name, a.Chart(), values); err != nil {
			return err
		}

		return nil
	}

	logrus.Debugf("name: %s; namespace: %s; chart: %s; upgrade helm app", a.name, a.namespace, a.Chart())
	return a.helm.Upgrade(a.name, a.chart(), values)
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

func (a *App) parseServices(manifests string) ([]*corev1.Service, error) {
	// Create a local builder...
	builder := resource.NewLocalBuilder().
		// Configure with a scheme to get typed objects in the versions registered with the scheme.
		// As an alternative, could call Unstructured() to get unstructured objects.
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		// Provide input via a Reader.
		// As an alternative, could call Path(false, "/path/to/file") to read from a file.
		Stream(bytes.NewBufferString(manifests), "input").
		// Flatten items contained in List objects
		Flatten().
		// Accumulate as many items as possible
		ContinueOnError()

	// Run the builder
	result := builder.Do()

	items, err := result.Infos()
	if err != nil {
		return nil, errors.WithMessage(err, "resource infos")
	}

	var services []*corev1.Service
	for _, item := range items {
		if item.Object.GetObjectKind().GroupVersionKind().Kind != "Service" {
			continue
		}
		svc, ok := item.Object.(*corev1.Service)
		if !ok {
			continue
		}
		services = append(services, svc)
	}

	return services, nil
}

func (a *App) Uninstall() error {
	return a.helm.Uninstall(a.name)
}
