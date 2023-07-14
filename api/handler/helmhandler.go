package handler

import (
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
)

//HelmHandler -
type HelmHandler interface {
	AddHelmRepo(helmRepo api_model.CheckHelmApp) error
	CheckHelmApp(checkHelmApp api_model.CheckHelmApp) (string, error)
	GetChartInformation(chart api_model.ChartInformation) (*[]api_model.HelmChartInformation, *util.APIHandleError)
	UpdateHelmRepo(names string) error
}
