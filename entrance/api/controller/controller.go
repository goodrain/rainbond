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
	"github.com/goodrain/rainbond/entrance/core"
	"github.com/goodrain/rainbond/entrance/store"

	"k8s.io/client-go/kubernetes"

	apistore "github.com/goodrain/rainbond/entrance/api/store"

	restful "github.com/emicklei/go-restful"
)

//Register api register
func Register(container *restful.Container, coreManager core.Manager, readStore store.ReadStore, apiStoreManager *apistore.Manager, clientSet *kubernetes.Clientset) {
	DomainSource{coreManager, readStore, apiStoreManager, 10000}.Register(container)
	NodeSource{coreManager, readStore, apiStoreManager, clientSet}.Register(container)
	PodSource{apiStoreManager}.Register(container)
	HealthStatus{apiStoreManager}.Register(container)
}

//ResponseType 返回内容
type ResponseType struct {
	Code      int          `json:"code"`
	Message   string       `json:"msg,omitempty"`
	MessageCN string       `json:"msgcn,omitempty"`
	Body      ResponseBody `json:"body,omitempty"`
}

//ResponseBody 返回主要内容体
type ResponseBody struct {
	Bean     interface{}   `json:"bean,omitempty"`
	List     []interface{} `json:"list,omitempty"`
	PageNum  int           `json:"pageNumber,omitempty"`
	PageSize int           `json:"pageSize,omitempty"`
	Total    int           `json:"total,omitempty"`
}

//NewResponseType 构建返回结构
func NewResponseType(code int, message string, messageCN string, bean interface{}, list []interface{}) ResponseType {
	return ResponseType{
		Code:      code,
		Message:   message,
		MessageCN: messageCN,
		Body: ResponseBody{
			Bean: bean,
			List: list,
		},
	}
}

//NewPostSuccessResponse 创建成功返回结构
func NewPostSuccessResponse(bean interface{}, list []interface{}, response *restful.Response) {
	response.WriteHeaderAndJson(201, NewResponseType(201, "", "", bean, list), restful.MIME_JSON)
	return
}

//NewSuccessResponse 创建成功返回结构
func NewSuccessResponse(bean interface{}, list []interface{}, response *restful.Response) {
	response.WriteHeaderAndJson(200, NewResponseType(200, "", "", bean, list), restful.MIME_JSON)
	return
}

//NewSuccessMessageResponse 创建成功返回结构
func NewSuccessMessageResponse(bean interface{}, list []interface{}, message, messageCN string, response *restful.Response) {
	response.WriteHeaderAndJson(200, NewResponseType(200, message, messageCN, bean, list), restful.MIME_JSON)
	return
}

//NewFaliResponse 创建返回失败结构
func NewFaliResponse(code int, message string, messageCN string, response *restful.Response) {
	response.WriteHeaderAndJson(code, NewResponseType(code, message, messageCN, nil, nil), restful.MIME_JSON)
	return
}
