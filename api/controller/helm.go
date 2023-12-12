package controller

import (
	"errors"
	"fmt"
	"github.com/goodrain/rainbond/api/handler"
	api_model "github.com/goodrain/rainbond/api/model"
	httputil "github.com/goodrain/rainbond/util/http"
	"net/http"
)

// HelmStruct -
type HelmStruct struct {
}

// CheckHelmApp check helm app
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
	err = handler.GetHelmManager().UpdateHelmRepo(checkHelmApp.RepoName)
	if err != nil {
		data["checkAdopt"] = "false"
		data["yaml"] = err.Error()
	} else {
		yaml, err := handler.GetHelmManager().CheckHelmApp(checkHelmApp)
		data["yaml"] = yaml
		if err != nil {
			data["checkAdopt"] = "false"
			data["yaml"] = err.Error()
		}
	}
	httputil.ReturnSuccess(r, w, data)
}

// GetChartInformation get helm chart details
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

// GetYamlByChart -
func (t *HelmStruct) GetYamlByChart(w http.ResponseWriter, r *http.Request) {
	var yc api_model.GetYamlByChart
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &yc, nil); !ok {
		return
	}
	data := map[string]string{"checkAdopt": "true"}
	if yc.EventID == "" {
		httputil.ReturnError(r, w, 400, "Failed to parse eventID.")
		return
	}
	chartPath := fmt.Sprintf("%s/import/%s/%s", handler.GetAppHandler().GetStaticDir(), yc.EventID, yc.FileName)
	yaml, err := handler.GetHelmManager().GetYamlByChart(chartPath, yc.Namespace, yc.Name, yc.Version, []string{})
	if err != nil {
		data["checkAdopt"] = "false"
		data["yaml"] = err.Error()
	}
	data["yaml"] = yaml
	httputil.ReturnSuccess(r, w, data)
}

// GetUploadChartInformation -
func (t *HelmStruct) GetUploadChartInformation(w http.ResponseWriter, r *http.Request) {
	eventID := r.FormValue("event_id")
	data, err := handler.GetHelmManager().GetUploadChartInformation(eventID)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, data)
}

// CheckUploadChart -
func (t *HelmStruct) CheckUploadChart(w http.ResponseWriter, r *http.Request) {
	var cuc api_model.UploadChart
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &cuc, nil); !ok {
		return
	}
	data := map[string]string{"checkAdopt": "true"}
	err := handler.GetHelmManager().CheckUploadChart(cuc.Name, cuc.Version, cuc.Namespace, cuc.EventID)
	if err != nil {
		data["checkAdopt"] = "false"
		data["yaml"] = err.Error()
	}
	httputil.ReturnSuccess(r, w, data)
}

// GetUploadChartValue -
func (t *HelmStruct) GetUploadChartValue(w http.ResponseWriter, r *http.Request) {
	eventID := r.FormValue("event_id")
	valueYaml, err := handler.GetHelmManager().GetUploadChartValue(eventID)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, valueYaml)
}

// GetUploadChartResource -
func (t *HelmStruct) GetUploadChartResource(w http.ResponseWriter, r *http.Request) {
	var cuc api_model.UploadChart
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &cuc, nil); !ok {
		return
	}
	chartResources, err := handler.GetHelmManager().GetUploadChartResource(cuc.Name, cuc.Version, cuc.Namespace, cuc.EventID, cuc.Overrides)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, chartResources)
}

// ImportUploadChartResource -
func (t *HelmStruct) ImportUploadChartResource(w http.ResponseWriter, r *http.Request) {
	var uci api_model.UploadChartImport
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &uci, nil); !ok {
		return
	}
	ac, err := handler.GetClusterHandler().AppYamlResourceImport(uci.Namespace, uci.TenantID, uci.AppID, uci.AR)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, ac)
}
