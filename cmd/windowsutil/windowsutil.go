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

package main

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/goodrain/rainbond/util/windows"

	"github.com/sirupsen/logrus"

	"github.com/goodrain/rainbond/cmd/windowsutil/option"
	"github.com/spf13/pflag"
)

func main() {
	conf := option.Config{}
	conf.AddFlags(pflag.CommandLine)
	pflag.Parse()
	if !conf.Check() {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	shell := strings.Split(conf.RunShell, "&nbsp;")
	logrus.Infof("run shell: %s", shell)
	cmd := exec.CommandContext(ctx, shell[0], shell[1:]...)
	startFunc := func() error {
		cmd.Stdin = os.Stdin
		reader, err := cmd.StdoutPipe()
		if err != nil {
			logrus.Errorf("open command stdout error %s", err.Error())
		}
		errReader, err := cmd.StderrPipe()
		if err != nil {
			logrus.Errorf("open command stderr error %s", err.Error())
		}
		go readBuffer(reader, logrus.Info)
		go readBuffer(errReader, logrus.Error)
		go func() {
			logrus.Info("start run progress")
			err := cmd.Start()
			if err != nil {
				logrus.Errorf("start cmd failure %s", err.Error())
				cancel()
			}
		}()
		var s os.Signal = syscall.SIGTERM
		defer func() {
			if cmd.Process != nil {
				if err := cmd.Process.Signal(s); err != nil {
					logrus.Errorf("send SIGTERM signal to progress failure %s", err.Error())
				}
				time.Sleep(time.Second * 2)
			}
		}()
		//step finally: listen Signal
		term := make(chan os.Signal)
		signal.Notify(term, os.Interrupt, syscall.SIGTERM)
		select {
		case ls := <-term:
			s = ls
			logrus.Warn("Received SIGTERM, exiting gracefully...")
		case <-ctx.Done():
		}
		logrus.Info("See you next time!")
		return nil
	}
	stopFunc := func() error {
		cancel()
		return nil
	}
	if conf.RunAsService {
		if err := windows.RunAsService(conf.ServiceName, startFunc, stopFunc, conf.Debug); err != nil {
			logrus.Fatalf("run command failure %s", err.Error())
		}
	} else {
		startFunc()
	}
}

func readBuffer(reader io.ReadCloser, print func(args ...interface{})) {
	defer reader.Close()
	bufreader := bufio.NewReader(reader)
	for {
		line, _, err := bufreader.ReadLine()
		if err != nil {
			if err == io.EOF {
				return
			}
			logrus.Errorf("read log buffer failure %s", err.Error())
			return
		}
		print(string(line))
	}
}
