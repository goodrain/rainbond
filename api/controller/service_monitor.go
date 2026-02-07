package controller

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/goodrain/rainbond/api/client/prometheus"
	"github.com/goodrain/rainbond/api/proxy"
	"github.com/goodrain/rainbond/api/util"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/pkg/component/storage"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	api_model "github.com/goodrain/rainbond/api/model"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	httputil "github.com/goodrain/rainbond/util/http"
)

// AddServiceMonitors add service monitor
func (t *TenantStruct) AddServiceMonitors(w http.ResponseWriter, r *http.Request) {
	var add api_model.AddServiceMonitorRequestStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &add, nil)
	if !ok {
		return
	}
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)
	tsm, err := handler.GetServiceManager().AddServiceMonitor(tenantID, serviceID, add)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, tsm)
}

// DeleteServiceMonitors delete service monitor
func (t *TenantStruct) DeleteServiceMonitors(w http.ResponseWriter, r *http.Request) {
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)
	name := chi.URLParam(r, "name")
	tsm, err := handler.GetServiceManager().DeleteServiceMonitor(tenantID, serviceID, name)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, tsm)
}

// UpdateServiceMonitors update service monitor
func (t *TenantStruct) UpdateServiceMonitors(w http.ResponseWriter, r *http.Request) {
	var update api_model.UpdateServiceMonitorRequestStruct
	ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &update, nil)
	if !ok {
		return
	}
	name := chi.URLParam(r, "name")
	serviceID := r.Context().Value(ctxutil.ContextKey("service_id")).(string)
	tenantID := r.Context().Value(ctxutil.ContextKey("tenant_id")).(string)
	tsm, err := handler.GetServiceManager().UpdateServiceMonitor(tenantID, serviceID, name, update)
	if err != nil {
		httputil.ReturnBcodeError(r, w, err)
		return
	}
	httputil.ReturnSuccess(r, w, tsm)
}

// UploadPackage upload package
func (t *TenantStruct) UploadPackage(w http.ResponseWriter, r *http.Request) {
	eventID := strings.TrimSpace(chi.URLParam(r, "eventID"))
	switch r.Method {
	case "POST":
		if eventID == "" {
			httputil.ReturnError(r, w, 400, "Failed to parse eventID.")
			return
		}
		logrus.Debug("Start receive upload file: ", eventID)
		reader, header, err := r.FormFile("packageTarFile")
		if err != nil {
			logrus.Errorf("Failed to parse upload file: %s", err.Error())
			httputil.ReturnError(r, w, 501, "Failed to parse upload file.")
			return
		}
		defer reader.Close()

		dirName := fmt.Sprintf("/grdata/package_build/temp/events/%s", eventID)
		storage.Default().StorageCli.MkdirAll(dirName)
		fileName := fmt.Sprintf("%s/%s", dirName, header.Filename)
		err = storage.Default().StorageCli.SaveFile(fileName, reader)
		if err != nil {
			httputil.ReturnError(r, w, 503, "Failed to save file: "+err.Error())
		}
		logrus.Debug("successful write file to: ", fileName)
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Add("Access-Control-Allow-Origin", origin)
		w.Header().Add("Access-Control-Allow-Methods", "POST,OPTIONS")
		w.Header().Add("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Access-Control-Allow-Headers", "x-requested-with,Content-Type,X-Custom-Header,X-TEAM-NAME,X-REGION-NAME")
		httputil.ReturnSuccess(r, w, nil)

	case "OPTIONS":
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Add("Access-Control-Allow-Origin", origin)
		w.Header().Add("Access-Control-Allow-Methods", "POST,OPTIONS")
		w.Header().Add("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Access-Control-Allow-Headers", "x-requested-with,Content-Type,X-Custom-Header,X-TEAM-NAME,X-REGION-NAME")
		httputil.ReturnSuccess(r, w, nil)
	}
}

// GetMonitorMetrics get monitor metrics
func GetMonitorMetrics(w http.ResponseWriter, r *http.Request) {
	target := r.FormValue("target")
	var metricMetadatas []prometheus.Metadata
	if target == "tenant" {
		metricMetadatas = handler.GetMonitorHandle().GetTenantMonitorMetrics(r.FormValue("tenant"))
	}
	if target == "app" {
		metricMetadatas = handler.GetMonitorHandle().GetAppMonitorMetrics(r.FormValue("tenant"), r.FormValue("app"))
	}
	if target == "component" {
		metricMetadatas = handler.GetMonitorHandle().GetComponentMonitorMetrics(r.FormValue("tenant"), r.FormValue("component"))
	}
	httputil.ReturnSuccess(r, w, metricMetadatas)
}

var fileManage *FileManage

// FileManage docker log
type FileManage struct {
	socketproxy proxy.Proxy
	clientset   *clientset.Clientset
	config      *rest.Config
}

// GetFileManage get docker log
func GetFileManage() *FileManage {
	if fileManage == nil {
		fileManage = &FileManage{
			socketproxy: proxy.CreateProxy("acp_node", "http", []string{"rbd-node:6100"}),
			clientset:   k8s.Default().Clientset,
			config:      k8s.Default().RestConfig,
		}
		//discover.GetEndpointDiscover().AddProject("acp_node", fileManage.socketproxy)
	}
	return fileManage
}

// UploadEvent volume file upload event
func (f FileManage) UploadEvent(w http.ResponseWriter, r *http.Request) {
	volumeName := w.Header().Get("volume_name")
	userName := w.Header().Get("user_name")
	tenantID := w.Header().Get("tenant_id")
	serviceID := w.Header().Get("service_id")
	fileName := w.Header().Get("file_name")
	status := w.Header().Get("status")
	msg := fmt.Sprintf("%v to upload file %v in storage %v", status, fileName, volumeName)
	_, err := util.CreateEvent(dbmodel.TargetTypeService, "volume-file-upload", serviceID, tenantID, "", userName, status, msg, 1)
	if err != nil {
		logrus.Error("create event error: ", err)
		httputil.ReturnError(r, w, 500, "操作失败")
	}
	httputil.ReturnSuccess(r, w, nil)
}

func (f FileManage) UploadFile(w http.ResponseWriter, r *http.Request) {
	// 设置 CORS 头
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}

	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Custom-Header")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Max-Age", "3600")

	// 处理预检请求
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	logrus.Debugf("开始处理文件上传请求，Method: %s, ContentType: %s, Origin: %s",
		r.Method, r.Header.Get("Content-Type"), origin)

	// 设置响应头
	w.Header().Set("volume_name", r.FormValue("volume_name"))
	w.Header().Set("user_name", r.FormValue("user_name"))
	w.Header().Set("tenant_id", r.FormValue("tenant_id"))
	w.Header().Set("service_id", r.FormValue("service_id"))
	w.Header().Set("status", "failed")

	// 获取上传参数
	destPath := r.FormValue("path")
	podName := r.FormValue("pod_name")
	namespace := r.FormValue("namespace")
	containerName := r.FormValue("container_name")

	logrus.Debugf("上传参数: destPath=%s, podName=%s, namespace=%s, containerName=%s",
		destPath, podName, namespace, containerName)

	if destPath == "" {
		httputil.ReturnError(r, w, 400, "目标路径不能为空")
		return
	}

	// 解析multipart form数据
	err := r.ParseMultipartForm(32 << 20) // 32MB
	if err != nil {
		logrus.Errorf("解析multipart form失败: %v", err)
		httputil.ReturnError(r, w, 400, fmt.Sprintf("解析上传数据失败: %v", err))
		return
	}

	// 检查form是否为nil
	if r.MultipartForm == nil {
		logrus.Error("MultipartForm为空")
		httputil.ReturnError(r, w, 400, "未找到上传文件")
		return
	}

	// 打印所有可用的form字段
	logrus.Debugf("Form字段: %+v", r.MultipartForm.Value)
	logrus.Debugf("文件字段: %+v", r.MultipartForm.File)

	// 尝试获取文件
	var files []*multipart.FileHeader
	if formFiles := r.MultipartForm.File["files"]; len(formFiles) > 0 {
		files = formFiles
	} else if formFiles := r.MultipartForm.File["file"]; len(formFiles) > 0 {
		files = formFiles
	}

	if len(files) == 0 {
		logrus.Error("未找到上传文件")
		httputil.ReturnError(r, w, 400, "未找到上传文件")
		return
	}

	// 创建临时目录存储上传的文件
	tempDir, err := os.MkdirTemp("", "upload-*")
	if err != nil {
		logrus.Errorf("创建临时目录失败: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("创建临时目录失败: %v", err))
		return
	}
	defer os.RemoveAll(tempDir)

	// 保存文件到临时目录
	for _, fileHeader := range files {
		logrus.Debugf("处理文件: %s", fileHeader.Filename)

		// 获取文件的相对路径
		var relativePath string
		contentDisposition := fileHeader.Header.Get("Content-Disposition")
		filenameRegex := regexp.MustCompile(`filename="([^"]+)"`)
		matches := filenameRegex.FindStringSubmatch(contentDisposition)
		if len(matches) > 1 {
			relativePath = matches[1] // 使用正则表达式从Content-Disposition中提取filename
			logrus.Debugf("使用 Content-Disposition 中的 filename: %s", relativePath)
		} else if webkitPath := fileHeader.Header.Get("webkitRelativePath"); webkitPath != "" {
			relativePath = webkitPath
			logrus.Debugf("使用 webkitRelativePath: %s", relativePath)
		} else {
			relativePath = fileHeader.Filename
			logrus.Debugf("使用文件名作为路径: %s", relativePath)
		}

		file, err := fileHeader.Open()
		if err != nil {
			logrus.Errorf("打开上传文件失败: %v", err)
			httputil.ReturnError(r, w, 500, fmt.Sprintf("打开上传文件失败: %v", err))
			return
		}
		defer file.Close()

		// 构建临时目录中的完整路径
		tempPath := filepath.Join(tempDir, relativePath)
		logrus.Debugf("临时文件路径: %s", tempPath)

		// 确保目标目录存在
		if err := os.MkdirAll(filepath.Dir(tempPath), 0755); err != nil {
			logrus.Errorf("创建目录失败: %v", err)
			httputil.ReturnError(r, w, 500, fmt.Sprintf("创建目录失败: %v", err))
			return
		}

		dst, err := os.Create(tempPath)
		if err != nil {
			logrus.Errorf("创建文件失败: %v", err)
			httputil.ReturnError(r, w, 500, fmt.Sprintf("创建文件失败: %v", err))
			return
		}
		defer dst.Close()

		if _, err = io.Copy(dst, file); err != nil {
			logrus.Errorf("保存文件失败: %v", err)
			httputil.ReturnError(r, w, 500, fmt.Sprintf("保存文件失败: %v", err))
			return
		}
		logrus.Debugf("成功保存文件: %s", tempPath)
	}

	// 上传文件到容器
	logrus.Debugf("开始上传文件到容器，源目录: %s，目标路径: %s", tempDir, destPath)
	// 获取第一个文件的目录结构信息来判断是单文件还是目录上传
	firstFile := files[0]
	if webkitPath := firstFile.Header.Get("webkitRelativePath"); webkitPath != "" {
		// 如果是目录上传(有webkitRelativePath),使用临时目录内容
		err = f.AppFileUpload(containerName, podName, tempDir, destPath, namespace)
	} else if len(files) == 1 {
		// 如果是单文件上传
		uploadPath := filepath.Join(tempDir, firstFile.Filename)
		err = f.AppFileUpload(containerName, podName, uploadPath, destPath, namespace)
	} else {
		// 多文件上传
		err = f.AppFileUpload(containerName, podName, tempDir, destPath, namespace)
	}

	if err != nil {
		logrus.Errorf("上传到容器失败: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("上传失败: %v", err))
		return
	}

	logrus.Debug("文件上传成功")
	w.Header().Set("status", "success")
	httputil.ReturnSuccess(r, w, nil)
}

func (f FileManage) DownloadFile(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("接收到文件下载请求: Method=%s, ContentType=%s", r.Method, r.Header.Get("Content-Type"))

	// 设置响应头
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("volume_name", r.FormValue("volume_name"))
	w.Header().Set("user_name", r.FormValue("user_name"))
	w.Header().Set("tenant_id", r.FormValue("tenant_id"))
	w.Header().Set("service_id", r.FormValue("service_id"))
	w.Header().Set("status", "failed")

	// 获取请求参数
	podName := r.FormValue("pod_name")
	p := r.FormValue("path")
	namespace := r.FormValue("namespace")
	fileName := strings.TrimSpace(chi.URLParam(r, "fileName"))
	containerName := r.FormValue("container_name")

	logrus.Debugf("下载文件参数: podName=%s, path=%s, namespace=%s, fileName=%s, containerName=%s",
		podName, p, namespace, fileName, containerName)

	if fileName == "" {
		logrus.Error("文件名为空")
		httputil.ReturnError(r, w, 400, "文件名不能为空")
		return
	}

	filePath := path.Join(p, fileName)
	logrus.Debugf("完整文件路径: %s", filePath)

	err := f.AppFileDownload(containerName, podName, filePath, namespace)
	if err != nil {
		logrus.Errorf("从Pod下载文件失败: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("下载文件失败: %v", err))
		return
	}

	defer func() {
		if err := os.Remove(path.Join("./", fileName)); err != nil {
			logrus.Errorf("删除临时文件失败: %v", err)
		}
	}()

	// 设置成功状态和文件下载头
	w.Header().Set("status", "success")
	w.Header().Set("Content-Disposition", "attachment;filename="+fileName)

	logrus.Debugf("开始传输文件: %s", fileName)
	http.ServeFile(w, r, fileName)
}

func (f FileManage) AppFileDownload(containerName, podName, filePath, namespace string) error {
	// Check if the file exists first
	checkCmd := []string{"test", "-e", filePath}
	req := f.clientset.CoreV1().RESTClient().Get().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   checkCmd,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(f.config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %v", err)
	}

	var checkStdout, checkStderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &checkStdout,
		Stderr: &checkStderr,
	})
	if err != nil {
		return fmt.Errorf("file not found: %s", filePath)
	}

	// Check if file is a directory
	isDirCmd := []string{"test", "-d", filePath}
	req = f.clientset.CoreV1().RESTClient().Get().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   isDirCmd,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err = remotecommand.NewSPDYExecutor(f.config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %v", err)
	}

	var isDirStdout, isDirStderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &isDirStdout,
		Stderr: &isDirStderr,
	})
	isDirectory := (err == nil)

	// For single files, try to use cat command (no tar dependency)
	if !isDirectory {
		return f.downloadFileUsingCat(containerName, podName, filePath, namespace)
	}

	// For directories, try tar first, fallback to cat if tar is not available
	err = f.downloadUsingTar(containerName, podName, filePath, namespace)
	if err != nil && strings.Contains(err.Error(), "tar") {
		logrus.Warnf("tar command not available, falling back to simple file copy: %v", err)
		return f.downloadFileUsingCat(containerName, podName, filePath, namespace)
	}
	return err
}

func (f FileManage) downloadFileUsingCat(containerName, podName, filePath, namespace string) error {
	req := f.clientset.CoreV1().RESTClient().Get().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   []string{"cat", filePath},
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(f.config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %v", err)
	}

	fileName := path.Base(filePath)
	destPath := path.Join("./", fileName)
	outFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %v", err)
	}
	defer outFile.Close()

	var stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: outFile,
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		return fmt.Errorf("failed to download file using cat: %v, stderr: %s", err, stderr.String())
	}

	logrus.Debugf("Successfully downloaded file %s using cat command", fileName)
	return nil
}

func (f FileManage) downloadUsingTar(containerName, podName, filePath, namespace string) error {
	reader, outStream := io.Pipe()

	req := f.clientset.CoreV1().RESTClient().Get().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   []string{"tar", "cf", "-", filePath},
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(f.config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %v", err)
	}

	go func() {
		defer outStream.Close()
		err = exec.Stream(remotecommand.StreamOptions{
			Stdin:  os.Stdin,
			Stdout: outStream,
			Stderr: os.Stderr,
			Tty:    false,
		})
		if err != nil {
			logrus.Errorf("stream error: %v", err)
		}
	}()

	prefix := getPrefix(filePath)
	prefix = path.Clean(prefix)
	destPath := path.Join("./", path.Base(prefix))
	err = unTarAll(reader, destPath, prefix)
	if err != nil {
		return fmt.Errorf("failed to untar: %v", err)
	}
	return nil
}

func (f FileManage) AppFileUpload(containerName, podName, srcPath, destPath, namespace string) error {
	logrus.Debugf("开始上传目录/文件: 源路径=%s, 目标路径=%s", srcPath, destPath)

	// 检查源路径是否存在
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("源路径检查失败: %v", err)
	}

	// For single files, use tee command (no tar dependency)
	if !srcInfo.IsDir() {
		return f.uploadFileUsingTee(containerName, podName, srcPath, destPath, namespace)
	}

	// For directories, try tar first, fallback to recursive tee if tar is not available
	err = f.uploadUsingTar(containerName, podName, srcPath, destPath, namespace)
	if err != nil && strings.Contains(err.Error(), "tar") {
		logrus.Warnf("tar command not available, falling back to recursive file copy: %v", err)
		return f.uploadDirectoryUsingTee(containerName, podName, srcPath, destPath, namespace)
	}
	return err
}

// uploadUsingTar uploads files/directories using tar command
func (f FileManage) uploadUsingTar(containerName, podName, srcPath, destPath, namespace string) error {
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("源路径检查失败: %v", err)
	}

	reader, writer := io.Pipe()

	// 在goroutine中处理tar打包
	go func() {
		defer writer.Close()

		// 如果是目录,使用完整目录路径
		if srcInfo.IsDir() {
			logrus.Debugf("正在打包目录: %s", srcPath)
			err = cpMakeTar(srcPath, destPath, writer)
		} else {
			logrus.Debugf("正在打包文件: %s", srcPath)
			err = cpMakeTar(filepath.Dir(srcPath), destPath, writer)
		}

		if err != nil {
			logrus.Errorf("tar打包失败: %v", err)
			writer.CloseWithError(err)
			return
		}
	}()

	// 构建在容器中解压的命令
	var cmdArr []string
	cmdArr = []string{"tar", "-xmf", "-"}

	// 确保目标路径存在
	if len(destPath) > 0 {
		// 先创建目标目录
		mkdirCmd := []string{"mkdir", "-p", destPath}
		mkdirReq := f.clientset.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(podName).
			Namespace(namespace).
			SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{
				Container: containerName,
				Command:   mkdirCmd,
				Stdin:     false,
				Stdout:    true,
				Stderr:    true,
				TTY:       false,
			}, scheme.ParameterCodec)

		mkdirExec, err := remotecommand.NewSPDYExecutor(f.config, "POST", mkdirReq.URL())
		if err != nil {
			return fmt.Errorf("创建目标目录执行器失败: %v", err)
		}

		if err := mkdirExec.Stream(remotecommand.StreamOptions{
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}); err != nil {
			return fmt.Errorf("在容器中创建目录失败: %v", err)
		}

		cmdArr = append(cmdArr, "-C", destPath)
	}

	// 执行解压命令
	req := f.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   cmdArr,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(f.config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("创建解压执行器失败: %v", err)
	}

	logrus.Debug("开始在容器中解压文件")
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  reader,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    false,
	})
	if err != nil {
		return fmt.Errorf("在容器中解压失败: %v", err)
	}

	logrus.Debug("文件/目录上传完成")
	return nil
}

// uploadDirectoryUsingTee recursively uploads directory using tee command (no tar dependency)
func (f FileManage) uploadDirectoryUsingTee(containerName, podName, srcPath, destPath, namespace string) error {
	logrus.Debugf("使用递归方式上传目录: %s -> %s", srcPath, destPath)

	return filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 获取相对路径
		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return fmt.Errorf("获取相对路径失败: %v", err)
		}

		// 构建目标路径
		var targetPath string
		if relPath == "." {
			targetPath = destPath
		} else {
			targetPath = filepath.Join(destPath, relPath)
		}

		// 如果是目录，创建目录
		if info.IsDir() {
			if relPath != "." {
				mkdirCmd := []string{"mkdir", "-p", targetPath}
				req := f.clientset.CoreV1().RESTClient().Post().
					Resource("pods").
					Name(podName).
					Namespace(namespace).
					SubResource("exec").
					VersionedParams(&corev1.PodExecOptions{
						Container: containerName,
						Command:   mkdirCmd,
						Stdin:     false,
						Stdout:    true,
						Stderr:    true,
						TTY:       false,
					}, scheme.ParameterCodec)

				exec, err := remotecommand.NewSPDYExecutor(f.config, "POST", req.URL())
				if err != nil {
					return fmt.Errorf("创建目录执行器失败: %v", err)
				}

				var stderr bytes.Buffer
				if err := exec.Stream(remotecommand.StreamOptions{
					Stdout: &bytes.Buffer{},
					Stderr: &stderr,
				}); err != nil {
					logrus.Warnf("创建目录警告: %v, stderr: %s", err, stderr.String())
				}
			}
			return nil
		}

		// 如果是文件，使用 tee 上传
		logrus.Debugf("上传文件: %s -> %s", path, targetPath)

		srcFile, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("打开源文件失败: %v", err)
		}
		defer srcFile.Close()

		teeCmd := []string{"tee", targetPath}
		req := f.clientset.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(podName).
			Namespace(namespace).
			SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{
				Container: containerName,
				Command:   teeCmd,
				Stdin:     true,
				Stdout:    true,
				Stderr:    true,
				TTY:       false,
			}, scheme.ParameterCodec)

		exec, err := remotecommand.NewSPDYExecutor(f.config, "POST", req.URL())
		if err != nil {
			return fmt.Errorf("创建 tee 执行器失败: %v", err)
		}

		var stderr bytes.Buffer
		err = exec.Stream(remotecommand.StreamOptions{
			Stdin:  srcFile,
			Stdout: &bytes.Buffer{},
			Stderr: &stderr,
			Tty:    false,
		})
		if err != nil {
			return fmt.Errorf("上传文件失败: %v, stderr: %s", err, stderr.String())
		}

		return nil
	})
}

// checkTarAvailable checks if tar command is available in the container
func (f FileManage) checkTarAvailable(containerName, podName, namespace string) bool {
	checkCmd := []string{"tar", "--version"}
	req := f.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   checkCmd,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(f.config, "POST", req.URL())
	if err != nil {
		return false
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	// If tar command exists, it should succeed or show version info
	return err == nil
}

// uploadFileUsingTee uploads a single file using tee command (no tar dependency)
func (f FileManage) uploadFileUsingTee(containerName, podName, srcPath, destPath, namespace string) error {
	// Read the source file
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer srcFile.Close()

	// Get filename from path
	fileName := filepath.Base(srcPath)

	// Build destination path
	var fullPath string
	if strings.HasSuffix(destPath, "/") || destPath == "" {
		fullPath = path.Join(destPath, fileName)
	} else {
		// Check if destPath is a directory in the container
		isDirCmd := []string{"test", "-d", destPath}
		isDirReq := f.clientset.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(podName).
			Namespace(namespace).
			SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{
				Container: containerName,
				Command:   isDirCmd,
				Stdin:     false,
				Stdout:    true,
				Stderr:    true,
				TTY:       false,
			}, scheme.ParameterCodec)

		isDirExec, err := remotecommand.NewSPDYExecutor(f.config, "POST", isDirReq.URL())
		if err != nil {
			return fmt.Errorf("failed to create directory check executor: %v", err)
		}

		var isDirStdout, isDirStderr bytes.Buffer
		err = isDirExec.Stream(remotecommand.StreamOptions{
			Stdout: &isDirStdout,
			Stderr: &isDirStderr,
		})

		// If test -d succeeds (err == nil), it's a directory
		if err == nil {
			fullPath = path.Join(destPath, fileName)
		} else {
			fullPath = destPath
		}
	}

	// Create parent directory if needed
	dirPath := path.Dir(fullPath)
	if dirPath != "." && dirPath != "/" {
		mkdirCmd := []string{"mkdir", "-p", dirPath}
		req := f.clientset.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(podName).
			Namespace(namespace).
			SubResource("exec").
			VersionedParams(&corev1.PodExecOptions{
				Container: containerName,
				Command:   mkdirCmd,
				Stdin:     false,
				Stdout:    true,
				Stderr:    true,
				TTY:       false,
			}, scheme.ParameterCodec)

		exec, err := remotecommand.NewSPDYExecutor(f.config, "POST", req.URL())
		if err != nil {
			return fmt.Errorf("failed to create mkdir executor: %v", err)
		}

		var stderr bytes.Buffer
		if err := exec.Stream(remotecommand.StreamOptions{
			Stdout: &bytes.Buffer{},
			Stderr: &stderr,
		}); err != nil {
			logrus.Warnf("mkdir warning: %v, stderr: %s", err, stderr.String())
		}
	}

	// Use tee command to write file
	teeCmd := []string{"tee", fullPath}
	req := f.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   teeCmd,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(f.config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create tee executor: %v", err)
	}

	var stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  srcFile,
		Stdout: &bytes.Buffer{},
		Stderr: &stderr,
		Tty:    false,
	})
	if err != nil {
		return fmt.Errorf("failed to upload file using tee: %v, stderr: %s", err, stderr.String())
	}

	logrus.Debugf("Successfully uploaded file %s using tee command", fileName)
	return nil
}

func cpMakeTar(srcPath string, destPath string, out io.Writer) error {
	tw := tar.NewWriter(out)
	defer tw.Close()

	// 获取源路径的信息
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("failed to get info for %s: %v", srcPath, err)
	}

	// 确定基础路径
	basePath := srcPath
	if srcInfo.IsDir() {
		// 如果是目录,使用目录本身作为基础路径
		basePath = srcPath
	} else {
		// 如果是文件,使用父目录作为基础路径
		basePath = filepath.Dir(srcPath)
	}

	return filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 获取相对路径
		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return err
		}

		// 如果是根目录,跳过
		if relPath == "." && srcInfo.IsDir() {
			return nil
		}

		// 创建header
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("failed to create header for %s: %v", path, err)
		}

		// 修改header名称,使用相对路径
		if srcInfo.IsDir() {
			// 如果源路径是目录,保持相对路径结构
			hdr.Name = relPath
		} else {
			// 如果源路径是文件,直接使用文件名
			hdr.Name = filepath.Base(srcPath)
		}

		if info.IsDir() {
			hdr.Name += "/"
		}

		logrus.Debugf("Adding to tar: %s", hdr.Name)

		// 写入header
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("failed to write header for %s: %v", path, err)
		}

		// 如果是普通文件,写入文件内容
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return fmt.Errorf("failed to write file %s: %v", path, err)
			}
		}

		return nil
	})
}

func unTarAll(reader io.Reader, destDir, prefix string) error {
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}

		if !strings.HasPrefix(header.Name, prefix) {
			return fmt.Errorf("tar contents corrupted")
		}

		mode := header.FileInfo().Mode()
		destFileName := filepath.Join(destDir, header.Name[len(prefix):])

		baseName := filepath.Dir(destFileName)
		if err := os.MkdirAll(baseName, 0755); err != nil {
			return err
		}
		if header.FileInfo().IsDir() {
			if err := os.MkdirAll(destFileName, 0755); err != nil {
				return err
			}
			continue
		}

		evaledPath, err := filepath.EvalSymlinks(baseName)
		if err != nil {
			return err
		}

		if mode&os.ModeSymlink != 0 {
			linkname := header.Linkname

			if !filepath.IsAbs(linkname) {
				_ = filepath.Join(evaledPath, linkname)
			}

			if err := os.Symlink(linkname, destFileName); err != nil {
				return err
			}
		} else {
			outFile, err := os.Create(destFileName)
			if err != nil {
				return err
			}
			defer outFile.Close()
			if _, err := io.Copy(outFile, tarReader); err != nil {
				return err
			}
			if err := outFile.Close(); err != nil {
				return err
			}
		}
	}

	return nil
}

func getPrefix(file string) string {
	return strings.TrimLeft(file, "/")
}

// CreateDirectory 在容器内创建目录
func (f FileManage) CreateDirectory(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("接收到创建目录请求: Method=%s, ContentType=%s", r.Method, r.Header.Get("Content-Type"))

	// 获取请求参数
	podName := r.FormValue("pod_name")
	namespace := r.FormValue("namespace")
	containerName := r.FormValue("container_name")
	dirPath := r.FormValue("path")

	logrus.Debugf("创建目录参数: podName=%s, namespace=%s, containerName=%s, dirPath=%s",
		podName, namespace, containerName, dirPath)

	if dirPath == "" {
		logrus.Error("目录路径为空")
		httputil.ReturnError(r, w, 400, "目录路径不能为空")
		return
	}

	// 构建在容器中创建目录的命令
	req := f.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   []string{"mkdir", "-p", dirPath},
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(f.config, "POST", req.URL())
	if err != nil {
		logrus.Errorf("创建目录执行器失败: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("创建目录失败: %v", err))
		return
	}

	// 创建缓冲区来捕获输出
	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})

	// 记录命令输出
	if stdout.Len() > 0 {
		logrus.Debugf("命令输出: %s", stdout.String())
	}
	if stderr.Len() > 0 {
		logrus.Debugf("错误输出: %s", stderr.String())
	}

	if err != nil {
		logrus.Errorf("在容器中创建目录失败: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("创建目录失败: %v", err))
		return
	}

	logrus.Debugf("目录创建成功: %s", dirPath)
	httputil.ReturnSuccess(r, w, nil)
}
