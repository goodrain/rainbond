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
	"github.com/jinzhu/gorm"

	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/handler/group"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	httputil "github.com/goodrain/rainbond/util/http"
)

//Backups list all backup history by group app
func Backups(w http.ResponseWriter, r *http.Request) {
	groupID := r.FormValue("group_id")
	if groupID == "" {
		httputil.ReturnError(r, w, 400, "group id can not be empty")
		return
	}
	list, err := handler.GetAPPBackupHandler().GetBackupByGroupID(groupID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, list)
}

//NewBackups new group app backup
func NewBackups(w http.ResponseWriter, r *http.Request) {
	var gb group.Backup
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &gb.Body, nil)
	if !ok {
		return
	}
	bean, err := handler.GetAPPBackupHandler().NewBackup(gb)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, bean)
}

//BackupCopy backup copy
func BackupCopy(w http.ResponseWriter, r *http.Request) {
	var gb group.BackupCopy
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &gb.Body, nil)
	if !ok {
		return
	}
	bean, err := handler.GetAPPBackupHandler().BackupCopy(gb)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, bean)
}

//Restore restore group app
func Restore(w http.ResponseWriter, r *http.Request) {
	var br group.BackupRestore
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &br.Body, nil)
	if !ok {
		return
	}
	br.BackupID = chi.URLParam(r, "backup_id")
	tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)
	br.Body.TenantID = tenantID
	bean, err := handler.GetAPPBackupHandler().RestoreBackup(br)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, bean)
}

//RestoreResult restore group app result
func RestoreResult(w http.ResponseWriter, r *http.Request) {
	restoreID := chi.URLParam(r, "restore_id")
	bean, err := handler.GetAPPBackupHandler().RestoreBackupResult(restoreID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, bean)
}

//GetBackup get one backup status
func GetBackup(w http.ResponseWriter, r *http.Request) {
	backupID := chi.URLParam(r, "backup_id")
	bean, err := handler.GetAPPBackupHandler().GetBackup(backupID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, bean)
}

//DeleteBackup delete backup
func DeleteBackup(w http.ResponseWriter, r *http.Request) {
	backupID := chi.URLParam(r, "backup_id")

	err := handler.GetAPPBackupHandler().DeleteBackup(backupID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			httputil.ReturnError(r, w, 404, "not found")
			return
		}
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}
