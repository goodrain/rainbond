// RAINBOND, Application Management Platform
// Copyright (C) 2014-2019 Goodrain Co., Ltd.

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
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"

	"github.com/goodrain/rainbond/grctl/clients"
	"github.com/goodrain/rainbond/util"
	"github.com/urfave/cli"

	"github.com/goodrain/rainbond/node/nodem/client"
)

//NewCmdAnsible ansible config cmd
func NewCmdAnsible() cli.Command {
	c := cli.Command{
		Name:  "ansible",
		Usage: "Manage the ansible environment",
		Subcommands: []cli.Command{
			cli.Command{
				Name:  "hosts",
				Usage: "Manage the ansible hosts config environment",
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "hosts-file-path,p",
						Usage: "hosts file path",
						Value: "/opt/rainbond/rainbond-ansible/inventory/hosts",
					},
				},
				Action: func(c *cli.Context) error {
					Common(c)
					return WriteHostsFile(c)
				},
			},
		},
	}
	return c
}

//WriteHostsFile write hosts file
func WriteHostsFile(c *cli.Context) error {
	hosts, err := clients.RegionClient.Nodes().List()
	handleErr(err)
	config := GetAnsibleHostConfig(c.String("p"))
	for i := range hosts {
		config.AddHost(hosts[i])
	}
	return config.WriteFile()
}

//AnsibleHost  ansible host config
type AnsibleHost struct {
	AnsibleHostIP net.IP
	//ssh port
	AnsibleHostPort int
	HostID          string
	Role            client.HostRule
}

func (a *AnsibleHost) String() string {
	return fmt.Sprintf("id=%s ansible_host=%s ansible_port=%d ip=%s port=%d role=%s", a.HostID, a.AnsibleHostIP, a.AnsibleHostPort, a.AnsibleHostIP, a.AnsibleHostPort, a.Role)
}

//AnsibleHostGroup ansible host group config
type AnsibleHostGroup struct {
	Name     string
	HostList []*AnsibleHost
}

//AddHost add host
func (a *AnsibleHostGroup) AddHost(h *AnsibleHost) {
	for _, old := range a.HostList {
		if old.AnsibleHostIP.String() == h.AnsibleHostIP.String() {
			return
		}
	}
	a.HostList = append(a.HostList, h)
}
func (a *AnsibleHostGroup) String() string {
	rebuffer := bytes.NewBuffer(nil)
	rebuffer.WriteString(fmt.Sprintf("[%s]\n", a.Name))
	for i := range a.HostList {
		rebuffer.WriteString(a.HostList[i].String() + "\n")
	}
	return rebuffer.String()
}

//AnsibleHostConfig ansible hosts config
type AnsibleHostConfig struct {
	FileName  string
	GroupList map[string]*AnsibleHostGroup
}

//GetAnsibleHostConfig get config
func GetAnsibleHostConfig(name string) *AnsibleHostConfig {
	return &AnsibleHostConfig{
		FileName: name,
		GroupList: map[string]*AnsibleHostGroup{
			"all":         &AnsibleHostGroup{Name: "all"},
			"manage":      &AnsibleHostGroup{Name: "manage"},
			"new-manage":  &AnsibleHostGroup{Name: "new-manage"},
			"gateway":     &AnsibleHostGroup{Name: "gateway"},
			"new-gateway": &AnsibleHostGroup{Name: "new-gateway"},
			"compute":     &AnsibleHostGroup{Name: "compute"},
			"new-compute": &AnsibleHostGroup{Name: "new-compute"},
		},
	}
}

//Content return config file content
func (c *AnsibleHostConfig) Content() string {
	return c.ContentBuffer().String()
}

//ContentBuffer content buffer
func (c *AnsibleHostConfig) ContentBuffer() *bytes.Buffer {
	rebuffer := bytes.NewBuffer(nil)
	for i := range c.GroupList {
		rebuffer.WriteString(c.GroupList[i].String())
	}
	return rebuffer
}

//WriteFile write config file
func (c *AnsibleHostConfig) WriteFile() error {
	if c.FileName == "" {
		return fmt.Errorf("config file name can not be empty")
	}
	if err := util.CheckAndCreateDir(path.Dir(c.FileName)); err != nil {
		return err
	}
	if err := ioutil.WriteFile(c.FileName+".tmp", c.ContentBuffer().Bytes(), 0755); err != nil {
		return err
	}
	return os.Rename(c.FileName+".tmp", c.FileName)
}
func getSSHPort() int {
	return 22
}

//AddHost add host
func (c *AnsibleHostConfig) AddHost(h *client.HostNode) {
	//check role
	//check status
	ansibleHost := &AnsibleHost{
		AnsibleHostIP:   net.ParseIP(h.InternalIP),
		AnsibleHostPort: getSSHPort(),
		HostID:          h.ID,
		Role:            h.Role,
	}
	c.GroupList["all"].AddHost(ansibleHost)
	if h.Role.HasRule("manage") {
		if h.Status == client.NotInstalled || h.Status == client.InstallFailed {
			c.GroupList["new-manage"].AddHost(ansibleHost)
		} else {
			c.GroupList["manage"].AddHost(ansibleHost)
		}
	}
	if h.Role.HasRule("compute") {
		if h.Status == client.NotInstalled || h.Status == client.InstallFailed {
			c.GroupList["new-compute"].AddHost(ansibleHost)
		} else {
			c.GroupList["compute"].AddHost(ansibleHost)
		}
	}
	if h.Role.HasRule("gateway") {
		if h.Status == client.NotInstalled || h.Status == client.InstallFailed {
			c.GroupList["new-gateway"].AddHost(ansibleHost)
		} else {
			c.GroupList["gateway"].AddHost(ansibleHost)
		}
	}
}
