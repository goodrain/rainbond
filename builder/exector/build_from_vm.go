package exector

import (
	"fmt"
	humanize "github.com/dustin/go-humanize"
	"github.com/goodrain/rainbond-operator/util/constants"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/monitor/utils"
	"github.com/goodrain/rainbond/util"
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

var vmDockerfileTmpl = `
FROM scratch
ADD ${VM_PATH} /disk/
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
	envs := make(map[string]string)
	fileInfoList, err := ioutil.ReadDir(sourcePath)
	if err != nil {
		return err
	}
	if len(fileInfoList) != 1 {
		return fmt.Errorf("%v file len is not 1", sourcePath)
	}
	envs["VM_PATH"] = path.Join("./", fileInfoList[0].Name())
	dockerfile := util.ParseVariable(vmDockerfileTmpl, envs)

	dfpath := path.Join(sourcePath, "Dockerfile")
	logrus.Debugf("dest: %s; write dockerfile: %s", dfpath, dockerfile)
	err = ioutil.WriteFile(dfpath, []byte(dockerfile), 0755)
	if err != nil {
		return err
	}
	imageName := fmt.Sprintf("%v/%v", builder.REGISTRYDOMAIN, v.Image)
	err = sources.ImageBuild(v.Arch, sourcePath, "", "", utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace), v.ServiceID, v.DeployVersion, v.Logger, "vm-build", imageName, v.BuildKitImage, v.BuildKitArgs, v.BuildKitCache, v.kubeClient)
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

// RunVMBuild -
func (v *VMBuildItem) RunVMBuild() error {
	if strings.HasPrefix(v.VMImageSource, "/grdata") {
		defer os.RemoveAll(v.VMImageSource)
		return v.vmBuild(v.VMImageSource)
	}
	vmImageSource := fmt.Sprintf("/grdata/package_build/temp/events/%v", v.ServiceID)
	err := downloadFile(vmImageSource, v.VMImageSource, v.Logger)
	if err != nil {
		return err
	}
	defer os.RemoveAll(vmImageSource)
	return v.vmBuild(vmImageSource)
}

func downloadFile(downPath, url string, Logger event.Logger) error {
	rsp, err := http.Get(url)
	defer func() {
		_ = rsp.Body.Close()
	}()
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
		downError := fmt.Sprintf("download vm image %v failre: %v", url, err.Error())
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
