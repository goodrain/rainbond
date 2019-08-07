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
	config := GetAnsibleHostConfig(filePath)
	for i := range hosts {
		config.AddHost(hosts[i], installConfPath)
	}
	return config.WriteFile()
}

//AnsibleHost ansible host config
type AnsibleHost struct {
	AnsibleHostIP net.IP
	//ssh port
	AnsibleHostPort int
	HostID          string
	Role            client.HostRule
	CreateTime      time.Time
}

func (a *AnsibleHost) String() string {
	return fmt.Sprintf("%s ansible_host=%s ansible_port=%d ip=%s port=%d role=%s", a.HostID, a.AnsibleHostIP, a.AnsibleHostPort, a.AnsibleHostIP, a.AnsibleHostPort, a.Role)
}

type HostsList []*AnsibleHost

func (list HostsList) Len() int {
	return len(list)
}

func (list HostsList) Less(i, j int) bool {
	return list[i].CreateTime.Before(list[j].CreateTime)
}

func (list HostsList) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

//AnsibleHostGroup ansible host group config
type AnsibleHostGroup struct {
	Name     string
	HostList HostsList
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
		if a.Name == "all" {
			rebuffer.WriteString(a.HostList[i].String() + "\n")
		} else if a.Name == "etcd" {
			rebuffer.WriteString(a.HostList[i].HostID + "\n")
		} else {
			rebuffer.WriteString(a.HostList[i].HostID + "\n")
		}
	}
	rebuffer.WriteString("\n")
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
			"etcd":        &AnsibleHostGroup{Name: "etcd"},
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
		sort.Sort(c.GroupList[i].HostList) // sort host by createTime
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
				} else {
					return port
				}
			}
		}
	}
	return 22
}

//AddHost add host
func (c *AnsibleHostConfig) AddHost(h *client.HostNode, installConfPath string) {
	//check role
	//check status
	ansibleHost := &AnsibleHost{
		AnsibleHostIP:   net.ParseIP(h.InternalIP),
		AnsibleHostPort: getSSHPort(installConfPath),
		HostID:          h.ID,
		Role:            h.Role,
		CreateTime:      h.CreateTime,
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
	for i := range h.NodeStatus.Conditions {
		if h.NodeStatus.Conditions[i].Type == "etcd" {
			c.GroupList["etcd"].AddHost(ansibleHost)
			break
		}
	}
}
