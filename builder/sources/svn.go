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

package sources

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/event"
	"golang.org/x/net/context"
)

//SvnPull SvnPull
func SvnPull(dir, user, password string) error {
	cmd := exec.Command(
		"svn",
		"update",
		"--ignore-externals",
		"--username",
		user,
		"--password",
		password)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Errorf("Failed to SVN update %s, see output below\n%sContinuing...", dir, out)
		return err
	}
	return nil
}

//SvnClone clone code by svn
func SvnClone(dir, url, user, password string, logger event.Logger, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	par, rep := filepath.Split(dir)
	cmd := exec.Command(
		"svn",
		"checkout",
		"--non-interactive",
		"--trust-server-cert-failures=unknown-ca",
		"--username",
		user,
		"--password",
		password,
		url,
		rep)
	cmd.Dir = par
	reader, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	readererr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	startReadProgress(ctx, reader, logger)
	startReadProgress(ctx, readererr, logger)
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return "", nil
}

//startReadProgress create svn log progress
func startReadProgress(ctx context.Context, read io.ReadCloser, logger event.Logger) {
	var reader = bufio.NewReader(read)
	go func() {
		defer read.Close()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				line, _, err := reader.ReadLine()
				if err != nil {
					if err.Error() != "EOF" {
						fmt.Println("read svn log err", err.Error())
					}
					return
				}
				if len(line) > 0 {
					progess := strings.Replace(string(line), "\r", "", -1)
					progess = strings.Replace(progess, "\n", "", -1)
					progess = strings.Replace(progess, "\u0000", "", -1)
					if len(progess) > 0 {
						message := fmt.Sprintf(`{"progress":"%s","id":"%s"}`, progess, "SVN:")
						logger.Debug(message, map[string]string{"step": "progress"})
					}
				}
			}
		}
	}()
}
