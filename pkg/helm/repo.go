package helm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"sigs.k8s.io/yaml"
)

// Repositories that have been permanently deleted and no longer work
var deprecatedRepos = map[string]string{
	"//kubernetes-charts.storage.googleapis.com":           "https://charts.helm.sh/stable",
	"//kubernetes-charts-incubator.storage.googleapis.com": "https://charts.helm.sh/incubator",
}

// Repo -
type Repo struct {
	repoFile  string
	repoCache string

	forceUpdate           bool
	insecureSkipTLSverify bool
}

// NewRepo creates a new repo.
func NewRepo(repoFile, repoCache string) *Repo {
	return &Repo{
		repoFile:  repoFile,
		repoCache: repoCache,
	}
}

func (o *Repo) Add(name, url, username, password string) error {
	var buf bytes.Buffer
	err := o.add(&buf, name, url, username, password)
	if err != nil {
		return err
	}

	s := buf.String()
	logrus.Debugf("add repo: %s", s)

	return nil
}

func (o *Repo) add(out io.Writer, name, url, username, password string) error {
	// Block deprecated repos
	for oldURL, newURL := range deprecatedRepos {
		if strings.Contains(url, oldURL) {
			return fmt.Errorf("repoName %q is no longer available; try %q instead", url, newURL)
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
		Name:                  name,
		URL:                   url,
		Username:              username,
		Password:              password,
		InsecureSkipTLSverify: o.insecureSkipTLSverify,
	}

	// If the repoName exists do one of two things:
	// 1. If the configuration for the templateName is the same continue without error
	// 2. When the config is different require --force-update
	if !o.forceUpdate && f.Has(name) {
		existing := f.Get(name)
		if c != *existing {

			// The input coming in for the templateName is different from what is already
			// configured. Return an error.
			return errors.Errorf("repository templateName (%s) already exists, please specify a different templateName", name)
		}

		// The add is idempotent so do nothing
		fmt.Fprintf(out, "%q already exists with the same configuration, skipping\n", name)
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
		return errors.Wrapf(err, "looks like %q is not a valid chart repository or cannot be reached", url)
	}

	f.Update(&c)

	if err := f.WriteFile(o.repoFile, 0644); err != nil {
		return err
	}
	fmt.Fprintf(out, "%q has been added to your repositories\n", name)
	return nil
}
