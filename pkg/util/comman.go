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

package util

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/twinj/uuid"
)

//CheckAndCreateDir check and create dir
func CheckAndCreateDir(path string) error {
	if subPathExists, err := FileExists(path); err != nil {
		return fmt.Errorf("Could not determine if subPath %s exists; will not attempt to change its permissions", path)
	} else if !subPathExists {
		// Create the sub path now because if it's auto-created later when referenced, it may have an
		// incorrect ownership and mode. For example, the sub path directory must have at least g+rwx
		// when the pod specifies an fsGroup, and if the directory is not created here, Docker will
		// later auto-create it with the incorrect mode 0750
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to mkdir:%s", path)
		}

		if err := os.Chmod(path, 0755); err != nil {
			return err
		}
	}
	return nil
}

//OpenOrCreateFile open or create file
func OpenOrCreateFile(filename string) (*os.File, error) {
	return os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0777)
}

//FileExists check file exist
func FileExists(filename string) (bool, error) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

//SearchFileBody 搜索文件中是否含有指定字符串
func SearchFileBody(filename, searchStr string) bool {
	body, _ := ioutil.ReadFile(filename)
	return strings.Contains(string(body), searchStr)
}

//IsHaveFile 指定目录是否含有文件
//.开头文件除外
func IsHaveFile(path string) bool {
	files, _ := ioutil.ReadDir(path)
	for _, file := range files {
		if !strings.HasPrefix(file.Name(), ".") {
			return true
		}
	}
	return false
}

//SearchFile 搜索指定目录是否有指定文件，指定搜索目录层数，-1为全目录搜索
func SearchFile(pathDir, name string, level int) bool {
	if level == 0 {
		return false
	}
	files, _ := ioutil.ReadDir(pathDir)
	var dirs []os.FileInfo
	for _, file := range files {
		if file.IsDir() {
			dirs = append(dirs, file)
			continue
		}
		if file.Name() == name {
			return true
		}
	}
	if level == 1 {
		return false
	}
	for _, dir := range dirs {
		ok := SearchFile(path.Join(pathDir, dir.Name()), name, level-1)
		if ok {
			return ok
		}
	}
	return false
}

//FileExistsWithSuffix 指定目录是否含有指定后缀的文件
func FileExistsWithSuffix(pathDir, suffix string) bool {
	files, _ := ioutil.ReadDir(pathDir)
	for _, file := range files {
		if strings.HasSuffix(file.Name(), suffix) {
			return true
		}
	}
	return false
}

//CmdRunWithTimeout exec cmd with timeout
func CmdRunWithTimeout(cmd *exec.Cmd, timeout time.Duration) (bool, error) {
	done := make(chan error)
	if cmd.Process != nil { //还原执行状态
		cmd.Process = nil
		cmd.ProcessState = nil
	}
	if err := cmd.Start(); err != nil {
		return false, err
	}
	go func() {
		done <- cmd.Wait()
	}()
	var err error
	select {
	case <-time.After(timeout):
		// timeout
		if err = cmd.Process.Kill(); err != nil {
			logrus.Errorf("failed to kill: %s, error: %s", cmd.Path, err.Error())
		}
		go func() {
			<-done // allow goroutine to exit
		}()
		logrus.Info("process:%s killed", cmd.Path)
		return true, err
	case err = <-done:
		return false, err
	}
}

//ReadHostID 读取当前机器ID
//ID是节点的唯一标识，acp_node将把ID与机器信息的绑定关系维护于etcd中
func ReadHostID(filePath string) (string, error) {
	if filePath == "" {
		filePath = "/etc/goodrain/host_uuid.conf"
	}
	_, err := os.Stat(filePath)
	if err != nil {
		if strings.HasSuffix(err.Error(), "no such file or directory") {
			uid := uuid.NewV4().String()
			err = ioutil.WriteFile(filePath, []byte("host_uuid="+uid), 0777)
			if err != nil {
				logrus.Error("Write host_uuid file error.", err.Error())
			}
			return uid, nil
		}
		return "", err
	}
	body, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	info := strings.Split(strings.TrimSpace(string(body)), "=")
	if len(info) == 2 {
		return info[1], nil
	}
	return "", fmt.Errorf("Invalid host uuid from file")
}

//LocalIP 获取本机 ip
// 获取第一个非 loopback ip
func LocalIP() (net.IP, error) {
	tables, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, t := range tables {
		addrs, err := t.Addrs()
		if err != nil {
			return nil, err
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.IsLoopback() {
				continue
			}
			if v4 := ipnet.IP.To4(); v4 != nil {
				return v4, nil
			}
		}
	}
	return nil, fmt.Errorf("cannot find local IP address")
}

//GetIDFromKey 从 etcd 的 key 中取 id
func GetIDFromKey(key string) string {
	index := strings.LastIndex(key, "/")
	if index < 0 {
		return ""
	}
	if strings.Contains(key, "-") { //build in任务，为了给不同node做一个区分
		return strings.Split(key[index+1:], "-")[0]
	}

	return key[index+1:]
}

//Deweight 去除数组重复
func Deweight(data *[]string) {
	var result []string
	if len(*data) < 1024 {
		// 切片长度小于1024的时候，循环来过滤
		for i := range *data {
			flag := true
			for j := range result {
				if result[j] == (*data)[i] {
					flag = false // 存在重复元素，标识为false
					break
				}
			}
			if flag && (*data)[i] != "" { // 标识为false，不添加进结果
				result = append(result, (*data)[i])
			}
		}
	} else {
		// 大于的时候，通过map来过滤
		var tmp = make(map[string]byte)
		for _, d := range *data {
			l := len(tmp)
			tmp[d] = 0
			if len(tmp) != l && d != "" { // 加入map后，map长度变化，则元素不重复
				result = append(result, d)
			}
		}
	}
	*data = result
}
