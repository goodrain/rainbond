package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	"github.com/goodrain/rainbond/pkg/helm"
	rutil "github.com/goodrain/rainbond/util"
	hrepo "github.com/helm/helm/pkg/repo"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/chart/loader"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sigs.k8s.io/yaml"
	"strings"
)

// AppTemplate -
type AppTemplate struct {
	Name     string
	Versions hrepo.ChartVersions
}

// HelmAction -
type HelmAction struct {
	ctx            context.Context
	kubeClient     *kubernetes.Clientset
	rainbondClient versioned.Interface
	repo           *helm.Repo
	config         *rest.Config
	mapper         meta.RESTMapper
}

// CreateHelmManager 创建 helm 客户端
func CreateHelmManager(clientset *kubernetes.Clientset, rainbondClient versioned.Interface, config *rest.Config, mapper meta.RESTMapper) *HelmAction {
	repo := helm.NewRepo(repoFile, repoCache)
	return &HelmAction{
		kubeClient:     clientset,
		rainbondClient: rainbondClient,
		ctx:            context.Background(),
		repo:           repo,
		config:         config,
		mapper:         mapper,
	}
}

var (
	dataDir   = "/grdata/helm"
	repoFile  = path.Join(dataDir, "repo/repositories.yaml")
	repoCache = path.Join(dataDir, "cache")
)

// GetChartInformation 获取 helm 应用 chart 包的详细版本信息
func (h *HelmAction) GetChartInformation(chart api_model.ChartInformation) (*[]api_model.HelmChartInformation, *util.APIHandleError) {
	req, err := http.NewRequest("GET", chart.RepoURL+"/index.yaml", nil)
	if err != nil {
		return nil, &util.APIHandleError{Code: 400, Err: errors.Wrap(err, "GetChartInformation NewRequest")}
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, &util.APIHandleError{Code: 400, Err: errors.Wrap(err, "GetChartInformation client.Do")}
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, &util.APIHandleError{Code: 400, Err: errors.Wrap(err, "GetChartInformation ioutil.ReadAll")}
	}
	jbody, err := yaml.YAMLToJSON(body)
	if err != nil {
		return nil, &util.APIHandleError{Code: 400, Err: errors.Wrap(err, "GetChartInformation yaml.YAMLToJSON")}
	}
	var indexFile hrepo.IndexFile
	if err := json.Unmarshal(jbody, &indexFile); err != nil {
		logrus.Errorf("json.Unmarshal: %v", err)
		return nil, &util.APIHandleError{Code: 400, Err: errors.Wrap(err, "GetChartInformation json.Unmarshal")}
	}
	if len(indexFile.Entries) == 0 {
		return nil, &util.APIHandleError{Code: 400, Err: fmt.Errorf("entries not found")}
	}
	var chartInformations []api_model.HelmChartInformation
	if chart, ok := indexFile.Entries[chart.ChartName]; ok {
		for _, version := range chart {
			v := version
			chartInformations = append(chartInformations, api_model.HelmChartInformation{
				Version:  v.Version,
				Keywords: v.Keywords,
				Pic:      v.Icon,
				Abstract: v.Description,
			})
		}
	}
	return &chartInformations, nil
}

// CheckHelmApp check helm app
func (h *HelmAction) CheckHelmApp(checkHelmApp api_model.CheckHelmApp) (string, error) {
	helmAppYaml, err := GetHelmAppYaml(checkHelmApp.Name, checkHelmApp.Chart, checkHelmApp.Version, checkHelmApp.Namespace, "", checkHelmApp.Overrides)
	if err != nil {
		return "", errors.Wrap(err, "helm app check failed")
	}
	return helmAppYaml, nil
}

// UpdateHelmRepo update repo
func (h *HelmAction) UpdateHelmRepo(names string) error {
	err := UpdateRepo(names)
	if err != nil {
		return errors.Wrap(err, "helm repo update failed")
	}
	return nil
}

// AddHelmRepo add helm repo
func (h *HelmAction) AddHelmRepo(helmRepo api_model.CheckHelmApp) error {
	err := h.repo.Add(helmRepo.RepoName, helmRepo.RepoURL, helmRepo.Username, helmRepo.Password)
	if err != nil {
		logrus.Errorf("add helm repo err: %v", err)
		return err
	}
	return nil
}

// GetHelmAppYaml get helm app yaml
func GetHelmAppYaml(name, chart, version, namespace, chartPath string, overrides []string) (string, error) {
	helmCmd, err := helm.NewHelm(namespace, repoFile, repoCache)
	if err != nil {
		logrus.Errorf("Failed to create help client：%v", err)
		return "", err
	}
	release, err := helmCmd.Install(chartPath, name, chart, version, overrides)
	if err != nil {
		logrus.Errorf("helm --dry-run install failure: %v", err)
		return "", err
	}
	return release.Manifest, nil
}

// UpdateRepo Update Helm warehouse
func UpdateRepo(names string) error {
	helmCmd, err := helm.NewHelm("", repoFile, repoCache)
	if err != nil {
		logrus.Errorf("Failed to create helm client：%v", err)
		return err
	}
	err = helmCmd.UpdateRepo(names)
	if err != nil {
		logrus.Errorf("helm update failure: %v", err)
		return err
	}
	return nil
}

// GetYamlByChart get yaml by chart
func (h *HelmAction) GetYamlByChart(chartPath, namespace, name, version string, overrides []string) (string, error) {
	helmAppYaml, err := GetHelmAppYaml(name, "", version, namespace, chartPath, overrides)
	if err != nil {
		return "", errors.Wrap(err, "helm app check failed")
	}
	return helmAppYaml, nil
}

// GetUploadChartInformation -
func (h *HelmAction) GetUploadChartInformation(eventID string) ([]api_model.HelmChartInformation, error) {
	basePath := path.Join("/grdata/package_build/temp/events", eventID)
	files, err := filepath.Glob(path.Join(basePath, "*"))
	if err != nil {
		return nil, err
	}
	if len(files) != 1 {
		return nil, fmt.Errorf("number of files is incorrect, make sure there is only one compressed package")
	}
	if strings.HasSuffix(files[0], ".tgz") {
		err = rutil.UnTar(files[0], basePath, true)
		if err != nil {
			return nil, err
		}
		err = os.RemoveAll(files[0])
		if err != nil {
			return nil, err
		}
	}
	files, err = filepath.Glob(path.Join(basePath, "*"))
	if err != nil {
		return nil, err
	}
	if len(files) != 1 {
		return nil, fmt.Errorf("number of files is incorrect, make sure there is only one dir")
	}
	chartPath := files[0]
	s, err := os.Stat(chartPath)
	if err != nil {
		return nil, err
	}
	if !s.IsDir() {
		return nil, fmt.Errorf("upload file not is tgz")
	}
	chart, err := loader.Load(chartPath)
	if err != nil {
		logrus.Errorf("load upload helm chart failure: %v", err)
		return nil, errors.Wrap(err, "load upload helm chart failure")
	}
	var chartInformation []api_model.HelmChartInformation
	if chart != nil && chart.Metadata != nil {
		chartInformation = append(chartInformation, api_model.HelmChartInformation{
			Name:     chart.Metadata.Name,
			Version:  chart.Metadata.Version,
			Keywords: chart.Metadata.Keywords,
			Pic:      chart.Metadata.Icon,
			Abstract: chart.Metadata.Description,
		})
	}
	return chartInformation, nil
}

// CheckUploadChart -
func (h *HelmAction) CheckUploadChart(name, version, namespace, eventID string) error {
	basePath := path.Join("/grdata/package_build/temp/events", eventID)
	files, err := filepath.Glob(path.Join(basePath, "*"))
	if err != nil {
		return err
	}
	helmCmd, err := helm.NewHelm(namespace, repoFile, repoCache)
	if err != nil {
		return err
	}
	chartPath := files[0]
	_, err = helmCmd.Install(chartPath, name, "", version, []string{})
	if err != nil {
		return err
	}
	return nil
}

// GetUploadChartResource -
func (h *HelmAction) GetUploadChartResource(name, version, namespace, eventID string, overrides []string) (interface{}, error) {
	basePath := path.Join("/grdata/package_build/temp/events", eventID)
	files, err := filepath.Glob(path.Join(basePath, "*"))
	if err != nil {
		return nil, err
	}
	helmCmd, err := helm.NewHelm(namespace, repoFile, repoCache)
	if err != nil {
		return nil, err
	}
	chartPath := files[0]
	release, err := helmCmd.Install(chartPath, name, "", version, overrides)
	if err != nil {
		return nil, err
	}
	chartBuildResourceList := handleFileORYamlToObject("upload-chart", []byte(release.Manifest), h.config)
	appResource := HandleDetailResource(namespace, chartBuildResourceList, false, h.kubeClient, h.mapper)
	return appResource, nil
}

// GetUploadChartValue -
func (h *HelmAction) GetUploadChartValue(eventID string) (*api_model.UploadChartValueYaml, error) {
	basePath := path.Join("/grdata/package_build/temp/events", eventID)
	files, err := filepath.Glob(path.Join(basePath, "*"))
	if err != nil {
		return nil, err
	}
	valuePath := path.Join(files[0], "values.yaml")
	valueYaml, err := os.ReadFile(valuePath)
	if err != nil {
		return nil, err
	}
	readmePath := path.Join(files[0], "README.md")
	readmeYaml, err := os.ReadFile(readmePath)
	if err != nil {
		return &api_model.UploadChartValueYaml{
			Values: map[string]string{"value.yaml": base64.StdEncoding.EncodeToString(valueYaml)},
			Readme: "",
		}, nil
	}
	return &api_model.UploadChartValueYaml{
		Values: map[string]string{"values.yaml": base64.StdEncoding.EncodeToString(valueYaml)},
		Readme: base64.StdEncoding.EncodeToString(readmeYaml),
	}, nil
}
