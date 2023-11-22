package exector

import (
	"fmt"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/event"
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
	err = sources.ImageBuild(v.Arch, sourcePath, "", "", "rbd-system", v.ServiceID, v.DeployVersion, v.Logger, "vm-build", imageName, v.BuildKitImage, v.BuildKitArgs, v.BuildKitCache, v.kubeClient)
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
	Logger.Info("begin down vm image "+url, map[string]string{"step": "builder-exector"})
	rsp, err := http.Get(url)
	defer func() {
		_ = rsp.Body.Close()
	}()

	downPath = path.Join(downPath, filepath.Base(url))
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
	_, err = io.Copy(f, rsp.Body)
	if err != nil {
		downError := fmt.Sprintf("download vm image %v failre: %v", url, err.Error())
		Logger.Error(downError, map[string]string{"step": "builder-exector", "status": "failure"})
	} else {
		Logger.Info("down vm image success", map[string]string{"step": "builder-exector"})
	}
	return err
}
