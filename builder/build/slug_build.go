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
	re.Logger.Info(util.Translation("Start compiling the source code"), map[string]string{"step": "build-exector"})
	s.tgzDir = fmt.Sprintf("/grdata/build/tenant/%s/slug/%s", re.TenantID, re.ServiceID)
	s.buildCacheDir = fmt.Sprintf("/cache/build/%s/cache/%s", re.TenantID, re.ServiceID)
	packageName := fmt.Sprintf("%s/%s.tgz", s.tgzDir, re.DeployVersion)
	if err := s.buildInContainer(re); err != nil {
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
func (s *slugBuild) buildInContainer(re *Request) error {
	dockerCmd := s.createCmd(re)
	logrus.Debugf("docker cmd:%s", dockerCmd)
	sourceCmd := s.createSourceCmd(re)
	logrus.Debugf("source cmd:%s", sourceCmd)
	source := exec.Command(sourceCmd[0], sourceCmd[1:]...)
	source.Dir = re.SourceDir
	cmd := exec.Command(dockerCmd[0], dockerCmd[1:]...)
	closed := make(chan struct{})
	defer close(closed)
	var b bytes.Buffer
	go s.readLog(&b, re.Logger, closed)
	commands, err := NewPipeCommand(source, cmd)
	if err != nil {
		re.Logger.Error("build failure:"+err.Error(), map[string]string{"step": "build-code", "status": "failure"})
		return fmt.Errorf("build in container error:%s", err.Error())
	}
	go s.readLog(commands.GetFinalStdout(), re.Logger, closed)
	go s.readLog(commands.GetFinalStderr(), re.Logger, closed)
	return commands.Run()
}
func (s *slugBuild) createSourceCmd(re *Request) []string {
	var cmd []string
	if re.ServerType == "svn" {
		cmd = append(cmd, "tar", "-c", "--exclude=.svn", "./")
	}
	if re.ServerType == "git" {
		cmd = append(cmd, "tar", "-c", "--exclude=.git", "./")
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

//PipeCommand PipeCommand
type PipeCommand struct {
	stack                    []*exec.Cmd
	finalStdout, finalStderr io.Reader
	pipestack                []*io.PipeWriter
}

//NewPipeCommand new pipe commands
func NewPipeCommand(stack ...*exec.Cmd) (*PipeCommand, error) {
	var errorbuffer bytes.Buffer
	pipestack := make([]*io.PipeWriter, len(stack)-1)
	i := 0
	for ; i < len(stack)-1; i++ {
		stdinpipe, stdoutpipe := io.Pipe()
		stack[i].Stdout = stdoutpipe
		stack[i].Stderr = &errorbuffer
		stack[i+1].Stdin = stdinpipe
		pipestack[i] = stdoutpipe
	}
	finalStdout, err := stack[i].StdoutPipe()
	if err != nil {
		return nil, err
	}
	finalStderr, err := stack[i].StderrPipe()
	if err != nil {
		return nil, err
	}
	pipeCommand := &PipeCommand{
		stack:       stack,
		pipestack:   pipestack,
		finalStdout: finalStdout,
		finalStderr: finalStderr,
	}
	return pipeCommand, nil
}

//Run Run
func (p *PipeCommand) Run() error {
	return call(p.stack, p.pipestack)
}

//GetFinalStdout get final command stdout reader
func (p *PipeCommand) GetFinalStdout() io.Reader {
	return p.finalStdout
}

//GetFinalStderr get final command stderr reader
func (p *PipeCommand) GetFinalStderr() io.Reader {
	return p.finalStderr
}

func call(stack []*exec.Cmd, pipes []*io.PipeWriter) (err error) {
	if stack[0].Process == nil {
		if err = stack[0].Start(); err != nil {
			return err
		}
	}
	if len(stack) > 1 {
		if err = stack[1].Start(); err != nil {
			return err
		}
		defer func() {
			if err == nil {
				pipes[0].Close()
				err = call(stack[1:], pipes[1:])
			}
		}()
	}
	return stack[0].Wait()
}
