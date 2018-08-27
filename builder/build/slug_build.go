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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/builder/sources"
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
	re.Logger.Info(util.Translation("Start compiling the source code"), map[string]string{"step": "build-exector"})
	s.tgzDir = fmt.Sprintf("/grdata/build/tenant/%s/slug/%s", re.TenantID, re.ServiceID)
	s.buildCacheDir = fmt.Sprintf("/cache/build/%s/cache/%s", re.TenantID, re.ServiceID)
	packageName := fmt.Sprintf("%s/%s.tgz", s.tgzDir, re.DeployVersion)
	if err := s.runBuildContainer(re); err != nil {
		re.Logger.Error(util.Translation("Compiling the source code failure"), map[string]string{"step": "build-code", "status": "failure"})
		logrus.Error("build perl error,", err.Error())
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

// func (s *slugBuild) buildInContainer(re *Request) error {
// 	dockerCmd := s.createCmd(re)
// 	logrus.Debugf("docker cmd:%s", dockerCmd)
// 	sourceCmd := s.createSourceCmd(re)
// 	logrus.Debugf("source cmd:%s", sourceCmd)
// 	source := exec.Command(sourceCmd[0], sourceCmd[1:]...)
// 	source.Dir = re.SourceDir
// 	cmd := exec.Command(dockerCmd[0], dockerCmd[1:]...)
// 	closed := make(chan struct{})
// 	defer close(closed)
// 	var b bytes.Buffer
// 	go s.readLog(&b, re.Logger, closed)
// 	commands, err := util.NewPipeCommand(source, cmd)
// 	if err != nil {
// 		re.Logger.Error("build failure:"+err.Error(), map[string]string{"step": "build-code", "status": "failure"})
// 		return fmt.Errorf("build in container error:%s", err.Error())
// 	}
// 	go s.readLog(commands.GetFinalStdout(), re.Logger, closed)
// 	go s.readLog(commands.GetFinalStderr(), re.Logger, closed)
// 	return commands.Run()
// }
// func (s *slugBuild) createSourceCmd(re *Request) []string {
// 	var cmd []string
// 	if re.ServerType == "svn" {
// 		cmd = append(cmd, "tar", "-c", "--exclude=.svn", "./")
// 	}
// 	if re.ServerType == "git" {
// 		cmd = append(cmd, "tar", "-c", "--exclude=.git", "./")
// 	}
// 	return cmd
// }
// func (s *slugBuild) createCmd(re *Request) []string {
// 	var cmd []string
// 	if ok, _ := util.FileExists("/var/run/docker.sock"); ok {
// 		cmd = append(cmd, "docker")
// 	} else {
// 		// not permissions
// 		cmd = append(cmd, "sudo", "-P", "docker")
// 	}
// 	cmd = append(cmd, "run", "-i", "--net=host", "--rm", "--name", re.ServiceID[:8]+"_"+re.DeployVersion)
// 	//handle cache mount
// 	cmd = append(cmd, "-v", re.CacheDir+":/tmp/cache:rw")
// 	cmd = append(cmd, "-v", s.tgzDir+":/tmp/slug:rw")
// 	//handle stdin and stdout
// 	cmd = append(cmd, "-a", "stdin", "-a", "stdout")
// 	//handle env
// 	for k, v := range re.BuildEnvs {
// 		if k != "" {
// 			logrus.Debugf("%s=%s", k, strconv.Quote(v))
// 			cmd = append(cmd, "-e", fmt.Sprintf("%s=%s", k, strconv.Quote(v)))
// 			if k == "PROC_ENV" {
// 				var mapdata = make(map[string]interface{})
// 				if err := json.Unmarshal(util.ToByte(v), &mapdata); err == nil {
// 					if runtime, ok := mapdata["runtimes"]; ok {
// 						cmd = append(cmd, "-e", fmt.Sprintf("%s=%s", "RUNTIME", strconv.Quote(runtime.(string))))
// 					}
// 				}
// 			}
// 		}
// 	}
// 	cmd = append(cmd, "-e", "SLUG_VERSION="+re.DeployVersion)
// 	cmd = append(cmd, "-e", "SERVICE_ID="+re.ServiceID)
// 	cmd = append(cmd, "-e", "TENANT_ID="+re.TenantID)
// 	cmd = append(cmd, "-e", "LANGUAGE="+re.Lang.String())
// 	//handle image
// 	cmd = append(cmd, "goodrain.me/builder", "local")
// 	return cmd
// }

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
				logger.Error(lineStr, map[string]string{"step": "build-exector"})
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
	return os.OpenFile(sourceTarFile, os.O_WRONLY, 0755)
}

func (s *slugBuild) runBuildContainer(re *Request) error {
	builderImageName := os.Getenv("BUILDER_IMAGE_NAME")
	if builderImageName == "" {
		builderImageName = "goodrain.me/builder"
	}
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
			if err := json.Unmarshal(util.ToByte(v), &mapdata); err == nil {
				if runtime, ok := mapdata["runtimes"]; ok {
					envs = append(envs, &sources.KeyValue{Key: "RUNTIME", Value: strconv.Quote(runtime.(string))})
				}
			}
		}
	}
	containerConfig := &sources.ContainerConfig{
		Metadata: &sources.ContainerMetadata{
			Name: re.ServiceID[:8] + "_" + re.DeployVersion,
		},
		Image: &sources.ImageSpec{
			Image: builderImageName,
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
	buffer := bytes.NewBuffer(nil)
	closed := make(chan struct{})
	defer close(closed)
	go s.readLog(buffer, re.Logger, closed)
	errchan := make(chan error, 1)
	close, err := containerService.AttachContainer(containerID, true, true, true, reader, buffer, buffer, &errchan)
	if err != nil {
		containerService.RemoveContainer(containerID)
		return fmt.Errorf("attach builder container error:%s", err.Error())
	}
	defer close()
	statuschan := containerService.WaitExitOrRemoved(containerID, true)
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
