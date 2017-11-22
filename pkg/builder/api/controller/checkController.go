// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.
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
	"github.com/bitly/go-simplejson"
	"io/ioutil"
)

func AddCodeCheck(w http.ResponseWriter, r *http.Request) {
	b,_:=ioutil.ReadAll(r.Body)
	logrus.Infof("request recive %s",string(b))
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
	bs:=[]byte(result.Condition)
	l,err:=simplejson.NewJson(bs)
	if err != nil {

	}
	language,err:=l.Get("language").String()
	if err != nil {

	}
	r.Language=language
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
