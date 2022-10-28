package controller

import (
	"errors"
	"fmt"
	"github.com/goodrain/rainbond/api/handler"
	api_model "github.com/goodrain/rainbond/api/model"
	httputil "github.com/goodrain/rainbond/util/http"
	"net/http"
)

type HelmStruct struct {
}

//CommandHelm execute helm command
func (t *HelmStruct) CommandHelm(w http.ResponseWriter, r *http.Request) {
	var cmd api_model.CommandHelmStruct
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &cmd, nil); !ok {
		return
	}
	helmCommandRet, err := handler.GetHelmManager().CommandHelm(cmd.Command)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, helmCommandRet)
}

//CheckHelmApp check helm app
func (t *HelmStruct) CheckHelmApp(w http.ResponseWriter, r *http.Request) {
	var checkHelmApp api_model.CheckHelmApp
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &checkHelmApp, nil); !ok {
		return
	}
	data := map[string]string{"checkAdopt": "true"}
	err := handler.GetHelmManager().AddHelmRepo(checkHelmApp)
	if err != nil && !errors.Is(err, fmt.Errorf("repository templateName (%s) already exists, please specify a different templateName", checkHelmApp.RepoName)) {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	yaml, err := handler.GetHelmManager().CheckHelmApp(checkHelmApp)
	if err != nil {
		data["checkAdopt"] = "false"
	}
	data["yaml"] = yaml
	httputil.ReturnSuccess(r, w, data)
}

//GetChartInformation get helm chart details
func (t *HelmStruct) GetChartInformation(w http.ResponseWriter, r *http.Request) {
	var chart api_model.ChartInformation
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &chart, nil); !ok {
		return
	}
	chartVersion, err := handler.GetHelmManager().GetChartInformation(chart)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, chartVersion)
}
