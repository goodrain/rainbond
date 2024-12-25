package controller

import (
	"archive/tar"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

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
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/goodrain/rainbond/api/client/prometheus"

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
		w.Header().Add("Access-Control-Allow-Origin", origin)
		w.Header().Add("Access-Control-Allow-Methods", "POST,OPTIONS")
		w.Header().Add("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Access-Control-Allow-Headers", "x-requested-with,Content-Type,X-Custom-Header")
		httputil.ReturnSuccess(r, w, nil)

	case "OPTIONS":
		origin := r.Header.Get("Origin")
		w.Header().Add("Access-Control-Allow-Origin", origin)
		w.Header().Add("Access-Control-Allow-Methods", "POST,OPTIONS")
		w.Header().Add("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Access-Control-Allow-Headers", "x-requested-with,Content-Type,X-Custom-Header")
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

// Get get
func (f FileManage) Get(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	w.Header().Add("Access-Control-Allow-Origin", origin)
	w.Header().Add("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
	w.Header().Add("Access-Control-Allow-Credentials", "true")
	w.Header().Add("Access-Control-Allow-Headers", "x-requested-with,Content-Type,X-Custom-Header")

	if r.Method == "OPTIONS" {
		httputil.ReturnSuccess(r, w, nil)
		return
	}

	switch r.Method {
	case "GET":
		w.Header().Set("Content-Type", "application/octet-stream")
		f.DownloadFile(w, r)
	case "POST":
		f.UploadFile(w, r)
	default:
		httputil.ReturnError(r, w, 405, "Method not allowed")
	}
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
	origin := r.Header.Get("Origin")
	w.Header().Add("Access-Control-Allow-Origin", origin)
	w.Header().Add("Access-Control-Allow-Methods", "POST,OPTIONS")
	w.Header().Add("Access-Control-Allow-Credentials", "true")
	w.Header().Add("Access-Control-Allow-Headers", "x-requested-with,Content-Type,X-Custom-Header")

	if r.Method == "OPTIONS" {
		httputil.ReturnSuccess(r, w, nil)
		return
	}

	logrus.Debugf("开始处理文件上传请求，Method: %s, ContentType: %s", r.Method, r.Header.Get("Content-Type"))

	w.Header().Add("volume_name", r.FormValue("volume_name"))
	w.Header().Add("user_name", r.FormValue("user_name"))
	w.Header().Add("tenant_id", r.FormValue("tenant_id"))
	w.Header().Add("service_id", r.FormValue("service_id"))
	w.Header().Add("status", "failed")
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

	// 创建临时目录存储上传的文件
	tempDir, err := os.MkdirTemp("", "upload-*")
	if err != nil {
		logrus.Errorf("创建临时目录失败: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("创建临时目录失败: %v", err))
		return
	}
	defer os.RemoveAll(tempDir)

	// 解析上传的文件
	err = r.ParseMultipartForm(32 << 20) // 32MB
	if err != nil {
		logrus.Errorf("解析表单失败: %v", err)
		httputil.ReturnError(r, w, 400, fmt.Sprintf("解析表单失败: %v", err))
		return
	}

	// 尝试获取多文件上传
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		// 如果没有多文件，尝试获取单文件上传
		file, header, err := r.FormFile("file")
		if err != nil {
			logrus.Errorf("没有找到上传文件: %v", err)
			httputil.ReturnError(r, w, 400, "没有上传文件")
			return
		}
		defer file.Close()
		w.Header().Add("file_name", header.Filename)
		logrus.Debugf("处理单文件上传: %s", header.Filename)

		// 保存单个文件
		filePath := filepath.Join(tempDir, header.Filename)
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			logrus.Errorf("创建目录失败: %v", err)
			httputil.ReturnError(r, w, 500, fmt.Sprintf("创建目录失败: %v", err))
			return
		}

		dst, err := os.Create(filePath)
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
	} else {
		// 保存多个文件到临时目录
		logrus.Debugf("处理多文件上传，文件数量: %d", len(files))

		// 获取基础目录名称（如果有的话）
		var baseDir string
		if len(files) > 0 {
			// 从第一个文件的路径中提取基础目录
			firstPath := files[0].Filename
			if idx := strings.Index(firstPath, "/"); idx > 0 {
				baseDir = firstPath[:idx]
				logrus.Debugf("检测到基础目录: %s", baseDir)
			}
		}

		for _, fileHeader := range files {
			logrus.Debugf("处理文件: %s", fileHeader.Filename)
			file, err := fileHeader.Open()
			if err != nil {
				logrus.Errorf("打开上传文件失败: %v", err)
				httputil.ReturnError(r, w, 500, fmt.Sprintf("打开上传文件失败: %v", err))
				return
			}
			defer file.Close()

			// 构建完整的目标路径，保持原有的目录结构
			relativePath := fileHeader.Filename
			if baseDir != "" && strings.HasPrefix(relativePath, baseDir+"/") {
				// 如果文件在基础目录下，移除基础目录前缀
				relativePath = strings.TrimPrefix(relativePath, baseDir+"/")
			}

			filePath := filepath.Join(tempDir, relativePath)
			logrus.Debugf("创建文件: %s", filePath)

			// 确保目标目录存在
			targetDir := filepath.Dir(filePath)
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				logrus.Errorf("创建目录失败 %s: %v", targetDir, err)
				httputil.ReturnError(r, w, 500, fmt.Sprintf("创建目录失败: %v", err))
				return
			}

			dst, err := os.Create(filePath)
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
			logrus.Debugf("成功保存文件: %s", filePath)
		}
	}

	// 上传文件到容器
	logrus.Debugf("开始上传文件到容器")
	err = f.AppFileUpload(containerName, podName, tempDir, destPath, namespace)
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
	podName := r.FormValue("pod_name")
	p := r.FormValue("path")
	namespace := r.FormValue("namespace")
	fileName := strings.TrimSpace(chi.URLParam(r, "fileName"))

	filePath := path.Join(p, fileName)
	containerName := r.FormValue("container_name")

	err := f.AppFileDownload(containerName, podName, filePath, namespace)
	if err != nil {
		logrus.Errorf("downloading file from Pod failure: %v", err)
		http.Error(w, "Error downloading file from Pod", http.StatusInternalServerError)
		return
	}
	defer os.Remove(path.Join("./", fileName))
	w.Header().Set("Content-Disposition", "attachment;filename="+fileName)
	storage.Default().StorageCli.ServeFile(w, r, path.Join("./", fileName))
}

func (f FileManage) AppFileUpload(containerName, podName, srcPath, destPath, namespace string) error {
	reader, writer := io.Pipe()
	if destPath != "/" && strings.HasSuffix(string(destPath[len(destPath)-1]), "/") {
		destPath = destPath[:len(destPath)-1]
	}
	destPath = destPath + "/" + path.Base(srcPath)
	go func() {
		defer writer.Close()
		cmdutil.CheckErr(cpMakeTar(srcPath, destPath, writer))
	}()
	var cmdArr []string
	cmdArr = []string{"tar", "-xmf", "-"}
	destDir := path.Dir(destPath)
	if len(destDir) > 0 {
		cmdArr = append(cmdArr, "-C", destDir)
	}
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
		return err
	}
	return exec.Stream(remotecommand.StreamOptions{
		Stdin:  reader,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    false,
	})
}

func (f FileManage) AppFileDownload(containerName, podName, filePath, namespace string) error {
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
		return err
	}
	go func() {
		defer outStream.Close()
		err = exec.Stream(remotecommand.StreamOptions{
			Stdin:  os.Stdin,
			Stdout: outStream,
			Stderr: os.Stderr,
			Tty:    false,
		})
		cmdutil.CheckErr(err)
	}()
	prefix := getPrefix(filePath)
	prefix = path.Clean(prefix)
	destPath := path.Join("./", path.Base(prefix))
	err = unTarAll(reader, destPath, prefix)
	if err != nil {
		return err
	}
	return nil
}

func cpMakeTar(srcPath string, destPath string, out io.Writer) error {
	tw := tar.NewWriter(out)

	defer func() {
		if err := tw.Close(); err != nil {
			logrus.Errorf("Error closing tar writer: %v\n", err)
		}
	}()

	// 获取源路径的信息
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("failed to get info for %s: %v", srcPath, err)
	}

	basePath := srcPath
	if srcInfo.IsDir() {
		basePath = filepath.Dir(srcPath)
	}

	return filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 获取相对于源路径的相对路径
		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return err
		}

		// 创建 tar 归档文件的文件头信息
		hdr, err := tar.FileInfoHeader(info, relPath)
		if err != nil {
			return fmt.Errorf("failed to create header for %s: %v", path, err)
		}

		// 写入文件头信息到 tar 归档
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("failed to write header for %s: %v", path, err)
		}

		if info.Mode().IsRegular() {
			// 如果是普通文件，则将文件内容写入到 tar 归档
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open %s: %v", path, err)
			}
			defer file.Close()

			_, err = io.Copy(tw, file)
			if err != nil {
				return fmt.Errorf("failed to write file %s to tar: %v", path, err)
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
	origin := r.Header.Get("Origin")
	w.Header().Add("Access-Control-Allow-Origin", origin)
	w.Header().Add("Access-Control-Allow-Methods", "POST,OPTIONS")
	w.Header().Add("Access-Control-Allow-Credentials", "true")
	w.Header().Add("Access-Control-Allow-Headers", "x-requested-with,Content-Type,X-Custom-Header")

	if r.Method == "OPTIONS" {
		httputil.ReturnSuccess(r, w, nil)
		return
	}

	podName := r.FormValue("pod_name")
	namespace := r.FormValue("namespace")
	containerName := r.FormValue("container_name")
	dirPath := r.FormValue("path")

	if dirPath == "" {
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

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    false,
	})
	if err != nil {
		logrus.Errorf("在容器中创建目录失败: %v", err)
		httputil.ReturnError(r, w, 500, fmt.Sprintf("创建目录失败: %v", err))
		return
	}

	httputil.ReturnSuccess(r, w, nil)
}
