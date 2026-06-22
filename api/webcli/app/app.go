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
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/barnettZQG/gotty/server"
	"github.com/barnettZQG/gotty/webtty"
	k8sutil "github.com/goodrain/rainbond/util/k8s"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	restclient "k8s.io/client-go/rest"
)

// ExecuteCommandTotal metric
var ExecuteCommandTotal float64

// ExecuteCommandFailed metric
var ExecuteCommandFailed float64

const (
	// EnvDebugToolboxImage configures the toolbox image used for debug terminals.
	EnvDebugToolboxImage = "WEBCLI_DEBUG_TOOLBOX_IMAGE"
	// DefaultDebugToolboxImage is used when EnvDebugToolboxImage is not set.
	DefaultDebugToolboxImage = "registry.cn-hangzhou.aliyuncs.com/goodrain/toolbox:v1.0"

	webcliModeDebug          = "debug"
	debugContainerNamePrefix = "rb-debug-"
)

// App -
type App struct {
	upgrader   *websocket.Upgrader
	restClient *restclient.RESTClient
	coreClient kubernetes.Interface
	config     *restclient.Config
}

// Options options
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

// DefaultOptions -
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

// InitMessage -
type InitMessage struct {
	TenantID      string `json:"T_id"`
	ServiceID     string `json:"S_id"`
	PodName       string `json:"C_id"`
	ContainerName string `json:"containerName"`
	Md5           string `json:"Md5"`
	Namespace     string `json:"namespace"`
	Mode          string `json:"mode"`
}

// SetUpgrader -
func (app *App) SetUpgrader(u *websocket.Upgrader) {
	app.upgrader = u
}

// HandleWS -
func (app *App) HandleWS(w http.ResponseWriter, r *http.Request) {
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

	// 等待容器就绪，最多重试 30 次（30秒）
	var containerName, ip string
	var args []string
	maxRetries := 30
	retryInterval := time.Second

	for i := 0; i < maxRetries; i++ {
		if init.Mode == webcliModeDebug {
			containerName, ip, args, err = app.GetDebugContainerArgs(init.Namespace, init.PodName, init.ContainerName)
		} else {
			containerName, ip, args, err = app.GetContainerArgs(init.Namespace, init.PodName, init.ContainerName)
		}
		if err == nil {
			break
		}

		// 如果是容器未就绪的错误，继续等待
		errMsg := err.Error()
		if strings.Contains(errMsg, "not running yet") ||
			strings.Contains(errMsg, "not ready yet") ||
			strings.Contains(errMsg, "status not found") {
			logrus.Infof("waiting for container to be ready (attempt %d/%d): %s", i+1, maxRetries, errMsg)
			time.Sleep(retryInterval)
			continue
		}

		// 其他错误直接返回
		logrus.Errorf("get default container failure %s", err.Error())
		conn.WriteMessage(websocket.TextMessage, []byte("Get default container name failure!"))
		ExecuteCommandFailed++
		return
	}

	// 超过重试次数仍未就绪
	if err != nil {
		logrus.Errorf("container not ready after %d retries: %s", maxRetries, err.Error())
		conn.WriteMessage(websocket.TextMessage, []byte("Container is not ready, please try again later!"))
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
		webtty.WithReconnect(60),
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

// DebugToolboxImage returns the configured toolbox image for debug terminals.
func DebugToolboxImage() string {
	if image := strings.TrimSpace(os.Getenv(EnvDebugToolboxImage)); image != "" {
		return image
	}
	return DefaultDebugToolboxImage
}
func (app *App) CreateKubeClient() error {
	config, err := k8sutil.NewRestConfig("")
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

// SetConfigDefaults -
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

// GetContainerArgs get default container name
func (app *App) GetContainerArgs(namespace, podname, containerName string) (string, string, []string, error) {
	var args = []string{"/bin/sh"}
	pod, err := app.coreClient.CoreV1().Pods(namespace).Get(context.Background(), podname, metav1.GetOptions{})
	if err != nil {
		return "", "", args, err
	}

	if err := validateExecPod(pod); err != nil {
		return "", "", args, err
	}

	targetContainer, err := selectTargetContainer(pod, containerName)
	if err != nil {
		return "", "", args, fmt.Errorf("not have container in pod %s/%s", namespace, podname)
	}
	for _, env := range targetContainer.Env {
		if env.Name == "ES_DEFAULT_EXEC_ARGS" {
			args = strings.Split(env.Value, " ")
		}
	}

	if err := ensureContainerRunning(pod.Status.ContainerStatuses, targetContainer.Name); err != nil {
		return "", "", args, err
	}

	return targetContainer.Name, pod.Status.PodIP, args, nil
}

// GetDebugContainerArgs ensures an ephemeral toolbox container exists and returns exec args for it.
func (app *App) GetDebugContainerArgs(namespace, podname, containerName string) (string, string, []string, error) {
	var args = []string{"/bin/sh"}
	pod, err := app.coreClient.CoreV1().Pods(namespace).Get(context.Background(), podname, metav1.GetOptions{})
	if err != nil {
		return "", "", args, err
	}

	if err := validateExecPod(pod); err != nil {
		return "", "", args, err
	}

	targetContainer, err := selectTargetContainer(pod, containerName)
	if err != nil {
		return "", "", args, fmt.Errorf("not have container in pod %s/%s", namespace, podname)
	}
	if err := ensureContainerRunning(pod.Status.ContainerStatuses, targetContainer.Name); err != nil {
		return "", "", args, err
	}

	debugContainer, debugStatus := findReusableDebugContainer(pod, targetContainer.Name, DebugToolboxImage())
	if debugContainer != nil {
		if debugStatus != nil && debugStatus.State.Running != nil {
			return debugContainer.Name, pod.Status.PodIP, args, nil
		}
		if debugStatus != nil && debugStatus.State.Terminated != nil {
			return app.createDebugContainer(namespace, podname, pod, targetContainer, args)
		}
		return "", "", args, fmt.Errorf("debug container %s is not running yet", debugContainer.Name)
	}

	return app.createDebugContainer(namespace, podname, pod, targetContainer, args)
}

func (app *App) createDebugContainer(namespace, podname string, pod *api.Pod, targetContainer *api.Container, args []string) (string, string, []string, error) {
	debugName, err := nextDebugContainerName(pod, targetContainer.Name)
	if err != nil {
		return "", "", args, err
	}

	podCopy := pod.DeepCopy()
	podCopy.Spec.EphemeralContainers = append(podCopy.Spec.EphemeralContainers, api.EphemeralContainer{
		EphemeralContainerCommon: api.EphemeralContainerCommon{
			Name:            debugName,
			Image:           DebugToolboxImage(),
			ImagePullPolicy: api.PullIfNotPresent,
			Command:         []string{"/bin/sh", "-c", "trap : TERM INT; sleep infinity & wait"},
			VolumeMounts:    debugVolumeMounts(targetContainer),
			Stdin:           true,
			TTY:             true,
		},
		TargetContainerName: targetContainer.Name,
	})

	if _, err := app.coreClient.CoreV1().Pods(namespace).UpdateEphemeralContainers(context.Background(), podname, podCopy, metav1.UpdateOptions{}); err != nil {
		return "", "", args, fmt.Errorf("create debug container failure: %w", err)
	}
	return "", "", args, fmt.Errorf("debug container %s is not running yet", debugName)
}

func validateExecPod(pod *api.Pod) error {
	if pod.Status.Phase == api.PodSucceeded || pod.Status.Phase == api.PodFailed {
		return fmt.Errorf("cannot exec into a container in a completed pod; current phase is %s", pod.Status.Phase)
	}
	if pod.Status.Phase != api.PodRunning {
		return fmt.Errorf("pod is not running yet; current phase is %s", pod.Status.Phase)
	}
	return nil
}

func selectTargetContainer(pod *api.Pod, containerName string) (*api.Container, error) {
	for i := range pod.Spec.Containers {
		container := &pod.Spec.Containers[i]
		if container.Name == containerName || (containerName == "" && i == 0) {
			return container, nil
		}
	}
	return nil, fmt.Errorf("container %s not found", containerName)
}

func ensureContainerRunning(statuses []api.ContainerStatus, containerName string) error {
	for _, status := range statuses {
		if status.Name == containerName {
			if !status.Ready || status.State.Running == nil {
				return fmt.Errorf("container %s is not ready yet", containerName)
			}
			return nil
		}
	}
	return fmt.Errorf("container %s status not found", containerName)
}

func findReusableDebugContainer(pod *api.Pod, targetContainerName, image string) (*api.EphemeralContainer, *api.ContainerStatus) {
	var firstMatchedContainer *api.EphemeralContainer
	var firstMatchedStatus *api.ContainerStatus
	for i := range pod.Spec.EphemeralContainers {
		container := &pod.Spec.EphemeralContainers[i]
		if container.TargetContainerName != targetContainerName || container.Image != image {
			continue
		}
		status := findEphemeralContainerStatus(pod, container.Name)
		if status != nil && status.State.Running != nil {
			return container, status
		}
		if firstMatchedContainer == nil {
			firstMatchedContainer = container
			firstMatchedStatus = status
		}
	}
	return firstMatchedContainer, firstMatchedStatus
}

func findEphemeralContainerStatus(pod *api.Pod, containerName string) *api.ContainerStatus {
	for i := range pod.Status.EphemeralContainerStatuses {
		status := &pod.Status.EphemeralContainerStatuses[i]
		if status.Name == containerName {
			return status
		}
	}
	return nil
}

func debugVolumeMounts(targetContainer *api.Container) []api.VolumeMount {
	volumeMounts := make([]api.VolumeMount, 0, len(targetContainer.VolumeMounts))
	for _, volumeMount := range targetContainer.VolumeMounts {
		if volumeMount.SubPath != "" || volumeMount.SubPathExpr != "" {
			continue
		}
		volumeMounts = append(volumeMounts, volumeMount)
	}
	return volumeMounts
}

func nextDebugContainerName(pod *api.Pod, targetContainerName string) (string, error) {
	base := debugContainerBaseName(targetContainerName)
	for i := 0; i < 100; i++ {
		name := base
		if i > 0 {
			suffix := fmt.Sprintf("-%d", i)
			name = trimDNSLabel(base, len(suffix)) + suffix
		}
		if !podContainerNameExists(pod, name) {
			return name, nil
		}
	}
	return "", fmt.Errorf("cannot generate debug container name for %s", targetContainerName)
}

func debugContainerBaseName(targetContainerName string) string {
	name := sanitizeDNSLabel(debugContainerNamePrefix + targetContainerName)
	if name == "" || name == debugContainerNamePrefix[:len(debugContainerNamePrefix)-1] {
		name = debugContainerNamePrefix + "container"
	}
	return trimDNSLabel(name, 0)
}

func sanitizeDNSLabel(value string) string {
	value = strings.ToLower(value)
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		valid := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if valid {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func trimDNSLabel(name string, reservedSuffixLength int) string {
	maxLength := 63 - reservedSuffixLength
	if maxLength < 1 {
		maxLength = 1
	}
	if len(name) > maxLength {
		name = name[:maxLength]
	}
	return strings.Trim(name, "-")
}

func podContainerNameExists(pod *api.Pod, name string) bool {
	for _, container := range pod.Spec.Containers {
		if container.Name == name {
			return true
		}
	}
	for _, container := range pod.Spec.InitContainers {
		if container.Name == name {
			return true
		}
	}
	for _, container := range pod.Spec.EphemeralContainers {
		if container.Name == name {
			return true
		}
	}
	return false
}

// NewRequest new exec request
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

func md5Func(str string) string {
	h := md5.New()
	h.Write([]byte(str))
	cipherStr := h.Sum(nil)
	return hex.EncodeToString(cipherStr)
}
