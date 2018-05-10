// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/goodrain/rainbond/grctl/clients"

)

func NewCmdExec() cli.Command {
	c:=cli.Command{
		Name:  "exec",
		Usage: "进入容器方法。grctl exec POD_NAME COMMAND ",
		Action: func(c *cli.Context) error {
			Common(c)
			return execContainer(c)
		},
	}
	return c
}

// grctl exec POD_ID COMMAND
func execContainer(c *cli.Context) error {
	//podID := c.Args().Get(1)
	args := c.Args().Tail()
	tenantID:=""


	podName := c.Args().First()
	//args := c.Args().Tail()
	//tenantID, err := clients.FindNamespaceByPod(podID)

	//clients.K8SClient.Core().Namespaces().Get("",metav1.GetOptions{}).


	kubeCtrl, err := exec.LookPath("kubectl")
	if err != nil {
		logrus.Error("Don't fnd the kubectl")
		return err
	}
	if len(args) == 0 {
		args = []string{"bash"}
	}
	pl,err:=clients.K8SClient.Core().Pods("").List(metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("error get pods by nil namespace")
		return err
	}
	for _,v:=range pl.Items {
		if v.Name == podName {
			tenantID=v.Namespace
			break
		}
	}
	defaultArgs := []string{kubeCtrl, "exec", "-it", "--namespace=" + tenantID,podName}
	args = append(defaultArgs, args...)
	//logrus.Info(args)
	cmd := exec.Cmd{
		Env:    os.Environ(),
		Path:   kubeCtrl,
		Args:   args,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	if err := cmd.Run(); err != nil {
		logrus.Error("Exec error.", err.Error())
		return err
	}
	return nil
}

