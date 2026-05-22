package exector

import (
	"fmt"
	humanize "github.com/dustin/go-humanize"
	"github.com/goodrain/rainbond-operator/util/constants"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/builder/sourceutil"
	"github.com/goodrain/rainbond/db"
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
	"time"
)

type vmBuildMedia string

const (
	vmBuildMediaISO  vmBuildMedia = "iso"
	vmBuildMediaDisk vmBuildMedia = "disk"
)

type vmDiskBuildStrategy string

const (
	vmDiskBuildStrategyDirect         vmDiskBuildStrategy = "direct"
	vmDiskBuildStrategyConvertRaw     vmDiskBuildStrategy = "convert-raw"
	vmDiskBuildStrategyConvertRawGzip vmDiskBuildStrategy = "convert-raw-gzip"
)

const defaultVMQCOW2ConverterImage = "quay.io/kubevirt/cdi-importer:v1.65.0"

var vmISODockerfileTmpl = `
FROM scratch
COPY --chown=107:107 ${VM_PATH} /disk/
`

var vmDiskDockerfileTmpl = `
FROM scratch
ADD --chown=107:107 ${VM_PATH} /disk/
`

var vmRawGzipToQCOW2DockerfileTmpl = `
FROM ${CONVERTER_IMAGE} AS convert
WORKDIR /work
COPY ${VM_PATH} /work/source.img.gz
RUN gzip -dc /work/source.img.gz > /work/source.img && /usr/bin/qemu-img convert -p -f raw -O qcow2 -c /work/source.img /work/rootdisk.qcow2 && rm -f /work/source.img /work/source.img.gz
FROM scratch
COPY --from=convert --chown=107:107 /work/rootdisk.qcow2 /disk/
`

var vmRawToQCOW2DockerfileTmpl = `
FROM ${CONVERTER_IMAGE} AS convert
WORKDIR /work
COPY ${VM_PATH} /work/source.img
RUN /usr/bin/qemu-img convert -p -f raw -O qcow2 -c /work/source.img /work/rootdisk.qcow2 && rm -f /work/source.img
FROM scratch
COPY --from=convert --chown=107:107 /work/rootdisk.qcow2 /disk/
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
	imageName := v.localImageName()
	err = sources.ImageBuild(v.Arch, sourcePath, utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace), v.ServiceID, v.DeployVersion, v.Logger, "vm-build", imageName, v.BuildKitImage, v.BuildKitArgs, v.BuildKitCache, v.kubeClient)
	if err != nil {
		v.Logger.Error(fmt.Sprintf("build image %s failure, find log in rbd-chaos", imageName), map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("build image error: %s", err.Error())
		return err
	}
	v.Logger.Info("push image to push local image registry success", map[string]string{"step": "builder-exector"})
	if err := v.storeVersionInfo(imageName); err != nil {
		logrus.Errorf("storage vm version info error: %s", err.Error())
		return err
	}
	if err := v.ImageClient.ImageRemove(imageName); err != nil {
		logrus.Errorf("remove image %s failure %s", imageName, err.Error())
	}
	return nil
}

func (v *VMBuildItem) localImageName() string {
	return fmt.Sprintf("%v/%v", builder.REGISTRYDOMAIN, v.Image)
}

func (v *VMBuildItem) storeVersionInfo(imageName string) error {
	version, err := db.GetManager().VersionInfoDao().GetVersionByDeployVersion(v.DeployVersion, v.ServiceID)
	if err != nil {
		return err
	}
	version.DeliveredType = "image"
	version.DeliveredPath = imageName
	if version.ImageName == "" {
		version.ImageName = v.Image
	}
	version.RepoURL = v.VMImageSource
	version.FinalStatus = "success"
	version.FinishTime = time.Now()
	return db.GetManager().VersionInfoDao().UpdateModel(version)
}

func (v *VMBuildItem) UpdateVersionInfo(status string) error {
	version, err := db.GetManager().VersionInfoDao().GetVersionByEventID(v.EventID)
	if err != nil {
		return err
	}
	version.FinalStatus = status
	version.RepoURL = v.VMImageSource
	version.FinishTime = time.Now()
	return db.GetManager().VersionInfoDao().UpdateModel(version)
}

func renderVMDockerfile(fileName string) (string, error) {
	media, err := resolveVMBuildMedia(fileName)
	if err != nil {
		return "", err
	}
	envs := map[string]string{
		"VM_PATH":         fileName,
		"CONVERTER_IMAGE": utils.GetenvDefault("VM_QCOW2_CONVERTER_IMAGE", defaultVMQCOW2ConverterImage),
	}
	switch media {
	case vmBuildMediaISO:
		return strings.TrimPrefix(util.ParseVariable(vmISODockerfileTmpl, envs), "\n"), nil
	case vmBuildMediaDisk:
		switch resolveVMDiskBuildStrategy(fileName) {
		case vmDiskBuildStrategyConvertRawGzip:
			return strings.TrimPrefix(util.ParseVariable(vmRawGzipToQCOW2DockerfileTmpl, envs), "\n"), nil
		case vmDiskBuildStrategyConvertRaw:
			return strings.TrimPrefix(util.ParseVariable(vmRawToQCOW2DockerfileTmpl, envs), "\n"), nil
		default:
			return strings.TrimPrefix(util.ParseVariable(vmDiskDockerfileTmpl, envs), "\n"), nil
		}
	default:
		return "", fmt.Errorf("unsupported vm build media %q", media)
	}
}

func resolveVMDiskBuildStrategy(fileName string) vmDiskBuildStrategy {
	name := strings.ToLower(strings.TrimSpace(fileName))
	switch {
	case strings.HasSuffix(name, ".img.gz"):
		return vmDiskBuildStrategyConvertRawGzip
	case strings.HasSuffix(name, ".img"):
		return vmDiskBuildStrategyConvertRaw
	default:
		return vmDiskBuildStrategyDirect
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
	if rsp.StatusCode < http.StatusOK || rsp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("download vm image %v failure: unexpected http status %d", url, rsp.StatusCode)
	}
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
	if rsp.ContentLength > 0 {
		Logger.Info(fmt.Sprintf("image size is %v, downloading will take some time, please be patient.", humanize.Bytes(uint64(rsp.ContentLength))), map[string]string{"step": "builder-exector"})
	} else {
		Logger.Info("image size is unknown, downloading will take some time, please be patient.", map[string]string{"step": "builder-exector"})
	}

	written, err := io.Copy(f, myDownloader)
	if err != nil {
		downError := fmt.Sprintf("download vm image %v failure: %v", url, err.Error())
		Logger.Error(downError, map[string]string{"step": "builder-exector", "status": "failure"})
		return err
	}
	Logger.Info(fmt.Sprintf("download vm image success, downloaded size is %v", humanize.Bytes(uint64(written))), map[string]string{"step": "builder-exector"})
	return nil
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
	if d.Total <= 0 {
		return
	}
	progress := float64(d.Current) * 100 / float64(d.Total)
	if progress >= d.Pace && d.Logger != nil {
		downLog := fmt.Sprintf("virtual machine image is being downloaded.current download progress is:%.2f%%", progress)
		d.Logger.Info(downLog, map[string]string{"step": "builder-exector"})
		d.Pace += 10
	}
	return
}
