package handler

import (
	"context"
	"encoding/json"
	"fmt"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	"github.com/goodrain/rainbond/pkg/generated/clientset/versioned"
	"github.com/goodrain/rainbond/pkg/helm"
	hrepo "github.com/helm/helm/pkg/repo"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"net/http"
	"path"
	"sigs.k8s.io/yaml"
)

//AppTemplate -
type AppTemplate struct {
	Name     string
	Versions hrepo.ChartVersions
}

//HelmAction -
type HelmAction struct {
	ctx            context.Context
	kubeClient     *kubernetes.Clientset
	rainbondClient versioned.Interface
	repo           *helm.Repo
}

// CreateHelmManager 创建 helm 客户端
func CreateHelmManager(clientset *kubernetes.Clientset, rainbondClient versioned.Interface) *HelmAction {
	repo := helm.NewRepo(repoFile, repoCache)
	return &HelmAction{
		kubeClient:     clientset,
		rainbondClient: rainbondClient,
		ctx:            context.Background(),
		repo:           repo,
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
	helmAppYaml, err := GetHelmAppYaml(checkHelmApp.Name, checkHelmApp.Chart, checkHelmApp.Version, checkHelmApp.Namespace, checkHelmApp.Overrides)
	if err != nil {
		return "", errors.Wrap(err, "helm app check failed")
	}
	return helmAppYaml, nil
}

//UpdateHelmRepo update repo
func (h *HelmAction) UpdateHelmRepo(names string) error {
	err := UpdateRepo(names)
	if err != nil {
		return errors.Wrap(err, "helm repo update failed")
	}
	return nil
}

//AddHelmRepo add helm repo
func (h *HelmAction) AddHelmRepo(helmRepo api_model.CheckHelmApp) error {
	err := h.repo.Add(helmRepo.RepoName, helmRepo.RepoURL, helmRepo.Username, helmRepo.Password)
	if err != nil {
		logrus.Errorf("add helm repo err: %v", err)
		return err
	}
	return nil
}

//GetHelmAppYaml get helm app yaml
func GetHelmAppYaml(name, chart, version, namespace string, overrides []string) (string, error) {
	logrus.Info("get into GetHelmAppYaml function")
	helmCmd, err := helm.NewHelm(namespace, repoFile, repoCache)
	if err != nil {
		logrus.Errorf("Failed to create help client：%v", err)
		return "", err
	}
	release, err := helmCmd.Install(name, chart, version, overrides)
	if err != nil {
		logrus.Errorf("helm --dry-run install failure: %v", err)
		return "", err
	}
	return release.Manifest, nil
}

//UpdateRepo Update Helm warehouse
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
