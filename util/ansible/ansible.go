package ansible

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/goodrain/rainbond/node/nodem/client"
	"github.com/goodrain/rainbond/util"
)

//WriteHostsFile write hosts file
func WriteHostsFile(filePath, installConfPath string, hosts []*client.HostNode) error {
	if os.Getenv("NOT_WRITE_ANSIBLE_HOSTS") != "" {
		return nil
	}
	config := GetAnsibleHostConfig(filePath)
	for i := range hosts {
		config.AddHost(hosts[i], installConfPath)
	}
	return config.WriteFile()
}

//Host ansible host config
type Host struct {
	AnsibleHostIP net.IP
	//ssh port
	AnsibleHostPort          int
	HostID                   string
	Role                     client.HostRule
	CreateTime               time.Time
	AnsibleSSHPrivateKeyFile string
}

// String reutrn Host string
func (a *Host) String() string {
	if strings.TrimSpace(a.AnsibleSSHPrivateKeyFile) == "" {
		return fmt.Sprintf("%s ansible_host=%s ansible_port=%d ip=%s port=%d role=%s", a.HostID, a.AnsibleHostIP, a.AnsibleHostPort, a.AnsibleHostIP, a.AnsibleHostPort, a.Role)
	}
	return fmt.Sprintf("%s ansible_host=%s ansible_port=%d ip=%s port=%d role=%s ansible_ssh_private_key_file=%s", a.HostID, a.AnsibleHostIP, a.AnsibleHostPort, a.AnsibleHostIP, a.AnsibleHostPort, a.Role, a.AnsibleSSHPrivateKeyFile)
}

// HostsList hosts list
type HostsList []*Host

func (list HostsList) Len() int {
	return len(list)
}

func (list HostsList) Less(i, j int) bool {
	return list[i].CreateTime.Before(list[j].CreateTime)
}

func (list HostsList) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

//HostGroup ansible host group config
type HostGroup struct {
	Name     string
	HostList HostsList
}

//AddHost add host
func (a *HostGroup) AddHost(h *Host) {
	for _, old := range a.HostList {
		if old.AnsibleHostIP.String() == h.AnsibleHostIP.String() {
			return
		}
	}
	a.HostList = append(a.HostList, h)

}

// String return HostList string
func (a *HostGroup) String() string {
	rebuffer := bytes.NewBuffer(nil)
	rebuffer.WriteString(fmt.Sprintf("[%s]\n", a.Name))
	for i := range a.HostList {
		if a.Name == "all" {
			rebuffer.WriteString(a.HostList[i].String() + "\n")
		} else {
			rebuffer.WriteString(a.HostList[i].HostID + "\n")
		}
	}
	rebuffer.WriteString("\n")
	return rebuffer.String()
}

//HostConfig ansible hosts config
type HostConfig struct {
	FileName  string
	GroupList map[string]*HostGroup
}

//GetAnsibleHostConfig get config
func GetAnsibleHostConfig(name string) *HostConfig {
	return &HostConfig{
		FileName: name,
		GroupList: map[string]*HostGroup{
			"all":         &HostGroup{Name: "all"},
			"manage":      &HostGroup{Name: "manage"},
			"new-manage":  &HostGroup{Name: "new-manage"},
			"gateway":     &HostGroup{Name: "gateway"},
			"new-gateway": &HostGroup{Name: "new-gateway"},
			"compute":     &HostGroup{Name: "compute"},
			"new-compute": &HostGroup{Name: "new-compute"},
			"etcd":        &HostGroup{Name: "etcd"},
		},
	}
}

//Content return config file content
func (c *HostConfig) Content() string {
	return c.ContentBuffer().String()
}

//ContentBuffer content buffer
func (c *HostConfig) ContentBuffer() *bytes.Buffer {
	rebuffer := bytes.NewBuffer(nil)
	for i := range c.GroupList {
		sort.Sort(c.GroupList[i].HostList) // sort host by createTime
		rebuffer.WriteString(c.GroupList[i].String())
	}
	return rebuffer
}

//WriteFile write config file
func (c *HostConfig) WriteFile() error {
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

func getSSHPort(configFile string) int {
	if ok, _ := util.FileExists(configFile); !ok {
		return 22
	}
	file, err := os.OpenFile(configFile, os.O_RDONLY, 0666)
	if err != nil {
		return 22
	}

	defer file.Close()

	reader := bufio.NewReader(file)
	for {
		str, _, err := reader.ReadLine()
		if err != nil {
			break
		}
		line := strings.TrimSpace(string(str))
		if strings.Contains(line, "=") && !strings.HasPrefix(line, "#") {
			keyvalue := strings.SplitN(string(line), "=", 2)
			if len(keyvalue) == 2 || keyvalue[0] == "INSTALL_SSH_PORT" {
				port, err := strconv.Atoi(keyvalue[1])
				if err != nil {
					return 22
				}
				return port
			}
		}
	}
	return 22
}

//AddHost add host
func (c *HostConfig) AddHost(h *client.HostNode, installConfPath string) {
	//check role
	//check status
	ansibleHost := &Host{
		AnsibleHostIP:            net.ParseIP(h.InternalIP),
		AnsibleHostPort:          getSSHPort(installConfPath),
		HostID:                   h.ID,
		Role:                     h.Role,
		CreateTime:               h.CreateTime,
		AnsibleSSHPrivateKeyFile: h.KeyPath,
	}
	c.GroupList["all"].AddHost(ansibleHost)
	checkNeedInstall := func(h *client.HostNode) bool {
		return h.Status == client.NotInstalled || h.Status == client.InstallFailed || h.Status == client.Installing
	}
	if h.Role.HasRule("manage") {
		if checkNeedInstall(h) {
			c.GroupList["new-manage"].AddHost(ansibleHost)
		} else {
			c.GroupList["manage"].AddHost(ansibleHost)
		}
		if _, ok := h.Labels["noinstall_etcd"]; !ok {
			c.GroupList["etcd"].AddHost(ansibleHost)
		}
	}
	if h.Role.HasRule("compute") {
		if checkNeedInstall(h) {
			c.GroupList["new-compute"].AddHost(ansibleHost)
		} else {
			c.GroupList["compute"].AddHost(ansibleHost)
		}
	}
	if h.Role.HasRule("gateway") {
		if checkNeedInstall(h) {
			c.GroupList["new-gateway"].AddHost(ansibleHost)
		} else {
			c.GroupList["gateway"].AddHost(ansibleHost)
		}
	}
	for i := range h.NodeStatus.Conditions {
		if h.NodeStatus.Conditions[i].Type == "etcd" {
			c.GroupList["etcd"].AddHost(ansibleHost)
			break
		}
	}
}
