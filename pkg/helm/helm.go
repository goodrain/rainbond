package helm

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
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
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/storage/driver"
	"helm.sh/helm/v3/pkg/strvals"
	helmtime "helm.sh/helm/v3/pkg/time"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/yaml"
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

var ociRefTagRegexp = regexp.MustCompile(`^(oci://[^:]+(:[0-9]{1,5})?[^:]+):(.*)$`)

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
	settings.RegistryConfig = filepath.Join(filepath.Dir(filepath.Dir(repoFile)), "registry", "config.json")
	if err := os.MkdirAll(filepath.Dir(settings.RegistryConfig), 0755); err != nil {
		return nil, errors.Wrap(err, "create registry config dir")
	}
	// initializes the action configuration
	if err := cfg.Init(settings.RESTClientGetter(), settings.Namespace(), helmDriver, func(format string, v ...interface{}) {
		logrus.Debugf(format, v)
	}); err != nil {
		return nil, errors.Wrap(err, "init config")
	}
	registryClient, err := registry.NewClient(
		registry.ClientOptDebug(settings.Debug),
		registry.ClientOptCredentialsFile(settings.RegistryConfig),
	)
	if err != nil {
		return nil, errors.Wrap(err, "init registry client")
	}
	cfg.RegistryClient = registryClient
	return &Helm{
		cfg:       cfg,
		settings:  settings,
		namespace: namespace,
		repoFile:  repoFile,
		repoCache: repoCache,
	}, nil
}

// UpdateRepo -
func (h *Helm) UpdateRepo(names string) error {
	return h.repoUpdate(names, ioutil.Discard)
}

// PreInstall -
func (h *Helm) PreInstall(name, chart, version string) error {
	_, err := h.install(name, chart, version, "", nil, true, ioutil.Discard)
	return err
}

// Install -
func (h *Helm) Install(chartPath, name, chart, version string, overrides []string) (*release.Release, error) {
	release, err := h.install(name, chart, version, chartPath, overrides, true, ioutil.Discard)
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

func (h *Helm) install(name, chart, version, chartPath string, overrides []string, dryRun bool, out io.Writer) (*release.Release, error) {
	client := action.NewInstall(h.cfg)
	client.ReleaseName = name
	client.Namespace = h.namespace
	client.Version = version
	client.DryRun = dryRun
	//client.IsUpgrade = true
	client.ClientOnly = true

	// 跳过 Kubernetes 版本检查，解决版本不匹配问题
	client.SkipCRDs = false
	client.DisableHooks = false
	client.DisableOpenAPIValidation = true

	var cp string
	if chartPath != "" {
		cp = chartPath
	} else {
		res, err := h.locateChart(chart, version)
		if err != nil {
			return nil, err
		}
		cp = res
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

	// 强制移除 chart 的 Kubernetes 版本要求，解决版本不匹配问题
	if chartRequested.Metadata != nil && chartRequested.Metadata.KubeVersion != "" {
		logrus.Infof("Removing chart kubeVersion requirement: %s", chartRequested.Metadata.KubeVersion)
		chartRequested.Metadata.KubeVersion = ""
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
	RegistryClient *registry.Client
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
		RegistryClient:   c.RegistryClient,
	}
	if c.RegistryClient != nil {
		dl.Options = append(dl.Options, getter.WithRegistryClient(c.RegistryClient))
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

func parseValuesYAML(valuesYAML string) (map[string]interface{}, error) {
	vals := map[string]interface{}{}
	if valuesYAML == "" {
		return vals, nil
	}
	if err := yaml.Unmarshal([]byte(valuesYAML), &vals); err != nil {
		return nil, fmt.Errorf("invalid values YAML: %v", err)
	}
	return vals, nil
}

func (h *Helm) newInstallAction(releaseName, version string) *action.Install {
	client := action.NewInstall(h.cfg)
	client.ReleaseName = releaseName
	client.Namespace = h.namespace
	client.Version = version
	client.DryRun = false
	client.ClientOnly = false
	return client
}

func (h *Helm) installLoadedChart(chartPath, releaseName, version, valuesYAML string) (*release.Release, error) {
	vals, err := parseValuesYAML(valuesYAML)
	if err != nil {
		return nil, err
	}
	chartLoaded, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("load chart: %v", err)
	}
	removeKubeVersionFromChart(chartLoaded)
	client := h.newInstallAction(releaseName, version)
	return client.Run(chartLoaded, vals)
}

func (h *Helm) loginRegistryIfNeeded(chartRef, username, password string) error {
	if h.cfg.RegistryClient == nil || username == "" {
		return nil
	}
	ref, err := url.Parse(chartRef)
	if err != nil {
		return err
	}
	if ref.Scheme != registry.OCIScheme {
		return nil
	}
	return h.cfg.RegistryClient.Login(ref.Host, registry.LoginOptBasicAuth(username, password))
}

func normalizeOCIChartReference(chartRef, version string) (string, string) {
	if version != "" || !strings.HasPrefix(chartRef, "oci://") {
		return chartRef, version
	}
	caps := ociRefTagRegexp.FindStringSubmatch(chartRef)
	if len(caps) != 4 {
		return chartRef, version
	}
	return caps[1], caps[3]
}

func (h *Helm) resolveChartPath(chartRef, repoURL, version, username, password string) (string, string, error) {
	chartRef, version = normalizeOCIChartReference(chartRef, version)
	if err := h.loginRegistryIfNeeded(chartRef, username, password); err != nil {
		return "", "", fmt.Errorf("login registry %s: %v", chartRef, err)
	}
	client := h.newInstallAction("", version)
	cpo := &ChartPathOptions{
		ChartPathOptions: client.ChartPathOptions,
		RegistryClient:   h.cfg.RegistryClient,
	}
	cpo.RepoURL = repoURL
	cpo.Version = version
	cpo.Username = username
	cpo.Password = password
	cp, err := cpo.LocateChart(chartRef, h.settings.RepositoryCache, h.settings)
	if err != nil {
		return "", "", fmt.Errorf("locate chart %s: %v", chartRef, err)
	}
	return cp, version, nil
}

// InstallFromReference installs a chart from a repo name, repo URL, direct chart URL or OCI reference.
func (h *Helm) InstallFromReference(chartRef, repoURL, version, releaseName, valuesYAML, username, password string) (*release.Release, error) {
	cp, resolvedVersion, err := h.resolveChartPath(chartRef, repoURL, version, username, password)
	if err != nil {
		return nil, err
	}
	return h.installLoadedChart(cp, releaseName, resolvedVersion, valuesYAML)
}

// InstallFromChartPath installs a chart from a local directory or archive path.
func (h *Helm) InstallFromChartPath(chartPath, version, releaseName, valuesYAML string) (*release.Release, error) {
	return h.installLoadedChart(chartPath, releaseName, version, valuesYAML)
}

// LoadChartFromReference resolves a chart reference and loads chart metadata without installing it.
func (h *Helm) LoadChartFromReference(chartRef, repoURL, version, username, password string) (*chart.Chart, string, string, error) {
	cp, resolvedVersion, err := h.resolveChartPath(chartRef, repoURL, version, username, password)
	if err != nil {
		return nil, "", "", err
	}
	chartLoaded, err := loader.Load(cp)
	if err != nil {
		return nil, "", "", fmt.Errorf("load chart: %v", err)
	}
	return chartLoaded, cp, resolvedVersion, nil
}

// LoadChartFromPath loads a chart from a local directory or archive path without installing it.
func (h *Helm) LoadChartFromPath(chartPath string) (*chart.Chart, error) {
	chartLoaded, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("load chart: %v", err)
	}
	return chartLoaded, nil
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

// removeKubeVersionFromChart 递归移除 chart 及其所有子 chart 的 Kubernetes 版本要求
func removeKubeVersionFromChart(ch *chart.Chart) {
	if ch.Metadata != nil && ch.Metadata.KubeVersion != "" {
		logrus.Infof("Removing kubeVersion requirement from chart %s: %s", ch.Name(), ch.Metadata.KubeVersion)
		ch.Metadata.KubeVersion = ""
	}

	// 递归处理所有子 chart
	for _, subChart := range ch.Dependencies() {
		removeKubeVersionFromChart(subChart)
	}
}

// ListReleases returns all Helm releases in the current namespace.
func (h *Helm) ListReleases() ([]*release.Release, error) {
	client := action.NewList(h.cfg)
	client.All = true
	return client.Run()
}

// InstallFromRepo installs a chart from a configured Helm repo directly into the cluster.
//
// IMPORTANT: This method intentionally does NOT delegate to the private install() helper,
// because that helper unconditionally sets ClientOnly=true (line ~181), which causes Helm
// to only render manifests without actually deploying to the cluster.
// This method calls action.NewInstall directly with DryRun=false, ClientOnly=false.
func (h *Helm) InstallFromRepo(repoName, chart, version, releaseName, valuesYAML string) (*release.Release, error) {
	chartRef := fmt.Sprintf("%s/%s", repoName, chart)
	return h.InstallFromReference(chartRef, "", version, releaseName, valuesYAML, "", "")
}
