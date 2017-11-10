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

package cmd
import (
	"github.com/urfave/cli"
	"github.com/Sirupsen/logrus"
	"os"
	"os/exec"
	"github.com/goodrain/rainbond/cmd/grctl/option"
	"path"
	"strconv"
	"crypto/sha256"
	"fmt"
)

func NewCmdLog() cli.Command {
	c:=cli.Command{
		Name: "log",
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "f",
				Usage: "添加此参数日志持续输出。",
			},
		},
		Usage: "获取服务的日志。grctl log SERVICE_ID",
		Action: func(c *cli.Context) error {
			Common(c)
			return getLogInfo(c)
		},
	}
	return c
}
func GetServiceAliasID(ServiceID string) string {
	if len(ServiceID) > 11 {
		newWord := strconv.Itoa(int(ServiceID[10])) + ServiceID + strconv.Itoa(int(ServiceID[3])) + "log" + strconv.Itoa(int(ServiceID[2])/7)
		ha := sha256.New224()
		ha.Write([]byte(newWord))
		return fmt.Sprintf("%x", ha.Sum(nil))[0:16]
	}
	return ServiceID
}
// grctrl log SERVICE_ID
func getLogInfo(c *cli.Context) error {
	value := c.Args().Get(0)
	// tenantID, err := db.FindNamespaceByServiceID(value)
	// if err != nil {
	// 	logrus.Error(err.Error())
	// 	return err
	// }
	alias := GetServiceAliasID(value)
	config := option.GetConfig()
	logFilePath := path.Join(config.DockerLogPath, alias, "stdout.log")

	//logrus.Info(logFilePath)
	var cmd exec.Cmd

	if c.Bool("f") {
		tail, err := exec.LookPath("tail")
		if err != nil {
			logrus.Error("Don't find the tail.", err.Error())
			return err
		}
		cmd = exec.Cmd{
			Env:    os.Environ(),
			Path:   tail,
			Args:   []string{tail, "-f", logFilePath},
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}
	} else {
		cat, err := exec.LookPath("cat")
		if err != nil {
			logrus.Error("Don't find the cat.", err.Error())
			return err
		}
		cmd = exec.Cmd{
			Env:    os.Environ(),
			Path:   cat,
			Args:   []string{cat, logFilePath},
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}
	}
	if err := cmd.Run(); err != nil {
		logrus.Error("Log error.", err.Error())
		return err
	}
	return nil
}


