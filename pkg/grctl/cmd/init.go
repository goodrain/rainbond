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
	"os/exec"
	"net/http"
	"io/ioutil"
	"strings"
	"bytes"
	//"github.com/goodrain/rainbond/pkg/grctl/clients"
	"fmt"

	"time"
	"github.com/goodrain/rainbond/pkg/grctl/clients"
)

func NewCmdInit() cli.Command {
	c:=cli.Command{
		Name:  "init",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "etcd",
				Usage: "etcd ip,127.0.0.1",
			},
			cli.StringFlag{
				Name:  "type",
				Usage: "node type:manage/compute, manage",
			},
			cli.StringFlag{
				Name:  "mip",
				Usage: "当前节点内网IP, 10.0.0.1",
			},
			cli.StringFlag{
				Name:  "repo_ver",
				Usage: "repo version,3.4",
			},
			cli.StringFlag{
				Name:  "install_type",
				Usage: "online/offline ,online",
			},
		},
		Usage: "初始化集群。grctl init cluster",
		Action: func(c *cli.Context) error {
			return initCluster(c)
		},
	}
	return c
}
func NewCmdInstallStatus() cli.Command {
	c:=cli.Command{
		Name:  "install_status",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "taskID",
				Usage: "install_k8s,空则自动寻找",
			},

		},
		Usage: "获取task执行状态。grctl install_status",
		Action: func(c *cli.Context) error {
			taskID:=c.String("taskID")
			if taskID=="" {
				tasks,err:=clients.NodeClient.Tasks().List()
				if err != nil {
					logrus.Errorf("error get task list,details %s",err.Error())
					return nil
				}
				for _,v:=range tasks {
					for _,vs:=range v.Status{
						if  vs.Status=="start"||vs.Status=="create"{
							//Status(v.ID)
							return nil
						}

					}
				}
			}else {
				//Status(taskID)
			}
			return nil
		},
	}
	return c
}




func initCluster(c *cli.Context) error {
	resp, err := http.Get("http://repo.goodrain.com/gaops/jobs/install/prepare/init.sh")

	if err != nil {
		logrus.Errorf("error get init script,details %s",err.Error())
		return err
	}
	defer resp.Body.Close()

	b, _ := ioutil.ReadAll(resp.Body)
	args:=[]string{c.String("etcd"),c.String("type"),c.String("mip"),c.String("repo_ver"),c.String("install_type")}
	arg:=strings.Join(args," ")
	argCheck:=strings.Join(args,"")
	if len(argCheck) > 0 {
		arg+=";"
	}else {
		arg=""
	}

	fmt.Println("begin init cluster,please don't exit,wait install")
	cmd := exec.Command("bash", "-c",arg+string(b))
	buf:=bytes.NewBuffer(nil)
	cmd.Stderr=buf
	cmd.Run()
	out:=buf.String()
	arr:=strings.SplitN(out,"{",2)
	outJ:="{"+arr[1]
	jsonStr:=strings.TrimSpace(outJ)
	jsonStr=strings.Replace(jsonStr,"\n","",-1)
	jsonStr=strings.Replace(jsonStr," ","",-1)

	if strings.Contains(jsonStr, "Success") {
		fmt.Println("init success，start install")
	}else{
		fmt.Println("init failed！")
		return nil
	}
	time.Sleep(5*time.Second)

	Task(c,"check_manage_base_services",false)
	Task(c,"check_manage_services",false)

	fmt.Println("install manage node success,next you can :")
	fmt.Println("	add compute node--grctl node add -h")
	fmt.Println("	install compute node--grctl install compute -h")
	fmt.Println("	up compute node--grctl node up -h")
	return nil
}

