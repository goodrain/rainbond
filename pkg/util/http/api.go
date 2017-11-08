/*
Copyright 2017 The Goodrain Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package http

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"

	"golang.org/x/net/context"

	"github.com/go-chi/render"

	govalidator "github.com/thedevsaddam/govalidator"
)

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

//ReturnList 返回列表
func ReturnList(r *http.Request, w http.ResponseWriter, listAllNumber, page int, datas ...interface{}) {
	r = r.WithContext(context.WithValue(r.Context(), render.StatusCtxKey, http.StatusOK))
	render.DefaultResponder(w, r, ResponseBody{List: datas, ListAllNumber: listAllNumber, Page: page})
}

//ReturnError 返回错误信息
func ReturnError(r *http.Request, w http.ResponseWriter, code int, msg string) {
	r = r.WithContext(context.WithValue(r.Context(), render.StatusCtxKey, code))
	render.DefaultResponder(w, r, ResponseBody{Msg: msg})
}

//Return  自定义
func Return(r *http.Request, w http.ResponseWriter, code int, reb ResponseBody) {
	r = r.WithContext(context.WithValue(r.Context(), render.StatusCtxKey, code))
	render.DefaultResponder(w, r, reb)
}
