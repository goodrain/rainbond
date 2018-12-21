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
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/goodrain/rainbond/util"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/builder/sources"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/urfave/cli" //"github.com/goodrain/rainbond/grctl/clients"
)

//NewCmdInit grctl init
func NewCmdInit() cli.Command {
	c := cli.Command{
		Name: "init",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "role",
				Usage: "Node identity property",
				Value: "master,worker",
			},
			cli.StringFlag{
				Name:  "work_dir",
				Usage: "Installation configuration directory",
				Value: "/opt/rainbond/rainbond-ansible",
			},
			cli.StringFlag{
				Name:  "iip",
				Usage: "Internal IP",
				Value: "",
			},
			cli.StringFlag{
				Name:  "eip",
				Usage: "External IP",
				Value: "",
			},
			cli.StringFlag{
				Name:  "vip",
				Usage: "Virtual IP",
				Value: "",
			},
			cli.StringFlag{
				Name:  "rainbond-version",
				Usage: "Rainbond Install Version. default 5.0",
				Value: "5.0",
			},
			cli.StringFlag{
				Name:  "rainbond-repo",
				Usage: "Rainbond install repo",
				Value: "https://github.com/goodrain/rainbond-ansible.git",
			},
			cli.StringFlag{
				Name:  "install-type",
				Usage: "Install Type: online/offline",
				Value: "online",
			},
			cli.StringFlag{
				Name:  "deploy-type",
				Usage: "Deploy Type: onenode/multinode/thirdparty,默认onenode",
				Value: "onenode",
			},
			cli.StringFlag{
				Name:  "domain",
				Usage: "Application domain",
				Value: "",
			},
			cli.StringFlag{
				Name:  "pod-cidr",
				Usage: "Configuration pod-cidr",
				Value: "",
			},
			cli.StringFlag{
				Name:  "enable-feature",
				Usage: "New feature，disabled by default. default: windows",
				Value: "",
			},
			cli.StringFlag{
				Name:  "enable-online-images",
				Usage: "Get image online. default: offline",
				Value: "",
			},
			cli.StringFlag{
				Name:  "storage",
				Usage: "Storage type, default:NFS",
				Value: "nfs",
			},
			cli.StringFlag{
				Name:  "network",
				Usage: "Network type, support calico/flannel/midonet,default: calico",
				Value: "calico",
			},
			cli.StringFlag{
				Name:  "storage-args",
				Usage: "Stores mount parameters",
				Value: "/grdata nfs rw 0 0",
			},
			cli.StringFlag{
				Name:  "config-file,f",
				Usage: "Global Config Path, default",
				Value: "/opt/rainbond/rainbond-ansible/scripts/installer/global.sh",
			},
			cli.BoolFlag{
				Name:   "test",
				Usage:  "use test shell",
				Hidden: true,
			},
		},
		Usage: "grctl init cluster",
		Action: func(c *cli.Context) error {
			initCluster(c)
			return nil
		},
	}
	return c
}

//NewCmdInstallStatus install status
func NewCmdInstallStatus() cli.Command {
	c := cli.Command{
		Name: "install_status",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "taskID",
				Usage: "install_k8s,空则自动寻找",
			},
		},
		Usage: "获取task执行状态。grctl install_status",
		Action: func(c *cli.Context) error {
			taskID := c.String("taskID")
			if taskID == "" {
				tasks, err := clients.RegionClient.Tasks().List()
				if err != nil {
					logrus.Errorf("error get task list,details %s", err.Error())
					return nil
				}
				for _, v := range tasks {
					for _, vs := range v.Status {
						if vs.Status == "start" || vs.Status == "create" {
							//Status(v.ID)
							return nil
						}

					}
				}
			} else {
				//Status(taskID)
			}
			return nil
		},
	}
	return c
}
func updateConfigFile(path string, config map[string]string) error {
	initConfig := make(map[string]string)
	var file *os.File
	var err error
	if ok, _ := util.FileExists(path); ok {
		file, err = os.OpenFile(path, os.O_RDWR, 0755)
		if err != nil {
			return err
		}
		defer file.Close()
		reader := bufio.NewReader(file)
		for {
			line, _, err := reader.ReadLine()
			if err != nil {
				break
			}
			if strings.Contains(string(line), "=") {
				keyvalue := strings.SplitN(string(line), "=", 1)
				if len(keyvalue) < 2 {
					break
				}
				initConfig[keyvalue[0]] = keyvalue[1]
			}
		}
	} else {
		file, err = util.OpenOrCreateFile(path)
		if err != nil {
			return err
		}
		defer file.Close()
	}
	for k, v := range config {
		initConfig[k] = v
	}
	for k, v := range initConfig {
		if k == "" {
			continue
		}
		if v == "" {
			file.WriteString(fmt.Sprintf("%s=\"\"\n", k))
		} else {
			file.WriteString(fmt.Sprintf("%s=\"%s\"\n", k, v))
		}
	}
	return nil
}
func getConfig(c *cli.Context) map[string]string {
	configs := make(map[string]string)
	configs["ROLE"] = c.String("role")
	//configs["work_dir"] = c.String("work_dir")
	configs["IIP"] = c.String("iip")
	configs["EIP"] = c.String("eip")
	configs["VIP"] = c.String("vip")
	configs["VERSION"] = c.String("rainbond-version")
	// configs["rainbond-repo"] = c.String("rainbond-repo")
	configs["INSTALL_TYPE"] = c.String("install-type")
	configs["DEPLOY_TYPE"] = c.String("deploy-type")
	configs["DOMAIN"] = c.String("domain")
	configs["STORAGE"] = c.String("storage")
	configs["NETWORK_TYPE"] = c.String("network")
	configs["POD_NETWORK_CIDR"] = c.String("pod-cidr")
	configs["STORAGE_ARGS"] = c.String("storage-args")
	configs["PULL_ONLINE_IMAGES"] = c.String("enable-online-images")
	return configs
}
func initCluster(c *cli.Context) {
	// check if the rainbond is already installed
	//fmt.Println("Checking install enviremant.")
	_, err := os.Stat("/opt/rainbond/.rainbond.success")
	if err == nil {
		println("Rainbond is already installed, if you whant reinstall, then please delete the file: /opt/rainbond/.rainbond.success")
		return
	}
	// download source code from github if in online model
	if c.String("install-type") == "online" {
		fmt.Println("Download the installation configuration file remotely...")
		csi := sources.CodeSourceInfo{
			RepositoryURL: c.String("rainbond-repo"),
			Branch:        c.String("rainbond-version"),
		}
		os.RemoveAll(c.String("work_dir"))
		os.MkdirAll(c.String("work_dir"), 0755)
		_, err := sources.GitClone(csi, c.String("work_dir"), event.GetTestLogger(), 5)
		if err != nil {
			println(err.Error())
			return
		}
	}

	if err := updateConfigFile(c.String("config-file"), getConfig(c)); err != nil {
		showError("update config file failure " + err.Error())
	}

	//storage file
	//fmt.Println("Check storage type")
	//ioutil.WriteFile("/tmp/.storage.value", []byte(c.String("storage-args")), 0644)

	// start setup script to install rainbond
	fmt.Println("Initializes the installation of the first node...")
	cmd := exec.Command("bash", "-c", fmt.Sprintf("cd %s ; ./setup.sh", c.String("work_dir")))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err = cmd.Run()
	if err != nil {
		println(err.Error())
		return
	}

	ioutil.WriteFile("/opt/rainbond/.rainbond.success", []byte(c.String("rainbond-version")), 0644)

	return
}
