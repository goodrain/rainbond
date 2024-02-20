package handler

import (
	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
)

// HelmHandler -
type HelmHandler interface {
	AddHelmRepo(helmRepo apimodel.CheckHelmApp) error
	CheckHelmApp(checkHelmApp apimodel.CheckHelmApp) (string, error)
	GetChartInformation(chart apimodel.ChartInformation) (*[]apimodel.HelmChartInformation, *util.APIHandleError)
	UpdateHelmRepo(names string) error
	GetYamlByChart(chartPath, namespace, name, version string, overrides []string) (string, error)
	GetUploadChartInformation(eventID string) ([]apimodel.HelmChartInformation, error)
	CheckUploadChart(name, version, namespace, eventID string) error
	GetUploadChartResource(name, version, namespace, eventID string, overrides []string) (interface{}, error)
	GetUploadChartValue(eventID string) (*apimodel.UploadChartValueYaml, error)
}
