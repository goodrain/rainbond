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

package utils

import (
	"fmt"
	httputil "github.com/goodrain/rainbond/util/http"
	"net/http"

	"github.com/sirupsen/logrus"
)

//APIHandleError handle create err for api
type APIHandleError struct {
	Code int
	Err  error
}

//CreateAPIHandleError create APIHandleError
func CreateAPIHandleError(code int, err error) *APIHandleError {
	return &APIHandleError{
		Code: code,
		Err:  err,
	}
}

//CreateAPIHandleErrorFromDBError from db error create APIHandleError
func CreateAPIHandleErrorFromDBError(msg string, err error) *APIHandleError {
	return &APIHandleError{
		Code: 500,
		Err:  fmt.Errorf("%s:%s", msg, err.Error()),
	}
}
func (a *APIHandleError) Error() string {
	return a.Err.Error()
}

func (a *APIHandleError) String() string {
	return fmt.Sprintf("%d:%s", a.Code, a.Err.Error())
}

//Handle 处理
func (a *APIHandleError) Handle(r *http.Request, w http.ResponseWriter) {
	if a.Code >= 500 {
		logrus.Error(a.String())
		httputil.ReturnError(r, w, a.Code, a.Error())
		return
	}
	httputil.ReturnError(r, w, a.Code, a.Error())
	return
}
