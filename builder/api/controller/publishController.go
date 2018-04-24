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
	"github.com/Sirupsen/logrus"
	"net/http"
	"github.com/goodrain/rainbond/builder/model"
	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/go-chi/chi"
	"strings"
	"io/ioutil"
	"encoding/json"
)

func GetAppPublish(w http.ResponseWriter, r *http.Request) {
	serviceKey := strings.TrimSpace(chi.URLParam(r, "serviceKey"))
	appVersion := strings.TrimSpace(chi.URLParam(r, "appVersion"))
	appp,err:=db.GetManager().AppPublishDao().GetAppPublish(serviceKey,appVersion)
	if err != nil {
		httputil.ReturnError(r,w,404,err.Error())
	}
	httputil.ReturnSuccess(r, w, appp)
}


func AddAppPublish(w http.ResponseWriter, r *http.Request) {
	var result model.AppPublish
	b,err:=ioutil.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("error get request body ,details %s",err.Error())
		httputil.ReturnError(r,w,404,err.Error())
		return
	}
	defer r.Body.Close()
	logrus.Infof("request body is %s",b)
	err=json.Unmarshal(b,&result)
	if err != nil {
		logrus.Errorf("error unmarshal use raw support,details %s",err.Error())
		httputil.ReturnError(r,w,400,err.Error())
		return
	}

	dbmodel:=convertPublishToDB(&result)
	//checkAndGet
	err=db.GetManager().AppPublishDao().AddModel(dbmodel)
	if err!=nil {
		logrus.Errorf("error save publish record to db,details %s",err.Error())
		httputil.ReturnError(r,w,500,err.Error())
	}
	httputil.ReturnSuccess(r, w, nil)
}
func convertPublishToDB(publish *model.AppPublish) *dbmodel.AppPublish {

	var dbm dbmodel.AppPublish
	dbm.ShareID=publish.ShareID
	dbm.AppVersion=publish.AppVersion
	dbm.DestYB=publish.DestYB
	dbm.DestYS=publish.DestYS
	dbm.Image=publish.Image
	dbm.ServiceKey=publish.ServiceKey
	dbm.Slug=publish.Slug
	dbm.Status=publish.Status
	return &dbm
}
