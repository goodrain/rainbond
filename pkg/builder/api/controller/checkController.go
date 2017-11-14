package controller

import (
	"net/http"
	"github.com/go-chi/chi"
	"strings"
	"github.com/goodrain/rainbond/pkg/builder/model"
	"encoding/json"
	"github.com/goodrain/rainbond/pkg/db"
	dbmodel "github.com/goodrain/rainbond/pkg/db/model"
	httputil "github.com/goodrain/rainbond/pkg/util/http"

	"github.com/Sirupsen/logrus"
)

func AddCodeCheck(w http.ResponseWriter, r *http.Request) {
	result := new(model.CodeCheckResult)
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	err := decoder.Decode(result)
	if err != nil {
		logrus.Errorf("error decode json,details %s",err.Error())
		httputil.ReturnError(r,w,400,"bad request")
		return
	}
	dbmodel:=convertModelToDB(result)
	//checkAndGet
	db.GetManager().CodeCheckResultDao().AddModel(dbmodel)
	httputil.ReturnSuccess(r, w, nil)
}
func Update(w http.ResponseWriter, r *http.Request) {
	serviceID := strings.TrimSpace(chi.URLParam(r, "serviceID"))
	result := new(model.CodeCheckResult)
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	err := decoder.Decode(result)
	if err != nil {
		logrus.Errorf("error decode json,details %s",err.Error())
		httputil.ReturnError(r,w,400,"bad request")
		return
	}
	result.ServiceID=serviceID
	dbmodel:=convertModelToDB(result)
	dbmodel.DockerFileReady=true
	db.GetManager().CodeCheckResultDao().UpdateModel(dbmodel)
	httputil.ReturnSuccess(r, w, nil)
}
func convertModelToDB(result *model.CodeCheckResult) *dbmodel.CodeCheckResult {
	r:=dbmodel.CodeCheckResult{}
	r.ServiceID=result.ServiceID
	r.CheckType=result.CheckType
	r.CodeFrom=result.CodeFrom
	r.CodeVersion=result.CodeVersion
	r.Condition=result.Condition
	r.GitProjectId=result.GitProjectId
	r.GitURL=result.GitURL
	r.URLRepos=result.URLRepos

	r.BuildImageName=result.BuildImageName
	r.InnerPort=result.InnerPort
	pl,_:=json.Marshal(result.PortList)
	r.PortList=string(pl)
	vl,_:=json.Marshal(result.VolumeList)
	r.VolumeList=string(vl)
	r.VolumeMountPath=result.VolumeMountPath
	return &r
}
func GetCodeCheck(w http.ResponseWriter, r *http.Request) {
	serviceID := strings.TrimSpace(chi.URLParam(r, "serviceID"))
	//findResultByServiceID
	cr,err:=db.GetManager().CodeCheckResultDao().GetCodeCheckResult(serviceID)
	if err!=nil {

	}
	httputil.ReturnSuccess(r,w,cr)
}
