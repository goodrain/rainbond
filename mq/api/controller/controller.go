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
	"context"
	"io/ioutil"
	"strings"

	"github.com/goodrain/rainbond/mq/api/mq"

	"github.com/sirupsen/logrus"

	discovermodel "github.com/goodrain/rainbond/worker/discover/model"

	restful "github.com/emicklei/go-restful"
)

//Register 注册
func Register(container *restful.Container, mq mq.ActionMQ) {
	MQSource{mq}.Register(container)
}

//MQSource 消息队列接口
type MQSource struct {
	mq mq.ActionMQ
}

//Register 注册
func (u MQSource) Register(container *restful.Container) {
	ws := new(restful.WebService)
	ws.Path("/mq").
		Doc("message queue interface").
		Consumes(restful.MIME_XML, restful.MIME_JSON).
		Produces(restful.MIME_JSON, restful.MIME_XML) // you can specify this per route as well

	ws.Route(ws.POST("/{topic}").To(u.enqueue).
		// docs
		Doc("send a task message to the topic queue").
		Operation("enqueue").
		Param(ws.PathParameter("topic", "queue topic name").DataType("string")).
		Reads(discovermodel.Task{}).
		Returns(201, "消息发送成功", ResponseType{}).
		ReturnsError(400, "消息格式有误", ResponseType{}))

	ws.Route(ws.GET("/{topic}").To(u.dequeue).
		// docs
		Doc("get a task message from the topic queue").
		Operation("dequeue").
		Param(ws.PathParameter("topic", "queue topic name").DataType("string")).
		Writes(ResponseType{
			Body: ResponseBody{
				Bean: discovermodel.Task{},
			},
		})) // from the request
	ws.Route(ws.GET("/topics").To(u.getAllTopics).
		// docs
		Doc("get all support topic").
		Operation("getAllTopics").
		Writes(ResponseType{
			Body: ResponseBody{
				Bean: discovermodel.Task{},
			},
		})) // from the request
	container.Add(ws)
}

//ResponseType 返回内容
type ResponseType struct {
	Code      int          `json:"code"`
	Message   string       `json:"msg"`
	MessageCN string       `json:"msgcn"`
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

func (u *MQSource) enqueue(request *restful.Request, response *restful.Response) {
	topic := request.PathParameter("topic")
	if topic == "" || !u.mq.TopicIsExist(topic) {
		NewFaliResponse(400, "topic can not be empty or topic is not define", "主题不能为空或者当前主题未注册", response)
		return
	}
	body, err := ioutil.ReadAll(request.Request.Body)
	if err != nil {
		NewFaliResponse(500, "request body error."+err.Error(), "读取数据错误", response)
		return
	}
	request.Request.Body.Close()
	_, err = discovermodel.NewTask(body)
	if err != nil {
		NewFaliResponse(400, "request body error."+err.Error(), "读取数据错误，数据不合法", response)
		return
	}

	ctx, cancel := context.WithCancel(request.Request.Context())
	defer cancel()
	err = u.mq.Enqueue(ctx, topic, string(body))
	if err != nil {
		NewFaliResponse(500, "enqueue error."+err.Error(), "消息入队列错误", response)
		return
	}
	logrus.Debugf("Add a task to queue :%s", strings.TrimSpace(string(body)))
	NewPostSuccessResponse(nil, nil, response)
}

func (u *MQSource) dequeue(request *restful.Request, response *restful.Response) {
	topic := request.PathParameter("topic")
	if topic == "" || !u.mq.TopicIsExist(topic) {
		NewFaliResponse(400, "topic can not be empty or topic is not define", "主题不能为空或者当前主题未注册", response)
		return
	}
	ctx, cancel := context.WithCancel(request.Request.Context())
	defer cancel()
	value, err := u.mq.Dequeue(ctx, topic)
	if err != nil {
		NewFaliResponse(500, "dequeue error."+err.Error(), "消息出队列错误", response)
		return
	}
	task, err := discovermodel.NewTask([]byte(value))
	if err != nil {
		NewFaliResponse(500, "dequeue error."+err.Error(), "队列读出消息格式不合法", response)
		return
	}
	NewSuccessResponse(task, nil, response)
	logrus.Debugf("Consume a task from queue :%s", strings.TrimSpace(value))
}

func (u *MQSource) getAllTopics(request *restful.Request, response *restful.Response) {
	topics := u.mq.GetAllTopics()
	var list []interface{}
	for _, t := range topics {
		list = append(list, t)
	}
	NewSuccessResponse(nil, list, response)
}
