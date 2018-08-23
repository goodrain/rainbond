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
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/event"
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
	re.Logger.Info("开始编译代码包", map[string]string{"step": "build-exector"})
	s.tgzDir = fmt.Sprintf("/grdata/build/tenant/%s/slug/%s", re.TenantID, re.ServiceID)
	s.buildCacheDir = fmt.Sprintf("/cache/build/%s/cache/%s", re.TenantID, re.ServiceID)
	packageName := fmt.Sprintf("%s/%s.tgz", s.tgzDir, re.DeployVersion)
	logfile := fmt.Sprintf("/grdata/build/tenant/%s/slug/%s/%s.log",
		re.TenantID, re.ServiceID, re.DeployVersion)
	buildName := func(s, buildVersion string) string {
		mm := []byte(s)
		return string(mm[:8]) + "_" + buildVersion
	}(re.ServiceID, re.DeployVersion)
	cmd := []string{"build.pl",
		"-b", re.Branch,
		"-s", re.SourceDir,
		"-c", re.CacheDir,
		"-d", s.tgzDir,
		"-v", re.DeployVersion,
		"-l", logfile,
		"-tid", re.TenantID,
		"-sid", re.ServiceID,
		"-r", re.Runtime,
		"-g", re.Lang.String(),
		"-st", re.ServerType,
		"--name", buildName}
	if len(re.BuildEnvs) != 0 {
		buildEnvStr := ""
		mm := []string{}
		for k, v := range re.BuildEnvs {
			mm = append(mm, k+"="+v)
		}
		if len(mm) > 1 {
			buildEnvStr = strings.Join(mm, ":::")
		} else {
			buildEnvStr = mm[0]
		}
		cmd = append(cmd, "-e")
		cmd = append(cmd, buildEnvStr)
	}
	logrus.Debugf("source code build cmd:%s", cmd)
	if err := s.ShowExec("perl", cmd, re.Logger); err != nil {
		re.Logger.Error("编译代码包失败", map[string]string{"step": "build-code", "status": "failure"})
		logrus.Error("build perl error,", err.Error())
		return nil, err
	}
	re.Logger.Info("编译代码包完成。", map[string]string{"step": "build-code", "status": "success"})
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
	go s.readLog(stdout, logger)
	go s.readLog(stderr, logger)
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

func (s *slugBuild) readLog(stderr io.Reader, logger event.Logger) {
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
	}
}
