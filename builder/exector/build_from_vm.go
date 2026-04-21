package exector

import (
	"fmt"
	humanize "github.com/dustin/go-humanize"
	"github.com/goodrain/rainbond-operator/util/constants"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/builder/sourceutil"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	utils "github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"io"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type vmBuildMedia string

const (
	vmBuildMediaISO  vmBuildMedia = "iso"
	vmBuildMediaDisk vmBuildMedia = "disk"
)

var vmISODockerfileTmpl = `
FROM scratch
COPY --chown=107:107 ${VM_PATH} /disk/
`

var vmDiskDockerfileTmpl = `
FROM scratch
ADD --chown=107:107 ${VM_PATH} /disk/
`

// VMBuildItem -
type VMBuildItem struct {
	Logger        event.Logger `json:"logger"`
	Arch          string       `json:"arch"`
	VMImageSource string       `json:"vm_image_source"`
	ImageClient   sources.ImageClient
	Configs       map[string]gjson.Result `json:"configs"`
	ServiceID     string                  `json:"service_id"`
	DeployVersion string                  `json:"deploy_version"`
	Image         string                  `json:"image"`
	BuildKitImage string
	BuildKitArgs  []string
	BuildKitCache bool
	Action        string `json:"action"`
	EventID       string `json:"event_id"`
	TenantID      string `json:"tenant_id"`
	kubeClient    kubernetes.Interface
}

// NewVMBuildItem -
func NewVMBuildItem(in []byte) *VMBuildItem {
	eventID := gjson.GetBytes(in, "event_id").String()
	logger := event.GetManager().GetLogger(eventID)
	return &VMBuildItem{
		Logger:        logger,
		Arch:          gjson.GetBytes(in, "arch").String(),
		VMImageSource: gjson.GetBytes(in, "vm_image_source").String(),
		ServiceID:     gjson.GetBytes(in, "service_id").String(),
		DeployVersion: gjson.GetBytes(in, "deploy_version").String(),
		TenantID:      gjson.GetBytes(in, "tenant_id").String(),
		Configs:       gjson.GetBytes(in, "configs").Map(),
		Action:        gjson.GetBytes(in, "action").String(),
		EventID:       gjson.GetBytes(in, "event_id").String(),
		Image:         gjson.GetBytes(in, "image").String(),
	}
}

func (v *VMBuildItem) vmBuild(sourcePath string) error {
	fileInfoList, err := ioutil.ReadDir(sourcePath)
	if err != nil {
		return err
	}
	if len(fileInfoList) != 1 {
		return fmt.Errorf("%v file len is not 1", sourcePath)
	}
	logrus.Infof(
		"vm runtime image build context: service_id=%s event_id=%s source_path=%s file=%s",
		v.ServiceID,
		v.EventID,
		sourcePath,
		fileInfoList[0].Name(),
	)
	dockerfile, err := renderVMDockerfile(fileInfoList[0].Name())
	if err != nil {
		return err
	}

	dfpath := path.Join(sourcePath, "Dockerfile")
	logrus.Debugf("dest: %s; write dockerfile: %s", dfpath, dockerfile)
	err = ioutil.WriteFile(dfpath, []byte(dockerfile), 0755)
	if err != nil {
		return err
	}
	imageName := fmt.Sprintf("%v/%v", builder.REGISTRYDOMAIN, v.Image)
	err = sources.ImageBuild(v.Arch, sourcePath, utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace), v.ServiceID, v.DeployVersion, v.Logger, "vm-build", imageName, v.BuildKitImage, v.BuildKitArgs, v.BuildKitCache, v.kubeClient)
	if err != nil {
		v.Logger.Error(fmt.Sprintf("build image %s failure, find log in rbd-chaos", imageName), map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("build image error: %s", err.Error())
		return err
	}
	v.Logger.Info("push image to push local image registry success", map[string]string{"step": "builder-exector"})
	if err := v.ImageClient.ImageRemove(imageName); err != nil {
		logrus.Errorf("remove image %s failure %s", imageName, err.Error())
	}
	return nil
}

func renderVMDockerfile(fileName string) (string, error) {
	media, err := resolveVMBuildMedia(fileName)
	if err != nil {
		return "", err
	}
	envs := map[string]string{
		"VM_PATH": path.Join("./", fileName),
	}
	switch media {
	case vmBuildMediaISO:
		return strings.TrimPrefix(util.ParseVariable(vmISODockerfileTmpl, envs), "\n"), nil
	case vmBuildMediaDisk:
		return strings.TrimPrefix(util.ParseVariable(vmDiskDockerfileTmpl, envs), "\n"), nil
	default:
		return "", fmt.Errorf("unsupported vm build media %q", media)
	}
}

func resolveVMBuildMedia(fileName string) (vmBuildMedia, error) {
	name := strings.ToLower(strings.TrimSpace(fileName))
	for _, suffix := range []string{".gz", ".xz"} {
		if strings.HasSuffix(name, suffix) {
			name = strings.TrimSuffix(name, suffix)
			break
		}
	}
	switch {
	case strings.HasSuffix(name, ".iso"):
		return vmBuildMediaISO, nil
	case strings.HasSuffix(name, ".qcow2"), strings.HasSuffix(name, ".img"), strings.HasSuffix(name, ".tar"):
		return vmBuildMediaDisk, nil
	default:
		return "", fmt.Errorf("unsupported vm image format for %q", fileName)
	}
}

// RunVMBuild -
func (v *VMBuildItem) RunVMBuild() error {
	if sourceutil.IsLocalPackageSource(v.VMImageSource) {
		logrus.Infof("vm build uses local package source: service_id=%s event_id=%s source=%s", v.ServiceID, v.EventID, v.VMImageSource)
		if _, err := sourceutil.ReadLocalPackageDir(v.VMImageSource); err != nil {
			return err
		}
		defer os.RemoveAll(v.VMImageSource)
		return v.vmBuild(v.VMImageSource)
	}
	vmImageSource := fmt.Sprintf("/grdata/package_build/temp/events/%v", v.ServiceID)
	logrus.Infof("vm build downloads remote source: service_id=%s event_id=%s source=%s target_dir=%s", v.ServiceID, v.EventID, v.VMImageSource, vmImageSource)
	err := downloadFile(vmImageSource, v.VMImageSource, v.Logger)
	if err != nil {
		return err
	}
	defer os.RemoveAll(vmImageSource)
	return v.vmBuild(vmImageSource)
}

func downloadFile(downPath, url string, Logger event.Logger) error {
	// 创建一个 HTTP client 和 request
	client := sourceutil.NewRemotePackageHTTPClient(url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	// 添加请求头，例如设置 User-Agent
	req.Header.Set("User-Agent", "MyCustomDownloader/1.0")

	// 发送请求
	rsp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = rsp.Body.Close()
	}()
	logrus.Infof("vm source download response: url=%s status=%d content_length=%d", url, rsp.StatusCode, rsp.ContentLength)
	baseURL := filepath.Base(url)
	fileName := strings.Split(baseURL, "?")[0]
	downPath = path.Join(downPath, fileName)
	dir := filepath.Dir(downPath)
	// 递归创建目录
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	f, err := os.OpenFile(downPath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	myDownloader := &MyDownloader{
		Reader: rsp.Body,
		Total:  rsp.ContentLength,
		Logger: Logger,
		Pace:   10,
	}

	Logger.Info(fmt.Sprintf("begin download vm image %v, image name is %v", url, fileName), map[string]string{"step": "builder-exector"})
	Logger.Info(fmt.Sprintf("image size is %v, downloading will take some time, please be patient.", humanize.Bytes(uint64(rsp.ContentLength))), map[string]string{"step": "builder-exector"})

	_, err = io.Copy(f, myDownloader)
	if err != nil {
		downError := fmt.Sprintf("download vm image %v failure: %v", url, err.Error())
		Logger.Error(downError, map[string]string{"step": "builder-exector", "status": "failure"})
	}
	return err
}

type MyDownloader struct {
	io.Reader       // 读取器
	Total     int64 // 总大小
	Current   int64 // 当前大小
	Logger    event.Logger
	Pace      float64
}

func (d *MyDownloader) Read(p []byte) (n int, err error) {
	n, err = d.Reader.Read(p)
	d.Current += int64(n)
	if float64(d.Current*10000/d.Total)/100 == d.Pace {
		downLog := fmt.Sprintf("virtual machine image is being downloaded.current download progress is:%.2f%%", float64(d.Current*10000/d.Total)/100)
		d.Logger.Info(downLog, map[string]string{"step": "builder-exector"})
		d.Pace += 10
	}
	if float64(d.Current*10000/d.Total)/100 == 100 {
		d.Logger.Info("download vm image success", map[string]string{"step": "builder-exector"})
	}
	return
}
