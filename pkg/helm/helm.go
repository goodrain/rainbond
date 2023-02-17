package helm

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/goodrain/rainbond/util/commonutil"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/provenance"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/storage/driver"
	"helm.sh/helm/v3/pkg/strvals"
	helmtime "helm.sh/helm/v3/pkg/time"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// ReleaseInfo -
type ReleaseInfo struct {
	Revision    int           `json:"revision"`
	Updated     helmtime.Time `json:"updated"`
	Status      string        `json:"status"`
	Chart       string        `json:"chart"`
	AppVersion  string        `json:"app_version"`
	Description string        `json:"description"`
}

// ReleaseHistory -
type ReleaseHistory []ReleaseInfo

// Helm -
type Helm struct {
	cfg       *action.Configuration
	settings  *cli.EnvSettings
	namespace string

	repoFile  string
	repoCache string
}

// NewHelm creates a new helm.
func NewHelm(namespace, repoFile, repoCache string) (*Helm, error) {
	configFlags := genericclioptions.NewConfigFlags(true)
	configFlags.Namespace = commonutil.String(namespace)
	kubeClient := kube.New(configFlags)

	cfg := &action.Configuration{
		KubeClient: kubeClient,
		Log: func(s string, i ...interface{}) {
			logrus.Debugf(s, i)
		},
		RESTClientGetter: configFlags,
	}
	helmDriver := ""
	settings := cli.New()
	settings.Debug = true
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

// PreInstall -
func (h *Helm) PreInstall(name, chart, version string) error {
	_, err := h.install(name, chart, version, nil, true, ioutil.Discard)
	return err
}

// Install -
func (h *Helm) Install(name, chart, version string, overrides []string) (*release.Release, error) {
	release, err := h.install(name, chart, version, overrides, true, ioutil.Discard)
	return release, err
}

func (h *Helm) locateChart(chart, version string) (string, error) {
	repoAndName := strings.Split(chart, "/")
	if len(repoAndName) != 2 {
		return "", errors.New("invalid chart. expect repo/name, but got " + chart)
	}

	chartCache := path.Join(h.settings.RepositoryCache, chart, version)
	cp := path.Join(chartCache, repoAndName[1]+"-"+version+".tgz")
	if f, err := os.Open(cp); err == nil {
		defer f.Close()

		// check if the chart file is up to date.
		hash, err := provenance.Digest(f)
		if err != nil {
			return "", errors.Wrap(err, "digist chart file")
		}

		// get digiest from repo index.
		digest, err := h.getDigest(chart, version)
		if err != nil {
			return "", err
		}

		if hash == digest {
			return cp, nil
		}
	}

	cpo := &ChartPathOptions{}
	cpo.Version = version
	settings := h.settings
	cp, err := cpo.LocateChart(chart, chartCache, settings)
	if err != nil {
		return "", err
	}

	return cp, err
}

func (h *Helm) getDigest(chart, version string) (string, error) {
	repoAndApp := strings.Split(chart, "/")
	if len(repoAndApp) != 2 {
		return "", errors.New("wrong chart format, expect repo/name, but got " + chart)
	}
	repoName, appName := repoAndApp[0], repoAndApp[1]

	indexFile, err := repo.LoadIndexFile(path.Join(h.repoCache, repoName+"-index.yaml"))
	if err != nil {
		return "", errors.Wrap(err, "load index file")
	}

	entries, ok := indexFile.Entries[appName]
	if !ok {
		return "", errors.New(fmt.Sprintf("chart(%s) not found", chart))
	}

	for _, entry := range entries {
		if entry.Version == version {
			return entry.Digest, nil
		}
	}

	return "", errors.New(fmt.Sprintf("chart(%s) version(%s) not found", chart, version))
}

func (h *Helm) install(name, chart, version string, overrides []string, dryRun bool, out io.Writer) (*release.Release, error) {
	client := action.NewInstall(h.cfg)
	client.ReleaseName = name
	client.Namespace = h.namespace
	client.Version = version
	client.DryRun = dryRun
	//client.IsUpgrade = true
	client.ClientOnly = true

	cp, err := h.locateChart(chart, version)
	if err != nil {
		return nil, err
	}

	logrus.Debugf("CHART PATH: %s\n", cp)

	p := getter.All(h.settings)
	// User specified a value via --set
	vals, err := h.parseOverrides(overrides)
	if err != nil {
		return nil, err
	}

	// Check chart dependencies to make sure all are present in /charts
	chartRequested, err := loader.Load(cp)
	if err != nil {
		return nil, err
	}
	var crdYaml string
	crds := chartRequested.CRDObjects()
	for _, crd := range crds {
		crdYaml += string(crd.File.Data)
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
	rel, err := client.Run(chartRequested, vals)
	rel.Manifest = strings.TrimPrefix(crdYaml+"\n"+rel.Manifest, "\n")
	return rel, err
}

func (h *Helm) parseOverrides(overrides []string) (map[string]interface{}, error) {
	vals := make(map[string]interface{})
	for _, value := range overrides {
		if err := strvals.ParseInto(value, vals); err != nil {
			return nil, errors.Wrap(err, "failed parsing --set data")
		}
	}
	return vals, nil
}

// Upgrade -
func (h *Helm) Upgrade(name string, chart, version string, overrides []string) error {
	client := action.NewUpgrade(h.cfg)
	client.Namespace = h.namespace
	client.Version = version

	chartPath, err := h.locateChart(chart, version)
	if err != nil {
		return err
	}

	// User specified a value via --set
	vals, err := h.parseOverrides(overrides)
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

// Status -
func (h *Helm) Status(name string) (*release.Release, error) {
	// helm status RELEASE_NAME [flags]
	client := action.NewStatus(h.cfg)
	rel, err := client.Run(name)
	return rel, errors.Wrap(err, "helm status")
}

// Uninstall -
func (h *Helm) Uninstall(name string) error {
	logrus.Infof("uninstall helm app(%s/%s)", h.namespace, name)
	uninstall := action.NewUninstall(h.cfg)
	_, err := uninstall.Run(name)
	return err
}

// Rollback -
func (h *Helm) Rollback(name string, revision int) error {
	logrus.Infof("name: %s; revision: %d; rollback helm app", name, revision)
	client := action.NewRollback(h.cfg)
	client.Version = revision

	if err := client.Run(name); err != nil {
		return errors.Wrap(err, "helm rollback")
	}
	return nil
}

// History -
func (h *Helm) History(name string) (ReleaseHistory, error) {
	logrus.Debugf("name: %s; list helm app history", name)
	client := action.NewHistory(h.cfg)
	client.Max = 256

	hist, err := client.Run(name)
	if err != nil {
		if errors.Is(err, driver.ErrReleaseNotFound) {
			return nil, nil
		}
		return nil, errors.Wrap(err, "list helm app history")
	}

	releaseutil.Reverse(hist, releaseutil.SortByRevision)

	var rels []*release.Release
	for i := 0; i < min(len(hist), client.Max); i++ {
		rels = append(rels, hist[i])
	}

	if len(rels) == 0 {
		logrus.Debugf("name: %s; helm app history not found", name)
		return nil, nil
	}

	releaseHistory := getReleaseHistory(rels)

	return releaseHistory, nil
}

// Load loads the chart from the repository.
func (h *Helm) Load(chart, version string) (string, error) {
	return h.locateChart(chart, version)
}

// ChartPathOptions -
type ChartPathOptions struct {
	action.ChartPathOptions
}

// LocateChart looks for a chart directory in known places, and returns either the full path or an error.
func (c *ChartPathOptions) LocateChart(name, dest string, settings *cli.EnvSettings) (string, error) {
	name = strings.TrimSpace(name)
	version := strings.TrimSpace(c.Version)

	if _, err := os.Stat(name); err == nil {
		abs, err := filepath.Abs(name)
		if err != nil {
			return abs, err
		}
		if c.Verify {
			if _, err := downloader.VerifyChart(abs, c.Keyring); err != nil {
				return "", err
			}
		}
		return abs, nil
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, ".") {
		return name, errors.Errorf("path %q not found", name)
	}

	dl := downloader.ChartDownloader{
		Out:     os.Stdout,
		Keyring: c.Keyring,
		Getters: getter.All(settings),
		Options: []getter.Option{
			getter.WithBasicAuth(c.Username, c.Password),
			getter.WithTLSClientConfig(c.CertFile, c.KeyFile, c.CaFile),
			getter.WithInsecureSkipVerifyTLS(c.InsecureSkipTLSverify),
		},
		RepositoryConfig: settings.RepositoryConfig,
		RepositoryCache:  settings.RepositoryCache,
	}
	if c.Verify {
		dl.Verify = downloader.VerifyAlways
	}
	if c.RepoURL != "" {
		chartURL, err := repo.FindChartInAuthAndTLSRepoURL(c.RepoURL, c.Username, c.Password, name, version,
			c.CertFile, c.KeyFile, c.CaFile, c.InsecureSkipTLSverify, getter.All(settings))
		if err != nil {
			return "", err
		}
		name = chartURL
	}

	if err := os.MkdirAll(dest, 0755); err != nil {
		return "", err
	}

	filename, _, err := dl.DownloadTo(name, version, dest)
	if err == nil {
		lname, err := filepath.Abs(filename)
		if err != nil {
			return filename, err
		}
		return lname, nil
	} else if settings.Debug {
		return filename, err
	}

	atVersion := ""
	if version != "" {
		atVersion = fmt.Sprintf(" at version %q", version)
	}
	return filename, errors.Errorf("failed to download %q%s (hint: running `helm repo update` may help)", name, atVersion)
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

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func getReleaseHistory(rls []*release.Release) (history ReleaseHistory) {
	for i := len(rls) - 1; i >= 0; i-- {
		r := rls[i]
		c := formatChartName(r.Chart)
		s := r.Info.Status.String()
		v := r.Version
		d := r.Info.Description
		a := formatAppVersion(r.Chart)

		rInfo := ReleaseInfo{
			Revision:    v,
			Status:      s,
			Chart:       c,
			AppVersion:  a,
			Description: d,
		}
		if !r.Info.LastDeployed.IsZero() {
			rInfo.Updated = r.Info.LastDeployed
		}
		history = append(history, rInfo)
	}

	return history
}

func formatChartName(c *chart.Chart) string {
	if c == nil || c.Metadata == nil {
		// This is an edge case that has happened in prod, though we don't
		// know how: https://github.com/helm/helm/issues/1347
		return "MISSING"
	}
	return fmt.Sprintf("%s-%s", c.Name(), c.Metadata.Version)
}

func formatAppVersion(c *chart.Chart) string {
	if c == nil || c.Metadata == nil {
		// This is an edge case that has happened in prod, though we don't
		// know how: https://github.com/helm/helm/issues/1347
		return "MISSING"
	}
	return c.AppVersion()
}
