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
	"encoding/json"
	"github.com/goodrain/rainbond/pkg/db"
	dbmodel "github.com/goodrain/rainbond/pkg/db/model"
	httputil "github.com/goodrain/rainbond/pkg/util/http"
	"github.com/go-chi/chi"
	"strings"
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
	result := new(model.AppPublish)
	decoder := json.NewDecoder(r.Body)
	defer r.Body.Close()
	err := decoder.Decode(result)
	if err != nil {
		logrus.Errorf("error decode json,details %s",err.Error())
		httputil.ReturnError(r,w,400,"bad request")
		return
	}
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
