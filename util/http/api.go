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

package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"sync"

	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/db"
	govalidator "github.com/goodrain/rainbond/util/govalidator"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

var validate *validator.Validate

func init() {
	var once sync.Once
	once.Do(func() {
		validate = validator.New()
	})
}

// ErrBadRequest -
type ErrBadRequest struct {
	err error
}

func (e ErrBadRequest) Error() string {
	return e.err.Error()
}

// Result represents a response for restful api.
type Result struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

//ValidatorStructRequest 验证请求数据
//data 传入指针
func ValidatorStructRequest(r *http.Request, data interface{}, message govalidator.MapData) url.Values {
	opts := govalidator.Options{
		Request: r,
		Data:    data,
	}
	if message != nil {
		opts.Messages = message
	}
	v := govalidator.New(opts)
	result := v.ValidateStructJSON()
	return result
}

//ValidatorMapRequest 验证请求数据从map
func ValidatorMapRequest(r *http.Request, rule govalidator.MapData, message govalidator.MapData) (map[string]interface{}, url.Values) {
	data := make(map[string]interface{}, 0)
	opts := govalidator.Options{
		Request: r,
		Data:    &data,
	}
	if rule != nil {
		opts.Rules = rule
	}
	if message != nil {
		opts.Messages = message
	}
	vd := govalidator.New(opts)
	e := vd.ValidateMapJSON()
	return data, e
}

//ValidatorRequestStructAndErrorResponse 验证并格式化请求数据为对象
// retrun true 继续执行
// return false 参数错误，终止
func ValidatorRequestStructAndErrorResponse(r *http.Request, w http.ResponseWriter, data interface{}, message govalidator.MapData) bool {
	if re := ValidatorStructRequest(r, data, message); len(re) > 0 {
		ReturnValidationError(r, w, re)
		return false
	}
	return true
}

//ValidatorRequestMapAndErrorResponse 验证并格式化请求数据为对象
// retrun true 继续执行
// return false 参数错误，终止
func ValidatorRequestMapAndErrorResponse(r *http.Request, w http.ResponseWriter, rule govalidator.MapData, messgae govalidator.MapData) (map[string]interface{}, bool) {
	data, re := ValidatorMapRequest(r, rule, messgae)
	if len(re) > 0 {
		ReturnValidationError(r, w, re)
		return nil, false
	}
	return data, true
}

//ResponseBody api返回数据格式
type ResponseBody struct {
	ValidationError url.Values  `json:"validation_error,omitempty"`
	Msg             string      `json:"msg,omitempty"`
	Bean            interface{} `json:"bean,omitempty"`
	List            interface{} `json:"list,omitempty"`
	//数据集总数
	ListAllNumber int `json:"number,omitempty"`
	//当前页码数
	Page int `json:"page,omitempty"`
}

//ParseResponseBody 解析成ResponseBody
func ParseResponseBody(red io.ReadCloser, dataType string) (re ResponseBody, err error) {
	if red == nil {
		err = errors.New("readcloser can not be nil")
		return
	}
	defer red.Close()
	switch render.GetContentType(dataType) {
	case render.ContentTypeJSON:
		err = render.DecodeJSON(red, &re)
	case render.ContentTypeXML:
		err = render.DecodeXML(red, &re)
	// case ContentTypeForm: // TODO
	default:
		err = errors.New("render: unable to automatically decode the request content type")
	}
	return
}

//ReturnValidationError 参数错误返回
func ReturnValidationError(r *http.Request, w http.ResponseWriter, err url.Values) {
	logrus.Debugf("validation error, uri: %s; msg: %v", r.RequestURI, ResponseBody{ValidationError: err})
	r = r.WithContext(context.WithValue(r.Context(), render.StatusCtxKey, http.StatusBadRequest))
	render.DefaultResponder(w, r, ResponseBody{ValidationError: err})
}

//ReturnSuccess 成功返回
func ReturnSuccess(r *http.Request, w http.ResponseWriter, datas interface{}) {
	r = r.WithContext(context.WithValue(r.Context(), render.StatusCtxKey, http.StatusOK))
	if datas == nil {
		render.DefaultResponder(w, r, ResponseBody{Bean: nil})
		return
	}
	v := reflect.ValueOf(datas)
	if v.Kind() == reflect.Slice {
		render.DefaultResponder(w, r, ResponseBody{List: datas})
		return
	}
	render.DefaultResponder(w, r, ResponseBody{Bean: datas})
	return
}

//ReturnList return list with page and count
func ReturnList(r *http.Request, w http.ResponseWriter, listAllNumber, page int, list interface{}) {
	r = r.WithContext(context.WithValue(r.Context(), render.StatusCtxKey, http.StatusOK))
	render.DefaultResponder(w, r, ResponseBody{List: list, ListAllNumber: listAllNumber, Page: page})
}

//ReturnError 返回错误信息
func ReturnError(r *http.Request, w http.ResponseWriter, code int, msg string) {
	logrus.Debugf("error code: %d; error uri: %s; error msg: %s", code, r.RequestURI, msg)
	r = r.WithContext(context.WithValue(r.Context(), render.StatusCtxKey, code))
	render.DefaultResponder(w, r, ResponseBody{Msg: msg})
}

//Return  自定义
func Return(r *http.Request, w http.ResponseWriter, code int, reb ResponseBody) {
	r = r.WithContext(context.WithValue(r.Context(), render.StatusCtxKey, code))
	render.DefaultResponder(w, r, reb)
}

//ReturnNoFomart  http return no format result
func ReturnNoFomart(r *http.Request, w http.ResponseWriter, code int, reb interface{}) {
	r = r.WithContext(context.WithValue(r.Context(), render.StatusCtxKey, code))
	render.DefaultResponder(w, r, reb)
}

//ReturnResNotEnough http return node resource not enough, http code = 412
func ReturnResNotEnough(r *http.Request, w http.ResponseWriter, eventID, msg string) {
	logrus.Debugf("resource not enough, msg: %s", msg)
	if err := db.GetManager().ServiceEventDao().UpdateReason(eventID, msg); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logrus.Warningf("update event reason: %v", err)
		}
	}

	r = r.WithContext(context.WithValue(r.Context(), render.StatusCtxKey, 412))
	render.DefaultResponder(w, r, ResponseBody{Msg: msg})
}

//ReturnBcodeError bcode error
func ReturnBcodeError(r *http.Request, w http.ResponseWriter, err error) {
	berr := bcode.Err2Coder(err)
	logrus.Debugf("path %s error code: %d; status: %d; error msg: %+v", r.RequestURI, berr.GetCode(), berr.GetStatus(), err)

	status := berr.GetStatus()
	result := Result{
		Code: berr.GetCode(),
		Msg:  berr.Error(),
	}

	if _, isErrBadRequest := err.(ErrBadRequest); isErrBadRequest {
		status = 400
		result.Code = 400
		result.Msg = err.Error()
	}

	if berr.GetStatus() == 500 {
		logrus.Errorf("path: %s\n: %+v", r.RequestURI, err)
	}

	r = r.WithContext(context.WithValue(r.Context(), render.StatusCtxKey, status))
	render.DefaultResponder(w, r, result)
}

// ReadEntity reads entity from http.Request
func ReadEntity(r *http.Request, x interface{}) error {
	if err := json.NewDecoder(r.Body).Decode(x); err != nil {
		return ErrBadRequest{err: err}
	}
	return nil
}

// ValidateStruct validates a structs exposed fields.
func ValidateStruct(x interface{}) error {
	if err := validate.Struct(x); err != nil {
		return ErrBadRequest{err: err}
	}
	return nil
}
