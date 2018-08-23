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

package build

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
)

func slugBuilder() (Build, error) {
	return &slugBuild{}, nil
}

type slugBuild struct {
	tgzDir        string
	buildCacheDir string
	sourceDir     string
}

func (s *slugBuild) Build(re *Request) (*Response, error) {
	//handle cache dir
	if _, ok := re.BuildEnvs["NO_CACHE"]; ok {
		os.RemoveAll(re.CacheDir)
	}
	re.Logger.Info("开始编译代码包", map[string]string{"step": "build-exector"})
	s.tgzDir = fmt.Sprintf("/grdata/build/tenant/%s/slug/%s", re.TenantID, re.ServiceID)
	s.buildCacheDir = fmt.Sprintf("/cache/build/%s/cache/%s", re.TenantID, re.ServiceID)
	packageName := fmt.Sprintf("%s/%s.tgz", s.tgzDir, re.DeployVersion)
	if err := s.buildInContainer(re); err != nil {
		re.Logger.Error("编译代码包失败", map[string]string{"step": "build-code", "status": "failure"})
		logrus.Error("build perl error,", err.Error())
		return nil, err
	}
	fileInfo, err := os.Stat(packageName)
	if err != nil {
		re.Logger.Error("构建代码包检测失败", map[string]string{"step": "build-code", "status": "failure"})
		logrus.Error("build package check error", err.Error())
		return nil, fmt.Errorf("build package failure")
	}
	if fileInfo.Size() == 0 {
		re.Logger.Error(fmt.Sprintf("构建失败！ 构建包大小为0 name：%s", packageName),
			map[string]string{"step": "build-code", "status": "failure"})
		return nil, fmt.Errorf("build package failure")
	}
	re.Logger.Info("代码构建完成", map[string]string{"step": "build-code", "status": "success"})
	res := &Response{
		MediumType: "slug",
		MediumPath: s.tgzDir,
	}
	return res, nil
}
func (s *slugBuild) buildInContainer(re *Request) error {
	dockerCmd := s.createCmd(re)
	sourceCmd := s.createSourceCmd(re)
	source := exec.Command(sourceCmd[0], sourceCmd[1:]...)
	read, err := source.StdoutPipe()
	if err != nil {
		return err
	}
	defer read.Close()
	cmd := exec.Command(dockerCmd[0], dockerCmd[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	cmd.Stdin = read
	closed := make(chan struct{})
	defer close(closed)
	go s.readLog(stdout, re.Logger, closed)
	go s.readLog(stderr, re.Logger, closed)
	if err := source.Start(); err != nil {
		if re.Logger != nil {
			re.Logger.Error(fmt.Sprintf("builder:Packaged source error"), map[string]string{"step": "build-exector"})
		}
		return fmt.Errorf("tar source dir error, %s", err.Error())
	}
	err = cmd.Start()
	if err != nil {
		if re.Logger != nil {
			re.Logger.Error(fmt.Sprintf("builder:%v", err), map[string]string{"step": "build-exector"})
		}
		logrus.Errorf("start build container error:%s", err.Error())
		return err
	}
	err = source.Wait()
	if err != nil {
		if re.Logger != nil {
			re.Logger.Error(fmt.Sprintf("builder:%v", err), map[string]string{"step": "build-exector"})
		}
		logrus.Errorf("wait build container error:%s", err.Error())
		return err
	}
	err = cmd.Wait()
	if err != nil {
		if re.Logger != nil {
			re.Logger.Error(fmt.Sprintf("builder:%v", err), map[string]string{"step": "build-exector"})
		}
		logrus.Errorf("wait build container error:%s", err.Error())
		return err
	}
	return nil
}
func (s *slugBuild) createSourceCmd(re *Request) []string {
	var cmd []string
	if re.ServerType == "svn" {
		cmd = append(cmd, "tar", "-c", "--exclude=.svn", re.SourceDir)
	}
	if re.ServerType == "git" {
		cmd = append(cmd, "tar", "-c", "--exclude=.git", re.SourceDir)
	}
	return cmd
}
func (s *slugBuild) createCmd(re *Request) []string {
	var cmd []string
	if ok, _ := util.FileExists("/var/run/docker.sock"); ok {
		cmd = append(cmd, "docker")
	} else {
		// not permissions
		cmd = append(cmd, "sudo", "-P", "docker")
	}
	cmd = append(cmd, "run", "-i", "--net=host", "--rm", "--name", re.ServiceID[:8]+"_"+re.DeployVersion)
	//handle cache mount
	cmd = append(cmd, "-v", re.CacheDir+":/tmp/cache:rw")
	cmd = append(cmd, "-v", s.tgzDir+":/tmp/slug:rw")
	//handle stdin and stdout
	cmd = append(cmd, "-a", "stdin", "-a", "stdout")
	//handle env
	for k, v := range re.BuildEnvs {
		if k != "" {
			cmd = append(cmd, "-e", fmt.Sprintf(`"%s=%s"`, k, v))
			if k == "PROC_ENV" {
				var mapdata = make(map[string]interface{})
				if err := json.Unmarshal(util.ToByte(v), &mapdata); err == nil {
					if runtime, ok := mapdata["runtimes"]; ok {
						cmd = append(cmd, "-e", fmt.Sprintf(`"%s=%s"`, "RUNTIME", runtime.(string)))
					}
				}
			}
		}
	}
	cmd = append(cmd, "-e", "SLUG_VERSION="+re.DeployVersion)
	cmd = append(cmd, "-e", "SERVICE_ID="+re.ServiceID)
	cmd = append(cmd, "-e", "TENANT_ID="+re.TenantID)
	cmd = append(cmd, "-e", "LANGUAGE="+re.Lang.String())
	//handle image
	cmd = append(cmd, "goodrain.me/builder", "local")
	return cmd
}
func (s *slugBuild) ShowExec(command string, params []string, logger event.Logger) error {
	cmd := exec.Command(command, params...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	closed := make(chan struct{})
	defer close(closed)
	go s.readLog(stdout, logger, closed)
	go s.readLog(stderr, logger, closed)
	err = cmd.Start()
	if err != nil {
		if logger != nil {
			logger.Error(fmt.Sprintf("builder:%v", err), map[string]string{"step": "build-exector"})
		}
		logrus.Errorf("start build container error:%s", err.Error())
		return err
	}
	err = cmd.Wait()
	if err != nil {
		if logger != nil {
			logger.Error(fmt.Sprintf("builder:%v", err), map[string]string{"step": "build-exector"})
		}
		logrus.Errorf("wait build container error:%s", err.Error())
		return err
	}
	return nil
}

func (s *slugBuild) readLog(stderr io.Reader, logger event.Logger, closed chan struct{}) {
	readerr := bufio.NewReader(stderr)
	for {
		line, _, err := readerr.ReadLine()
		if err != nil {
			if err != io.EOF {
				logrus.Errorf("Read build container log error:%s", err.Error())
			}
			return
		}
		if logger != nil {
			lineStr := string(line)
			if len(lineStr) > 0 {
				logger.Error(fmt.Sprintf("builder:%s", lineStr), map[string]string{"step": "build-exector"})
			}
		}
		select {
		case <-closed:
			return
		default:
		}
	}
}
