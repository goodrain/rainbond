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

package app

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"text/template"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/barnettZQG/gotty/server"
	"github.com/barnettZQG/gotty/webtty"
	httputil "github.com/goodrain/rainbond/util/http"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yudai/umutex"
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
)

//ExecuteCommandTotal metric
var ExecuteCommandTotal float64

//ExecuteCommandFailed metric
var ExecuteCommandFailed float64

//App -
type App struct {
	options *Options

	upgrader *websocket.Upgrader

	titleTemplate *template.Template

	onceMutex  *umutex.UnblockingMutex
	restClient *restclient.RESTClient
	coreClient *kubernetes.Clientset
	config     *restclient.Config
}

//Options options
type Options struct {
	Address     string `hcl:"address"`
	Port        string `hcl:"port"`
	PermitWrite bool   `hcl:"permit_write"`
	IndexFile   string `hcl:"index_file"`
	//titile format by golang templete
	TitleFormat     string                 `hcl:"title_format"`
	EnableReconnect bool                   `hcl:"enable_reconnect"`
	ReconnectTime   int                    `hcl:"reconnect_time"`
	PermitArguments bool                   `hcl:"permit_arguments"`
	CloseSignal     int                    `hcl:"close_signal"`
	RawPreferences  map[string]interface{} `hcl:"preferences"`
	SessionKey      string                 `hcl:"session_key"`
	K8SConfPath     string
}

//Version -
var Version = "0.0.2"

//DefaultOptions -
var DefaultOptions = Options{
	Address:         "",
	Port:            "8080",
	PermitWrite:     true,
	IndexFile:       "",
	TitleFormat:     "GRTTY Command",
	EnableReconnect: true,
	ReconnectTime:   10,
	CloseSignal:     1, // syscall.SIGHUP
	SessionKey:      "_auth_user_id",
}

//InitMessage -
type InitMessage struct {
	TenantID      string `json:"T_id"`
	ServiceID     string `json:"S_id"`
	PodName       string `json:"C_id"`
	ContainerName string `json:"containerName"`
	Md5           string `json:"Md5"`
	Namespace     string `json:"namespace"`
}

func checkSameOrigin(r *http.Request) bool {
	return true
}

//New -
func New(options *Options) (*App, error) {
	titleTemplate, _ := template.New("title").Parse(options.TitleFormat)
	app := &App{
		options: options,
		upgrader: &websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			Subprotocols:    []string{"webtty"},
			CheckOrigin:     checkSameOrigin,
		},
		titleTemplate: titleTemplate,
		onceMutex:     umutex.New(),
	}
	//create kube client and config
	if err := app.createKubeClient(); err != nil {
		return nil, err
	}
	return app, nil
}

//Run Run
func (app *App) Run() error {

	endpoint := net.JoinHostPort(app.options.Address, app.options.Port)

	wsHandler := http.HandlerFunc(app.handleWS)
	health := http.HandlerFunc(app.healthCheck)

	var siteMux = http.NewServeMux()

	siteHandler := http.Handler(siteMux)

	siteHandler = wrapHeaders(siteHandler)

	exporter := NewExporter()
	prometheus.MustRegister(exporter)

	wsMux := http.NewServeMux()
	wsMux.Handle("/", siteHandler)
	wsMux.Handle("/docker_console", wsHandler)
	wsMux.Handle("/health", health)
	wsMux.Handle("/metrics", promhttp.Handler())

	siteHandler = (http.Handler(wsMux))

	siteHandler = wrapLogger(siteHandler)

	server, err := app.makeServer(endpoint, &siteHandler)
	if err != nil {
		return errors.New("Failed to build server: " + err.Error())
	}
	go func() {
		logrus.Printf("webcli listen %s", endpoint)
		logrus.Fatal(server.ListenAndServe())
		logrus.Printf("Exiting...")
	}()
	return nil
}

func (app *App) makeServer(addr string, handler *http.Handler) (*http.Server, error) {
	server := &http.Server{
		Addr:    addr,
		Handler: *handler,
	}

	return server, nil
}

func (app *App) healthCheck(w http.ResponseWriter, r *http.Request) {
	httputil.ReturnSuccess(r, w, map[string]string{"status": "health", "info": "webcli service health"})
}

func (app *App) handleWS(w http.ResponseWriter, r *http.Request) {
	logrus.Printf("New client connected: %s", r.RemoteAddr)

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	conn, err := app.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logrus.Print("Failed to upgrade connection: " + err.Error())
		return
	}

	_, stream, err := conn.ReadMessage()
	if err != nil {
		logrus.Print("Failed to authenticate websocket connection " + err.Error())
		conn.Close()
		return
	}

	message := string(stream)
	logrus.Print("message=", message)

	var init InitMessage

	err = json.Unmarshal(stream, &init)

	//todo auth
	if init.PodName == "" {
		logrus.Print("Parameter is error, pod name is empty")
		conn.WriteMessage(websocket.TextMessage, []byte("pod name can not be empty"))
		conn.Close()
		return
	}
	key := init.TenantID + "_" + init.ServiceID + "_" + init.PodName
	md5 := md5Func(key)
	if md5 != init.Md5 {
		logrus.Print("Auth is not allowed !")
		conn.WriteMessage(websocket.TextMessage, []byte("Auth is not allowed!"))
		conn.Close()
		return
	}
	// base kubernetes api create exec slave
	if init.Namespace == "" {
		init.Namespace = init.TenantID
	}
	containerName, ip, args, err := app.GetContainerArgs(init.Namespace, init.PodName, init.ContainerName)
	if err != nil {
		logrus.Errorf("get default container failure %s", err.Error())
		conn.WriteMessage(websocket.TextMessage, []byte("Get default container name failure!"))
		ExecuteCommandFailed++
		return
	}
	request := app.NewRequest(init.PodName, init.Namespace, containerName, args)
	var slave server.Slave
	slave, err = NewExecContext(request, app.config)
	if err != nil {
		logrus.Errorf("open exec context failure %s", err.Error())
		conn.WriteMessage(websocket.TextMessage, []byte("open tty failure!"))
		ExecuteCommandFailed++
		return
	}
	defer slave.Close()
	opts := []webtty.Option{
		webtty.WithWindowTitle([]byte(ip)),
		webtty.WithReconnect(10),
		webtty.WithPermitWrite(),
	}
	// create web tty and run
	tty, err := webtty.New(&WsWrapper{conn}, slave, opts...)
	if err != nil {
		logrus.Errorf("open web tty context failure %s", err.Error())
		conn.WriteMessage(websocket.TextMessage, []byte("open tty failure!"))
		ExecuteCommandFailed++
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	err = tty.Run(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "master closed") {
			logrus.Infof("client close connection")
			return
		}
		logrus.Errorf("run web tty failure %s", err.Error())
		conn.WriteMessage(websocket.TextMessage, []byte("run tty failure!"))
		ExecuteCommandFailed++
		return
	}
}

//Exit -
func (app *App) Exit() (firstCall bool) {
	return true
}

func (app *App) createKubeClient() error {
	config, err := k8sutil.NewRestConfig(app.options.K8SConfPath)
	if err != nil {
		return err
	}
	config.UserAgent = "rainbond/webcli"
	coreAPI, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	SetConfigDefaults(config)
	app.config = config
	restClient, err := restclient.RESTClientFor(config)
	if err != nil {
		return err
	}
	app.restClient = restClient
	app.coreClient = coreAPI
	return nil
}

//SetConfigDefaults -
func SetConfigDefaults(config *rest.Config) error {
	if config.APIPath == "" {
		config.APIPath = "/api"
	}
	config.GroupVersion = &schema.GroupVersion{Group: "", Version: "v1"}
	config.NegotiatedSerializer = serializer.NewCodecFactory(runtime.NewScheme())
	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
	return nil
}

//GetContainerArgs get default container name
func (app *App) GetContainerArgs(namespace, podname, containerName string) (string, string, []string, error) {
	var args = []string{"/bin/sh"}
	pod, err := app.coreClient.CoreV1().Pods(namespace).Get(context.Background(), podname, metav1.GetOptions{})
	if err != nil {
		return "", "", args, err
	}

	if pod.Status.Phase == api.PodSucceeded || pod.Status.Phase == api.PodFailed {
		return "", "", args, fmt.Errorf("cannot exec into a container in a completed pod; current phase is %s", pod.Status.Phase)
	}
	for i, container := range pod.Spec.Containers {
		if container.Name == containerName || (containerName == "" && i == 0) {
			for _, env := range container.Env {
				if env.Name == "ES_DEFAULT_EXEC_ARGS" {
					args = strings.Split(env.Value, " ")
				}
			}
			return container.Name, pod.Status.PodIP, args, nil
		}
	}
	return "", "", args, fmt.Errorf("not have container in pod %s/%s", namespace, podname)
}

//NewRequest new exec request
func (app *App) NewRequest(podName, namespace, containerName string, command []string) *restclient.Request {
	// TODO: consider abstracting into a client invocation or client helper
	req := app.restClient.Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		Param("container", containerName).
		Param("stdin", "true").
		Param("stdout", "true").
		Param("stderr", "false").
		Param("tty", "true")
	for _, c := range command {
		req.Param("command", c)
	}
	return req
}

func wrapLogger(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &responseWrapper{w, 200}
		handler.ServeHTTP(rw, r)
		logrus.Printf("%s %d %s %s", r.RemoteAddr, rw.status, r.Method, r.URL.Path)
	})
}

func wrapHeaders(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "GoTTY/"+Version)
		handler.ServeHTTP(w, r)
	})
}

func md5Func(str string) string {
	h := md5.New()
	h.Write([]byte(str))
	cipherStr := h.Sum(nil)
	return hex.EncodeToString(cipherStr)
}
