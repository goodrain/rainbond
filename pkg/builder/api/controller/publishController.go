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
	"github.com/Sirupsen/logrus"
	"net/http"
	"github.com/goodrain/rainbond/pkg/builder/model"
	"github.com/goodrain/rainbond/pkg/db"
	dbmodel "github.com/goodrain/rainbond/pkg/db/model"
	httputil "github.com/goodrain/rainbond/pkg/util/http"
	validator "github.com/thedevsaddam/govalidator"
	"github.com/go-chi/chi"
	"strings"
	"github.com/bitly/go-simplejson"
	"io/ioutil"
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

func UpdateDeliveredPath(w http.ResponseWriter, r *http.Request) {
	rules := validator.MapData{
		"type": []string{"required"},
		"event_id": []string{"required"},
		"path": []string{"required"},
	}
	data, ok := httputil.ValidatorRequestMapAndErrorResponse(r, w, rules, nil)
	if !ok {
		return
	}
	deliveredType:=data["type"].(string)
	eventID:=data["event_id"].(string)
	deliveredPath:=data["path"].(string)
	version,err:=db.GetManager().VersionInfoDao().GetVersionByEventID(eventID)
	if err != nil {
		httputil.ReturnError(r,w,404,err.Error())
	}
	version.DeliveredType=deliveredType
	version.DeliveredPath=deliveredPath
	db.GetManager().VersionInfoDao().UpdateModel(version)
	httputil.ReturnSuccess(r, w, nil)
}




func AddAppPublish(w http.ResponseWriter, r *http.Request) {
	result := new(model.AppPublish)


	b,_:=ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	j,err:=simplejson.NewJson(b)
	if err != nil {
		logrus.Errorf("error decode json,details %s",err.Error())
		httputil.ReturnError(r,w,400,"bad request")
		return
	}

	result.AppVersion,_=j.Get("app_version").String()
	result.ServiceKey,_=j.Get("service_key").String()
	result.Slug,_=j.Get("slug").String()
	result.Image,_=j.Get("image").String()
	result.DestYS,_=j.Get("dest_ys").Bool()
	result.DestYB,_=j.Get("dest_yb").Bool()
	result.ShareID,_=j.Get("share_id").String()


	dbmodel:=convertPublishToDB(result)
	//checkAndGet
	db.GetManager().AppPublishDao().AddModel(dbmodel)
	httputil.ReturnSuccess(r, w, nil)
}
func convertPublishToDB(publish *model.AppPublish) *dbmodel.AppPublish {

	dbm:=dbmodel.AppPublish{}
	dbm.ShareID=publish.ShareID
	dbm.AppVersion=publish.AppVersion
	dbm.DestYB=publish.DestYB
	dbm.DestYS=publish.DestYS
	dbm.Image=publish.Image
	dbm.ServiceKey=publish.ServiceKey
	dbm.Slug=publish.Slug
	return &dbm
}
