// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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
	"github.com/goodrain/rainbond/builder/model"
	"encoding/json"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	httputil "github.com/goodrain/rainbond/util/http"

	"github.com/Sirupsen/logrus"
	"github.com/bitly/go-simplejson"
	"io/ioutil"
	"github.com/goodrain/rainbond/builder/discover"
)

func AddCodeCheck(w http.ResponseWriter, r *http.Request) {
	//b,_:=ioutil.ReadAll(r.Body)
	//{\"url_repos\": \"https://github.com/bay1ts/zk_cluster_mini.git\", \"check_type\": \"first_check\", \"code_from\": \"gitlab_manual\", \"service_id\": \"c24dea8300b9401b1461dd975768881a\", \"code_version\": \"master\", \"git_project_id\": 0, \"condition\": \"{\\\"language\\\":\\\"docker\\\",\\\"runtimes\\\":\\\"false\\\", \\\"dependencies\\\":\\\"false\\\",\\\"procfile\\\":\\\"false\\\"}\", \"git_url\": \"--branch master --depth 1 https://github.com/bay1ts/zk_cluster_mini.git\"}
	//logrus.Infof("request recive %s",string(b))
	result := new(model.CodeCheckResult)

	b,_:=ioutil.ReadAll(r.Body)
	j,err:=simplejson.NewJson(b)
	if err != nil {
		logrus.Errorf("error decode json,details %s",err.Error())
		httputil.ReturnError(r,w,400,"bad request")
		return
	}
	result.URLRepos,_=j.Get("url_repos").String()
	result.CheckType,_=j.Get("check_type").String()
	result.CodeFrom,_=j.Get("code_from").String()
	result.ServiceID,_=j.Get("service_id").String()
	result.CodeVersion,_=j.Get("code_version").String()
	result.GitProjectId,_=j.Get("git_project_id").String()
	result.Condition,_=j.Get("condition").String()
	result.GitURL,_=j.Get("git_url").String()

	defer r.Body.Close()


	dbmodel:=convertModelToDB(result)
	//checkAndGet
	db.GetManager().CodeCheckResultDao().AddModel(dbmodel)
	httputil.ReturnSuccess(r, w, nil)
}
func Update(w http.ResponseWriter, r *http.Request) {
	serviceID := strings.TrimSpace(chi.URLParam(r, "serviceID"))
	result := new(model.CodeCheckResult)

	b,_:=ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	logrus.Infof("update receive %s",string(b))
	j,err:=simplejson.NewJson(b)
	if err != nil {
		logrus.Errorf("error decode json,details %s",err.Error())
		httputil.ReturnError(r,w,400,"bad request")
		return
	}
	result.BuildImageName,_=j.Get("image").String()
	portList,err:=j.Get("port_list").Map()
	if err != nil {
		portList=make(map[string]interface{})
	}
	volumeList,err:=j.Get("volume_list").StringArray()
	if err != nil {
		volumeList=nil
	}
	strMap:=make(map[string]string)
	for k,v:=range portList {
		strMap[k]=v.(string)
	}
	result.VolumeList=volumeList
	result.PortList=strMap
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

	if result.Condition != "" {
		bs:=[]byte(result.Condition)
		l,err:=simplejson.NewJson(bs)
		if err != nil {
			logrus.Errorf("error get condition,details %s",err.Error())
		}
		language,err:=l.Get("language").String()
		if err != nil {
			logrus.Errorf("error get language,details %s",err.Error())
		}
		r.Language=language
	}
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
		logrus.Errorf("error get check result,details %s",err.Error())
		httputil.ReturnError(r,w,500,err.Error())
		return
	}
	httputil.ReturnSuccess(r,w,cr)
}

func CheckHalth(w http.ResponseWriter, r *http.Request)  {
      healthInfo := discover.HealthCheck()
      if healthInfo["status"] != "health"{
      	httputil.ReturnError(r,w,400,"builder service unusual")
	  }
      httputil.ReturnSuccess(r,w,healthInfo)
}
