package helm

import (
	"fmt"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"io"
	"sync"
)

var errNoRepositories = errors.New("no repositories found. You must add one before updating")

func updateCharts(repos []*repo.ChartRepository, out io.Writer, failOnRepoUpdateFail bool) error {
	var wg sync.WaitGroup
	var repoFailList []string
	for _, re := range repos {
		wg.Add(1)
		go func(re *repo.ChartRepository) {
			defer wg.Done()
			if _, err := re.DownloadIndexFile(); err != nil {
				repoFailList = append(repoFailList, re.Config.URL)
			}
		}(re)
	}
	wg.Wait()
	if len(repoFailList) > 0 && failOnRepoUpdateFail {
		return fmt.Errorf("Failed to update the following repositories: %s",
			repoFailList)
	}

	return nil
}

type repoUpdateOptions struct {
	update               func([]*repo.ChartRepository, io.Writer, bool) error
	repoFile             string
	repoCache            string
	names                []string
	failOnRepoUpdateFail bool
	settings             *cli.EnvSettings
}

func (h *Helm) repoUpdate(names string, out io.Writer) error {
	o := &repoUpdateOptions{update: updateCharts}
	o.repoFile = h.repoFile
	o.repoCache = h.repoCache
	o.names = []string{names}
	o.settings = h.settings
	return o.run(out)
}

func (o *repoUpdateOptions) run(out io.Writer) error {
	f, err := repo.LoadFile(o.repoFile)
	switch {
	case isNotExist(err):
		return errNoRepositories
	case err != nil:
		return errors.Wrapf(err, "failed loading file: %s", o.repoFile)
	case len(f.Repositories) == 0:
		return errNoRepositories
	}

	var repos []*repo.ChartRepository
	updateAllRepos := len(o.names) == 0

	if !updateAllRepos {
		// Fail early if the user specified an invalid repo to update
		if err := checkRequestedRepos(o.names, f.Repositories); err != nil {
			return err
		}
	}

	for _, cfg := range f.Repositories {
		if updateAllRepos || isRepoRequested(cfg.Name, o.names) {
			r, err := repo.NewChartRepository(cfg, getter.All(o.settings))
			if err != nil {
				return err
			}
			if o.repoCache != "" {
				r.CachePath = o.repoCache
			}
			repos = append(repos, r)
		}
	}

	return o.update(repos, out, o.failOnRepoUpdateFail)
}

func checkRequestedRepos(requestedRepos []string, validRepos []*repo.Entry) error {
	for _, requestedRepo := range requestedRepos {
		found := false
		for _, repo := range validRepos {
			if requestedRepo == repo.Name {
				found = true
				break
			}
		}
		if !found {
			return errors.Errorf("no repositories found matching '%s'.  Nothing will be updated", requestedRepo)
		}
	}
	return nil
}

func isRepoRequested(repoName string, requestedRepos []string) bool {
	for _, requestedRepo := range requestedRepos {
		if repoName == requestedRepo {
			return true
		}
	}
	return false
}
