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





func initCluster(c *cli.Context) error {
	//done:=make(chan int)
	//go func(done chan int) {
	//	to := time.NewTimer(time.Second)
	//	for true  {
	//		select {
	//		case <-done:
	//			fmt.Println("安装完成")
	//		case <-to.C:
	//			fmt.Println("安装超时")
	//		}
	//	}
	//}(done)
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

	fmt.Println("开始初始化集群")
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
		fmt.Println("初始化成功，开始安装服务")
	}else{
		fmt.Println("初始化失败！")
		return nil
	}
	time.Sleep(5*time.Second)

	Task(c,"check_manage_base_services",false)
	Task(c,"check_manage_services",false)
	//err=clients.NodeClient.Tasks().Get("").Exec([]string{})
	//if err != nil {
	//	logrus.Errorf("error execute task %s","check_manage_base_services")
	//}
	//Status("check_manage_base_services")
	//
	//err=clients.NodeClient.Tasks().Get("check_manage_services").Exec([]string{})
	//if err != nil {
	//	logrus.Errorf("error execute task %s","check_manage_services")
	//}
	//Status("check_manage_services")
	//done<-1
	//一般 job会在通过grctl执行时阻塞输出，这种通过 脚本执行的，需要单独查
	return nil
}

