package helm

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gofrs/flock"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"strings"
	"time"
)

// Repositories that have been permanently deleted and no longer work
var deprecatedRepos = map[string]string{
	"//kubernetes-charts.storage.googleapis.com":           "https://charts.helm.sh/stable",
	"//kubernetes-charts-incubator.storage.googleapis.com": "https://charts.helm.sh/incubator",
}

// Repo -
type Repo struct {
	name        string
	url         string
	username    string
	password    string
	forceUpdate bool

	repoFile  string
	repoCache string

	insecureSkipTLSverify bool
}

func NewRepo(name, url, username, password, repoFile, repoCache string) *Repo {
	return &Repo{
		name:        name,
		url:         url,
		username:    username,
		password:    password,
		forceUpdate: true,
		repoFile:    repoFile,
		repoCache:   repoCache,
	}
}

func (o *Repo) Add() error {
	var buf bytes.Buffer
	err := o.add(&buf)
	if err != nil {
		return err
	}

	s := buf.String()
	logrus.Infof("add repo: %s", s)

	return nil
}

func (o *Repo) add(out io.Writer) error {
	// Block deprecated repos
	for oldURL, newURL := range deprecatedRepos {
		if strings.Contains(o.url, oldURL) {
			return fmt.Errorf("repo %q is no longer available; try %q instead", o.url, newURL)
		}
	}

	// Ensure the file directory exists as it is required for file locking
	err := os.MkdirAll(filepath.Dir(o.repoFile), os.ModePerm)
	if err != nil && !os.IsExist(err) {
		return err
	}

	// Acquire a file lock for process synchronization
	fileLock := flock.New(strings.Replace(o.repoFile, filepath.Ext(o.repoFile), ".lock", 1))
	lockCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	locked, err := fileLock.TryLockContext(lockCtx, time.Second)
	if err == nil && locked {
		defer fileLock.Unlock()
	}
	if err != nil {
		return err
	}

	b, err := ioutil.ReadFile(o.repoFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var f repo.File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return err
	}

	c := repo.Entry{
		Name:                  o.name,
		URL:                   o.url,
		Username:              o.username,
		Password:              o.password,
		InsecureSkipTLSverify: o.insecureSkipTLSverify,
	}

	// If the repo exists do one of two things:
	// 1. If the configuration for the name is the same continue without error
	// 2. When the config is different require --force-update
	if !o.forceUpdate && f.Has(o.name) {
		existing := f.Get(o.name)
		if c != *existing {

			// The input coming in for the name is different from what is already
			// configured. Return an error.
			return errors.Errorf("repository name (%s) already exists, please specify a different name", o.name)
		}

		// The add is idempotent so do nothing
		fmt.Fprintf(out, "%q already exists with the same configuration, skipping\n", o.name)
		return nil
	}

	settings := cli.New()
	// Disable plugins
	settings.PluginsDirectory = "/foo/bar"
	r, err := repo.NewChartRepository(&c, getter.All(settings))
	if err != nil {
		return err
	}

	if o.repoCache != "" {
		r.CachePath = o.repoCache
	}
	if _, err := r.DownloadIndexFile(); err != nil {
		return errors.Wrapf(err, "looks like %q is not a valid chart repository or cannot be reached", o.url)
	}

	f.Update(&c)

	if err := f.WriteFile(o.repoFile, 0644); err != nil {
		return err
	}
	fmt.Fprintf(out, "%q has been added to your repositories\n", o.name)
	return nil
}
