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

package web

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/goodrain/rainbond/eventlog/cluster"
	"github.com/goodrain/rainbond/eventlog/cluster/discover"
	"github.com/goodrain/rainbond/eventlog/conf"
	"github.com/goodrain/rainbond/eventlog/exit/monitor"
	"github.com/goodrain/rainbond/eventlog/store"

	"golang.org/x/net/context"

	"fmt"

	"strings"

	_ "net/http/pprof"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	"github.com/twinj/uuid"
	httputil "github.com/goodrain/rainbond/util/http"

)

//SocketServer socket 服务
type SocketServer struct {
	conf                 conf.WebSocketConf
	log                  *logrus.Entry
	cancel               func()
	context              context.Context
	storemanager         store.Manager
	listenErr, errorStop chan error
	reStart              int
	timeout              time.Duration
	cluster              cluster.Cluster
	healthInfo           map[string]string
}

//NewSocket 创建zmq sub客户端
func NewSocket(conf conf.WebSocketConf, log *logrus.Entry, storeManager store.Manager, c cluster.Cluster, healthInfo map[string]string) *SocketServer {
	ctx, cancel := context.WithCancel(context.Background())
	d, err := time.ParseDuration(conf.TimeOut)
	if err != nil {
		d = time.Minute * 1
	}

	return &SocketServer{
		conf:         conf,
		log:          log,
		cancel:       cancel,
		context:      ctx,
		storemanager: storeManager,
		listenErr:    make(chan error),
		errorStop:    make(chan error),
		timeout:      d,
		cluster:      c,
		healthInfo:healthInfo,
	}
}

func (s *SocketServer) pushEventMessage(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:    s.conf.ReadBufferSize,
		WriteBufferSize:   s.conf.WriteBufferSize,
		EnableCompression: s.conf.EnableCompression,
		Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {

		},
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Error("Create web socket conn error.", err.Error())
		return
	}
	defer conn.Close()
	_, me, err := conn.ReadMessage()
	if err != nil {
		s.log.Error("Read EventID from first message error.", err.Error())
		return
	}
	conn.WriteMessage(websocket.TextMessage, []byte("ok"))
	info := strings.Split(string(me), "=")
	if len(info) != 2 {
		s.log.Error("Read EventID from first message error. The data format is not correct")
		return
	}
	EventID := info[1]
	if EventID == "" {
		s.log.Error("Event ID can not be empty when get socket message")
		return
	}
	s.log.Infof("Begin push event message of event (%s)", EventID)
	SubID := uuid.NewV4().String()
	ch := s.storemanager.WebSocketMessageChan("event", EventID, SubID)
	if ch == nil {
		// w.Write([]byte("Real-time message does not exist."))
		// w.Header().Set("Status Code", "200")
		s.log.Error("get web socket message chan from storemanager error.")
		return
	}
	defer func() {
		s.log.Debug("Push event message request closed")
		s.storemanager.RealseWebSocketMessageChan("event", EventID, SubID)
	}()
	stop := make(chan struct{})
	go s.reader(conn, stop)
	pingTicker := time.NewTicker(s.timeout * 8 / 10)
	defer pingTicker.Stop()
	for {
		select {
		case message, ok := <-ch:
			if !ok {
				return
			}
			if message != nil {
				//s.log.Debugf("websocket push a message,%s", message.Message)
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				err = conn.WriteMessage(websocket.TextMessage, message.Content)
				if err != nil {
					s.log.Warn("Push message to client error.", err.Error())
					return
				}
			}
		case <-stop:
			return
		case <-s.context.Done():
			return
		case <-pingTicker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}

}

func (s *SocketServer) pushDockerLog(w http.ResponseWriter, r *http.Request) {
	// if r.FormValue("host") == "" || r.FormValue("host") != s.cluster.GetInstanceID() {
	// 	w.WriteHeader(404)
	// 	return
	// }
	upgrader := websocket.Upgrader{
		ReadBufferSize:    s.conf.ReadBufferSize,
		WriteBufferSize:   s.conf.WriteBufferSize,
		EnableCompression: s.conf.EnableCompression,
		Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {

		},
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Error("Create web socket conn error.", err.Error())
		return
	}
	defer conn.Close()
	_, me, err := conn.ReadMessage()
	if err != nil {
		s.log.Error("Read ServiceID from first message error.", err.Error())
		return
	}
	info := strings.Split(string(me), "=")
	if len(info) != 2 {
		s.log.Error("Read ServiceID from first message error. The data format is not correct")
		return
	}
	ServiceID := info[1]
	if ServiceID == "" {
		s.log.Error("ServiceID ID can not be empty when get socket message")
		return
	}
	s.log.Infof("Begin push docker message of service (%s)", ServiceID)
	SubID := uuid.NewV4().String()
	ch := s.storemanager.WebSocketMessageChan("docker", ServiceID, SubID)
	if ch == nil {
		// w.Write([]byte("Real-time message does not exist."))
		// w.Header().Set("Status Code", "200")
		s.log.Error("get web socket message chan from storemanager error.")
		return
	}
	defer func() {
		s.log.Debug("Push docker log message request closed")
		s.storemanager.RealseWebSocketMessageChan("docker", ServiceID, SubID)
	}()
	conn.WriteMessage(websocket.TextMessage, []byte("ok"))
	stop := make(chan struct{})
	go s.reader(conn, stop)
	pingTicker := time.NewTicker(s.timeout * 8 / 10)
	defer pingTicker.Stop()
	for {
		select {
		case message, ok := <-ch:
			if !ok {
				return
			}
			if message != nil {
				//s.log.Debugf("websocket push a message,%s", message.Message)
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				err = conn.WriteMessage(websocket.TextMessage, message.Content)
				if err != nil {
					s.log.Warn("Push message to client error.", err.Error())
					return
				}
			}
		case <-stop:
			return
		case <-s.context.Done():
			return
		case <-pingTicker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}

}
func (s *SocketServer) pushMonitorMessage(w http.ResponseWriter, r *http.Request) {
	// if r.FormValue("host") == "" || r.FormValue("host") != s.cluster.GetInstanceID() {
	// 	w.WriteHeader(404)
	// 	return
	// }
	upgrader := websocket.Upgrader{
		ReadBufferSize:    s.conf.ReadBufferSize,
		WriteBufferSize:   s.conf.WriteBufferSize,
		EnableCompression: s.conf.EnableCompression,
		Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {

		},
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Error("Create web socket conn error.", err.Error())
		return
	}
	defer conn.Close()
	_, me, err := conn.ReadMessage()
	if err != nil {
		s.log.Error("Read tag key from first message error.", err.Error())
		return
	}
	info := strings.Split(string(me), "=")
	if len(info) != 2 {
		s.log.Error("Read tag key from first message error. The data format is not correct")
		return
	}
	ServiceID := info[1]
	if ServiceID == "" {
		s.log.Error("tag key can not be empty when get socket message")
		return
	}
	s.log.Infof("Begin push monitor message of service (%s)", ServiceID)
	SubID := uuid.NewV4().String()
	ch := s.storemanager.WebSocketMessageChan("monitor", ServiceID, SubID)
	if ch == nil {
		// w.Write([]byte("Real-time message does not exist."))
		// w.Header().Set("Status Code", "200")
		s.log.Error("get web socket message chan from storemanager error.")
		return
	}
	defer func() {
		s.log.Debug("Push docker log message request closed")
		s.storemanager.RealseWebSocketMessageChan("monitor", ServiceID, SubID)
	}()
	conn.WriteMessage(websocket.TextMessage, []byte("ok"))
	stop := make(chan struct{})
	go s.reader(conn, stop)
	pingTicker := time.NewTicker(s.timeout * 8 / 10)
	defer pingTicker.Stop()
	for {
		select {
		case message, ok := <-ch:
			if !ok {
				return
			}
			if message != nil {
				s.log.Debugf("websocket push a monitor message,%s", string(message.MonitorData))
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				err = conn.WriteMessage(websocket.TextMessage, message.MonitorData)
				if err != nil {
					s.log.Warn("Push message to client error.", err.Error())
					return
				}
			}
		case <-stop:
			return
		case <-s.context.Done():
			return
		case <-pingTicker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}

}
func (s *SocketServer) pushNewMonitorMessage(w http.ResponseWriter, r *http.Request) {
	// if r.FormValue("host") == "" || r.FormValue("host") != s.cluster.GetInstanceID() {
	// 	w.WriteHeader(404)
	// 	return
	// }
	upgrader := websocket.Upgrader{
		ReadBufferSize:    s.conf.ReadBufferSize,
		WriteBufferSize:   s.conf.WriteBufferSize,
		EnableCompression: s.conf.EnableCompression,
		Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {

		},
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Error("Create web socket conn error.", err.Error())
		return
	}
	defer conn.Close()
	_, me, err := conn.ReadMessage()
	if err != nil {
		s.log.Error("Read tag key from first message error.", err.Error())
		return
	}
	info := strings.Split(string(me), "=")
	if len(info) != 2 {
		s.log.Error("Read tag key from first message error. The data format is not correct")
		return
	}
	ServiceID := info[1]
	if ServiceID == "" {
		s.log.Error("tag key can not be empty when get socket message")
		return
	}
	s.log.Infof("Begin push monitor message of service (%s)", ServiceID)
	SubID := uuid.NewV4().String()
	ch := s.storemanager.WebSocketMessageChan("newmonitor", ServiceID, SubID)
	if ch == nil {
		// w.Write([]byte("Real-time message does not exist."))
		// w.Header().Set("Status Code", "200")
		s.log.Error("get web socket message chan from storemanager error.")
		return
	}
	defer func() {
		s.log.Debug("Push new monitor message request closed")
		s.storemanager.RealseWebSocketMessageChan("newmonitor", ServiceID, SubID)
	}()
	conn.WriteMessage(websocket.TextMessage, []byte("ok"))
	stop := make(chan struct{})
	go s.reader(conn, stop)
	pingTicker := time.NewTicker(s.timeout * 8 / 10)
	defer pingTicker.Stop()
	for {
		select {
		case message, ok := <-ch:
			if !ok {
				return
			}
			if message != nil {
				s.log.Debugf("websocket push a new monitor message")
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				err = conn.WriteMessage(websocket.TextMessage, message.MonitorData)
				if err != nil {
					s.log.Warn("Push message to client error.", err.Error())
					return
				}
			}
		case <-stop:
			return
		case <-s.context.Done():
			return
		case <-pingTicker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}

}
func (s *SocketServer) reader(ws *websocket.Conn, ch chan struct{}) {
	defer ws.Close()
	ws.SetReadLimit(512)
	ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(s.timeout)); return nil })
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
	}
	s.log.Debug("socket conn ping/pong time out ,will closed.")
	close(ch)
}

//Run 执行
func (s *SocketServer) Run() error {
	s.log.Info("WebSocker Server start")
	go s.listen()
	go s.checkHealth()
	return nil
}
func (s *SocketServer) listen() {
	http.HandleFunc("/event_log", s.pushEventMessage)
	http.HandleFunc("/docker_log", s.pushDockerLog)
	http.HandleFunc("/monitor_message", s.pushMonitorMessage)
	http.HandleFunc("/new_monitor_message", s.pushNewMonitorMessage)
	http.HandleFunc("/monitor", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})
	http.HandleFunc("/docker-instance", func(w http.ResponseWriter, r *http.Request) {
		ServiceID := r.FormValue("service_id")
		if ServiceID == "" {
			w.WriteHeader(412)
			w.Write([]byte(`{"message":"service id can not be empty.","status":"failure"}`))
			return
		}
		s.log.Info("ServiceID:" + ServiceID)
		instance := s.cluster.GetSuitableInstance(ServiceID)
		err := discover.SaveDockerLogInInstance(s.context, ServiceID, instance.HostID)
		if err != nil {
			s.log.Error("Save docker service and instance id to etcd error.")
			w.WriteHeader(500)
			w.Write([]byte(`{"message":"Save docker service and instance id to etcd error.","status":"failure"}`))
			return
		}
		w.WriteHeader(200)
		url := fmt.Sprintf("tcp://%s:%d", instance.HostIP, instance.DockerLogPort)
		w.Write([]byte(`{"host":"` + url + `","status":"success"}`))
	})
	http.HandleFunc("/event_push", s.receiveEventMessage)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if s.healthInfo["status"] != "health"{
			httputil.ReturnError(r,w,400,"eventlog service unusual")
		}
		httputil.ReturnSuccess(r,w,s.healthInfo)
	})
	//monitor setting
	s.prometheus()

	if s.conf.SSL {
		go func() {
			addr := fmt.Sprintf("%s:%d", s.conf.BindIP, s.conf.SSLBindPort)
			s.log.Infof("web socket ssl server listen %s", addr)
			err := http.ListenAndServeTLS(addr, s.conf.CertFile, s.conf.KeyFile, nil)
			if err != nil {
				s.log.Error("websocket listen error.", err.Error())
				s.listenErr <- err
			}
		}()
	}
	addr := fmt.Sprintf("%s:%d", s.conf.BindIP, s.conf.BindPort)
	s.log.Infof("web socket server listen %s", addr)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		s.log.Error("websocket listen error.", err.Error())
		s.listenErr <- err
	}
}
func (s *SocketServer) checkHealth() {
	tike := time.Tick(time.Minute * 10)
	for {
		select {
		case <-s.context.Done():
			return
		case <-tike:
			s.reStart = 0
		case err := <-s.listenErr:
			if s.reStart > s.conf.MaxRestartCount {
				s.log.Error("Web socket server listen error count more than max restart count.")
				s.errorStop <- err
			} else {
				go s.listen()
				s.reStart++
			}
		}
	}
}

//ListenError 返回错误通道
func (s *SocketServer) ListenError() chan error {
	return s.errorStop
}

//Stop 停止
func (s *SocketServer) Stop() {
	s.log.Info("WebSocker Server stop")
	s.cancel()
}

//receiveEventMessage 接收操作日志API
func (s *SocketServer) receiveEventMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var re ResponseType
	message, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(500)
		re = NewResponseType(500, err.Error(), "读取event消息内容错误", nil, nil)
	} else {
		select {
		case s.storemanager.ReceiveMessageChan() <- message:
			re = NewSuccessResponse(nil, nil)
			w.WriteHeader(200)
		default:
			re = NewResponseType(500, "event message chan is block", "event消息通道堵塞", nil, nil)
			w.WriteHeader(500)
		}
	}
	if r.Body != nil {
		r.Body.Close()
	}
	json.NewEncoder(w).Encode(re)
	return
}

func (s *SocketServer) prometheus() {
	prometheus.MustRegister(version.NewCollector("event_log"))
	exporter := monitor.NewExporter(s.storemanager, s.cluster)
	prometheus.MustRegister(exporter)
	http.Handle(s.conf.PrometheusMetricPath, promhttp.Handler())
}

//ResponseType 返回内容
type ResponseType struct {
	Code      int          `json:"code"`
	Message   string       `json:"msg"`
	MessageCN string       `json:"msgcn"`
	Body      ResponseBody `json:"body,omitempty"`
}

//ResponseBody 返回主体
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

//NewSuccessResponse 创建成功返回结构
func NewSuccessResponse(bean interface{}, list []interface{}) ResponseType {
	return NewResponseType(200, "", "", bean, list)
}
