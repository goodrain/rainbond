// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

package exector

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/goodrain/rainbond/pkg/db"
	"github.com/goodrain/rainbond/pkg/event"

	"github.com/pquerna/ffjson/ffjson"

	"github.com/goodrain/rainbond/pkg/builder/model"

	"github.com/Sirupsen/logrus"
	"github.com/akkuman/parseConfig"
)

const (
	cloneTimeout    = 60
	buildingTimeout = 180
	formatSourceDir = "/cache/build/%s/source/%s"
)

func (e *exectorManager) pluginDockerfileBuild(in []byte) {
	config := getConf(configPath)
	var tb model.BuildPluginTaskBody
	if err := ffjson.Unmarshal(in, &tb); err != nil {
		logrus.Errorf("unmarshal taskbody error, %v", err)
		return
	}
	eventID := tb.EventID
	logger := event.GetManager().GetLogger(eventID)
	logger.Info("从dockerfile构建插件任务开始执行", map[string]string{"step": "builder-exector", "status": "starting"})

	go func() {
		time.Sleep(buildingTimeout * time.Second)
		logrus.Debugf("building plugin time-out time is reach")
		version, err := db.GetManager().TenantPluginBuildVersionDao().GetBuildVersionByVersionID(tb.PluginID, tb.VersionID)
		if err != nil {
			logrus.Errorf("get version error, %v", err)
		}
		if version.Status != "complete" {
			version.Status = "timeout"
			if err := db.GetManager().TenantPluginBuildVersionDao().UpdateModel(version); err != nil {
				logrus.Errorf("update version error, %v", err)
			}
			logger.Error("插件构建超时，修改插件状态失败", map[string]string{"step": "callback", "status": "failure"})
		}
	}()
	go func() {
		logrus.Info("start exec build plugin from image worker")
		defer event.GetManager().ReleaseLogger(logger)
		for retry := 0; retry < 3; retry++ {
			err := e.runD(&tb, config, logger)
			if err != nil {
				logrus.Errorf("exec plugin build from dockerfile error:%s", err.Error())
				logger.Info("dockerfile构建插件任务执行失败，开始重试", map[string]string{"step": "builder-exector", "status": "failure"})
			} else {
				return
			}
		}
		version, err := db.GetManager().TenantPluginBuildVersionDao().GetBuildVersionByVersionID(tb.PluginID, tb.VersionID)
		if err != nil {
			logrus.Errorf("get version error, %v", err)
		}
		version.Status = "failure"
		if err := db.GetManager().TenantPluginBuildVersionDao().UpdateModel(version); err != nil {
			logrus.Errorf("update version error, %v", err)
		}
		logger.Error("dockerfile构建插件任务执行失败", map[string]string{"step": "callback", "status": "failure"})
	}()
}

func (e *exectorManager) runD(t *model.BuildPluginTaskBody, c parseConfig.Config, logger event.Logger) error {
	logger.Info("开始拉取代码", map[string]string{"step": "build-exector"})
	logrus.Debugf("开始拉取代码")
	sourceDir := fmt.Sprintf(formatSourceDir, t.TenantID, t.VersionID)
	if t.Repo == "" {
		t.Repo = "master"
	}
	if err := clone(t.GitURL, sourceDir, logger, t.Repo); err != nil {
		logger.Error("拉取代码失败", map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Errorf("拉取代码失败，%v", err)
		return err
	}
	if !checkDockerfile(sourceDir) {
		logger.Error("代码未检测到dockerfile，暂不支持构建，任务即将退出", map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Error("代码未检测到dockerfile")
		return fmt.Errorf("have no dockerfile")
	}

	logger.Info("代码检测为dockerfile，开始编译", map[string]string{"step": "build-exector"})
	curImage, err := buildImage(t.VersionID, t.GitURL, sourceDir, c.Get("publish > image > curr_registry").(string), logger)
	if err != nil {
		logrus.Errorf("build error, %v", err)
		return err
	}
	logger.Info(fmt.Sprintf("镜像编译完成，开始推送镜像，镜像名为 %s", curImage), map[string]string{"step": "build-exector"})

	if err := push(curImage, logger); err != nil {
		logger.Error("推送镜像失败", map[string]string{"step": "builder-exector", "status": "failure"})
		logrus.Error("推送镜像失败")
		return err
	}
	//TODO: 权限
	//if err := e.DockerPush(curImage); err != nil {
	//	logger.Info("推送镜像失败", map[string]string{"step": "builder-exector", "status": "failure"})
	//	return err
	//}

	logger.Info("推送镜像完成", map[string]string{"step": "build-exector"})
	version, err := db.GetManager().TenantPluginBuildVersionDao().GetBuildVersionByVersionID(t.PluginID, t.VersionID)
	if err != nil {
		logrus.Errorf("get version error, %v", err)
		return err
	}
	version.BuildLocalImage = curImage
	version.Status = "complete"
	if err := db.GetManager().TenantPluginBuildVersionDao().UpdateModel(version); err != nil {
		logrus.Errorf("update version error, %v", err)
		return err
	}
	logger.Info("从dockerfile构建插件完成", map[string]string{"step": "last", "status": "success"})
	return nil
}

func clone(gitURL string, sourceDir string, logger event.Logger, repo string) error {
	path := fmt.Sprintf("%s/.git/config", sourceDir)
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			logrus.Debugf("clone: %s", fmt.Sprintf("git clone -b %s %s %s", repo, gitURL, sourceDir))
			mm := []string{"clone", "-b", repo, gitURL, sourceDir}
			if err := ShowExec("git", mm, logger); err != nil {
				return err
			}
		} else {
			logrus.Debugf("file check error: %v", err)
			return err
		}
	} else {
		logrus.Debugf("pull: %s", fmt.Sprintf("sudo -P git -C %s pull", sourceDir))
		mm := []string{"-C", sourceDir, "pull"}
		if err := ShowExec("git", mm, logger); err != nil {
			return err
		}
	}
	return nil
}

func gitclone(gitURL string, sourceDir string, logger event.Logger, repo string) error {
	path := fmt.Sprintf("%s/.git/config", sourceDir)
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			logrus.Debugf("clone: %s", fmt.Sprintf("git clone -b %s %s %s", repo, gitURL, sourceDir))
			mm := []string{"-P", "git", "clone", "-b", repo, gitURL, sourceDir}
			cmd := exec.Command("sudo", mm...)
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				logrus.Errorf(fmt.Sprintf("builder err: %v", err))
				return err
			}
			errC := cmd.Start()
			if errC != nil {
				logger.Error(fmt.Sprintf("builder:%v", errC), map[string]string{"step": "build-exector"})
				return errC
			}
			reader := bufio.NewReader(stdout)
			go func() {
				for {
					line, errL := reader.ReadString('\n')
					if errL != nil || io.EOF == errL {
						break
					}
					//fmt.Print(line)
					logger.Debug(fmt.Sprintf("builder:%v", line), map[string]string{"step": "build-exector"})
				}
			}()
			errW := cmd.Wait()
			logrus.Debugf("errw is %v", errW)
			if errW != nil {
				cierr := strings.Split(errW.Error(), "\n")
				if strings.Contains(errW.Error(), "Cloning into") && len(cierr) < 3 {
					logrus.Errorf(fmt.Sprintf("builder:%v", errW))
					logger.Error(fmt.Sprintf("builder:%v", errW), map[string]string{"step": "build-exector"})
					return errW
				}
			}
			return nil
		}
		logrus.Debugf("file check error: %v", err)
		return err
	}
	logrus.Debugf("pull: %s", fmt.Sprintf("sudo -P git -C %s pull", sourceDir))
	mm := []string{"-P", "git", "-C", sourceDir, "pull"}
	if err := ShowExec("sudo", mm, logger); err != nil {
		return err
	}
	return nil
}

func checkDockerfile(sourceDir string) bool {
	if _, err := os.Stat(fmt.Sprintf("%s/Dockerfile", sourceDir)); os.IsNotExist(err) {
		return false
	}
	return true
}

func buildImage(version, gitURL, sourceDir, curRegistry string, logger event.Logger) (string, error) {
	mm := strings.Split(gitURL, "/")
	n1 := strings.Split(mm[len(mm)-1], ".")[0]
	imageName := fmt.Sprintf("%s/%s_%s", curRegistry, n1, version)
	//imagename must be lower
	logrus.Debugf("image name is %v", imageName)
	if os.Getenv("NO_CACHE") == "" {
		mm := []string{"-P", "docker", "build", "-t", imageName, "--no-cache", sourceDir}
		logrus.Debugf("build image: sudo -P docker build -t %s --no-cache %s", imageName, sourceDir)
		if err := ShowExec("sudo", mm, logger); err != nil {
			return "", err
		}
	} else {
		mm := []string{"-P", "docker", "build", "-t", imageName, sourceDir}
		logrus.Debugf("build image: sudo -P docker build -t %s %s", imageName, sourceDir)
		if err := ShowExec("sudo", mm, logger); err != nil {
			return "", err
		}
	}
	return imageName, nil
}
