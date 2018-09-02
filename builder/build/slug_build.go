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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/Sirupsen/logrus"
	"github.com/fsnotify/fsnotify"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/pquerna/ffjson/ffjson"
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
	re.Logger.Info(util.Translation("Start compiling the source code"), map[string]string{"step": "build-exector"})
	s.tgzDir = re.TGZDir
	s.buildCacheDir = re.CacheDir
	packageName := fmt.Sprintf("%s/%s.tgz", s.tgzDir, re.DeployVersion)
	if err := s.runBuildContainer(re); err != nil {
		re.Logger.Error(util.Translation("Compiling the source code failure"), map[string]string{"step": "build-code", "status": "failure"})
		logrus.Error("build slug in container error,", err.Error())
		return nil, err
	}
	fileInfo, err := os.Stat(packageName)
	if err != nil {
		re.Logger.Error(util.Translation("Check that the build result failure"), map[string]string{"step": "build-code", "status": "failure"})
		logrus.Error("build package check error", err.Error())
		return nil, fmt.Errorf("build package failure")
	}
	if fileInfo.Size() == 0 {
		re.Logger.Error(util.Translation("Source build failure and result slug size is 0"),
			map[string]string{"step": "build-code", "status": "failure"})
		return nil, fmt.Errorf("build package failure")
	}
	re.Logger.Info(util.Translation("Compiling the source code SUCCESS"), map[string]string{"step": "build-code", "status": "success"})
	res := &Response{
		MediumType: "slug",
		MediumPath: packageName,
	}
	return res, nil
}

func (s *slugBuild) readLogFile(logfile string, logger event.Logger, closed chan struct{}) {
	file, _ := os.Open(logfile)
	watcher, _ := fsnotify.NewWatcher()
	defer watcher.Close()
	_ = watcher.Add(logfile)
	readerr := bufio.NewReader(file)
	for {
		line, _, err := readerr.ReadLine()
		if err != nil {
			if err != io.EOF {
				logrus.Errorf("Read build container log error:%s", err.Error())
				return
			}
			wait := func() error {
				for {
					select {
					case <-closed:
						return nil
					case event := <-watcher.Events:
						if event.Op&fsnotify.Write == fsnotify.Write {
							return nil
						}
					case err := <-watcher.Errors:
						return err
					}
				}
			}
			if err := wait(); err != nil {
				logrus.Errorf("Read build container log error:%s", err.Error())
				return
			}
		}
		if logger != nil {
			var message = make(map[string]string)
			if err := ffjson.Unmarshal(line, &message); err == nil {
				if m, ok := message["log"]; ok {
					logger.Info(m, map[string]string{"step": "build-exector"})
				}
			} else {
				fmt.Println(err.Error())
			}
		}
		select {
		case <-closed:
			return
		default:
		}
	}
}

func (s *slugBuild) getSourceCodeTarFile(re *Request) (*os.File, error) {
	var cmd []string
	sourceTarFile := fmt.Sprintf("%s/%s.tar", util.GetParentDirectory(re.SourceDir), re.DeployVersion)
	if re.ServerType == "svn" {
		cmd = append(cmd, "tar", "-cf", sourceTarFile, "--exclude=.svn", "./")
	}
	if re.ServerType == "git" {
		cmd = append(cmd, "tar", "-cf", sourceTarFile, "--exclude=.git", "./")
	}
	source := exec.Command(cmd[0], cmd[1:]...)
	source.Dir = re.SourceDir
	logrus.Debugf("tar source code to file %s", sourceTarFile)
	if err := source.Run(); err != nil {
		return nil, err
	}
	return os.OpenFile(sourceTarFile, os.O_RDONLY, 0755)
}

func (s *slugBuild) runBuildContainer(re *Request) error {
	envs := []*sources.KeyValue{
		&sources.KeyValue{Key: "SLUG_VERSION", Value: re.DeployVersion},
		&sources.KeyValue{Key: "SERVICE_ID", Value: re.ServiceID},
		&sources.KeyValue{Key: "TENANT_ID", Value: re.TenantID},
		&sources.KeyValue{Key: "LANGUAGE", Value: re.Lang.String()},
	}
	for k, v := range re.BuildEnvs {
		envs = append(envs, &sources.KeyValue{Key: k, Value: v})
		if k == "PROC_ENV" {
			var mapdata = make(map[string]interface{})
			if err := json.Unmarshal([]byte(v), &mapdata); err == nil {
				if runtime, ok := mapdata["runtimes"]; ok {
					envs = append(envs, &sources.KeyValue{Key: "RUNTIME", Value: runtime.(string)})
				}
			}
		}
	}
	containerConfig := &sources.ContainerConfig{
		Metadata: &sources.ContainerMetadata{
			Name: re.ServiceID[:8] + "_" + re.DeployVersion,
		},
		Image: &sources.ImageSpec{
			Image: builder.BUILDERIMAGENAME,
		},
		Mounts: []*sources.Mount{
			&sources.Mount{
				ContainerPath: "/tmp/cache",
				HostPath:      re.CacheDir,
				Readonly:      false,
			},
			&sources.Mount{
				ContainerPath: "/tmp/slug",
				HostPath:      s.tgzDir,
				Readonly:      false,
			},
		},
		Envs:         envs,
		Stdin:        true,
		StdinOnce:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		NetworkConfig: &sources.NetworkConfig{
			NetworkMode: "host",
		},
		Args: []string{"local"},
	}
	reader, err := s.getSourceCodeTarFile(re)
	if err != nil {
		return fmt.Errorf("create source code tar file error:%s", err.Error())
	}
	defer func() {
		reader.Close()
		os.Remove(reader.Name())
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	containerService := sources.CreateDockerService(ctx, re.DockerClient)
	containerID, err := containerService.CreateContainer(containerConfig)
	if err != nil {
		return fmt.Errorf("create builder container error:%s", err.Error())
	}
	errchan := make(chan error, 1)
	writer := re.Logger.GetWriter("builder", "info")
	close, err := containerService.AttachContainer(containerID, true, true, true, reader, writer, writer, &errchan)
	if err != nil {
		containerService.RemoveContainer(containerID)
		return fmt.Errorf("attach builder container error:%s", err.Error())
	}
	defer close()
	statuschan := containerService.WaitExitOrRemoved(containerID, false)
	//start the container
	if err := containerService.StartContainer(containerID); err != nil {
		containerService.RemoveContainer(containerID)
		return fmt.Errorf("start builder container error:%s", err.Error())
	}
	if err := <-errchan; err != nil {
		logrus.Debugf("Error hijack: %s", err)
	}
	status := <-statuschan
	if status != 0 {
		return &ErrorBuild{Code: status}
	}
	return nil
}

//ErrorBuild build error
type ErrorBuild struct {
	Code int
}

func (e *ErrorBuild) Error() string {
	return fmt.Sprintf("Run build return %d", e.Code)
}
