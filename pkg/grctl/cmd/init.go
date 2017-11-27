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
	//"runtime"
	"fmt"
	"github.com/bitly/go-simplejson"
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

// grctl exec POD_ID COMMAND
func initCluster(c *cli.Context) error {
	//logrus.Infof("start init command")
	resp, err := http.Get("http://repo.goodrain.com/gaops/jobs/install/prepare/init.sh")

	//参数
	//$1 -- ETCD_NODE  eg: 127.0.0.1 ETCD IP
	//$2 -- NODE_TYPE  eg: manage/compute 默认 manage
	//$3 -- MIP eg: 10.0.0.1 当前机器ip
	//$4 -- REPO_VER eg: 3.4 默认3.4
	//$5 -- INSTALL_TYPE eg: online 默认online
	//若不传参数则表示
	//
	//默认为管理节点 在线安装3.4版本的etcd
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
	//logrus.Infof("args is %s,len is %d",arg,len(arg))
	cmd := exec.Command("bash", "-c",arg+string(b))
	buf:=bytes.NewBuffer(nil)
	cmd.Stderr=buf
	cmd.Run()
	out:=buf.String()
	arr:=strings.SplitN(out,"{",2)
	arr[1]="{"+arr[1]
	json:=arr[1]
	fmt.Println(json)
	//j,err:=simplejson.NewJson([]byte(json))
	//if err != nil {
	//
	//}
	//etcd,err:=j.Get("global").Get("ETCD_ADDRS").String()

	//go func(c *exec.Cmd) {
	//	defer func() {
	//		if r := recover(); r != nil {
	//			const size = 64 << 10
	//			buf := make([]byte, size)
	//			buf = buf[:runtime.Stack(buf, false)]
	//			logrus.Warnf("panic running job: %v\n%s", r, buf)
	//		}
	//	}()
	//
	//}(cmd)
	return nil
}

