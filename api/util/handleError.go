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

package util

import (
	"fmt"
	"net/http"
	"strings"

	httputil "github.com/goodrain/rainbond/util/http"

	"github.com/jinzhu/gorm"

	"github.com/Sirupsen/logrus"
)

//APIHandleError handle create err for api
type APIHandleError struct {
	Code int
	Err  error
	Data interface{}
}

//CreateAPIHandleError create APIHandleError
func CreateAPIHandleError(code int, err error) *APIHandleError {
	return &APIHandleError{
		Code: code,
		Err:  err,
	}
}

// CreateAPIHandleErrorV2 creates APIHandleError
// Support setting data
func CreateAPIHandleErrorV2(code int, err error, data interface{}) *APIHandleError {
	return &APIHandleError{
		Code: code,
		Err:  err,
		Data: data,
	}
}

//CreateAPIHandleErrorf create handle error
func CreateAPIHandleErrorf(code int, format string, args ...interface{}) *APIHandleError {
	return &APIHandleError{
		Code: code,
		Err:  fmt.Errorf(format, args...),
	}
}

//CreateAPIHandleErrorFromDBError from db error create APIHandleError
func CreateAPIHandleErrorFromDBError(msg string, err error) *APIHandleError {
	if err.Error() == gorm.ErrRecordNotFound.Error() {
		return &APIHandleError{
			Code: 404,
			Err:  fmt.Errorf("%s:%s", msg, err.Error()),
		}
	}
	if strings.HasSuffix(strings.TrimRight(err.Error(), " "), "is exist") {
		return &APIHandleError{
			Code: 400,
			Err:  fmt.Errorf("%s:%s", msg, err.Error()),
		}
	}
	return &APIHandleError{
		Code: 500,
		Err:  fmt.Errorf("%s:%s", msg, err.Error()),
	}
}
func (a *APIHandleError) Error() string {
	return a.Err.Error()
}

func (a *APIHandleError) String() string {
	return fmt.Sprintf("(Code:%d) %s", a.Code, a.Err.Error())
}

//Handle 处理
func (a *APIHandleError) Handle(r *http.Request, w http.ResponseWriter) {
	if a.Code >= 500 {
		logrus.Error(a.String())
		httputil.ReturnError(r, w, a.Code, a.Error())
		return
	}

	if a.Data != nil {
		httputil.Return(r, w, a.Code, httputil.ResponseBody{
			Msg:  a.Error(),
			Bean: a.Data,
		})
		return
	}

	httputil.ReturnError(r, w, a.Code, a.Error())
	return
}
