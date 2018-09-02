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

package util

import (
	"archive/zip"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
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

//DirIsEmpty 验证目录是否为空
func DirIsEmpty(dir string) bool {
	infos, err := ioutil.ReadDir(dir)
	if len(infos) == 0 || err != nil {
		return true
	}
	return false
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
		filePath = "/opt/rainbond/etc/node/node_host_uuid.conf"
	}
	_, err := os.Stat(filePath)
	if err != nil {
		if strings.HasSuffix(err.Error(), "no such file or directory") {
			uid, err := CreateHostID()
			if err != nil {
				return "", err
			}
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

//CreateHostID create host id by mac addr
func CreateHostID() (string, error) {
	macAddrs := getMacAddrs()
	if macAddrs == nil || len(macAddrs) == 0 {
		return "", fmt.Errorf("read macaddr error when create node id")
	}
	ip, _ := LocalIP()
	hash := md5.New()
	hash.Write([]byte(macAddrs[0] + ip.String()))
	uid := fmt.Sprintf("%x", hash.Sum(nil))
	if len(uid) >= 32 {
		return uid[:32], nil
	}
	for i := len(uid); i < 32; i++ {
		uid = uid + "0"
	}
	return uid, nil
}

func getMacAddrs() (macAddrs []string) {
	netInterfaces, err := net.Interfaces()
	if err != nil {
		fmt.Printf("fail to get net interfaces: %v", err)
		return macAddrs
	}

	for _, netInterface := range netInterfaces {
		macAddr := netInterface.HardwareAddr.String()
		if len(macAddr) == 0 {
			continue
		}
		macAddrs = append(macAddrs, macAddr)
	}
	return macAddrs
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

//GetDirSizeByCmd get dir sizes by du command
//return kb
func GetDirSizeByCmd(path string) float64 {
	out, err := CmdExec(fmt.Sprintf("du -sk %s", path))
	if err != nil {
		fmt.Println(err)
		return 0
	}
	info := strings.Split(out, "	")
	fmt.Println(info)
	if len(info) < 2 {
		return 0
	}
	i, _ := strconv.Atoi(info[0])
	return float64(i)
}

//GetFileSize get file size
func GetFileSize(path string) int64 {
	if fileInfo, err := os.Stat(path); err == nil {
		return fileInfo.Size()
	}
	return 0
}

//GetDirSize kb为单位
func GetDirSize(path string) float64 {
	if ok, err := FileExists(path); err != nil || !ok {
		return 0
	}

	fileSizes := make(chan int64)
	concurrent := make(chan int, 10)
	var wg sync.WaitGroup

	wg.Add(1)
	go walkDir(path, &wg, fileSizes, concurrent)

	go func() {
		wg.Wait() //等待goroutine结束
		close(fileSizes)
	}()
	var nfiles, nbytes int64
loop:
	for {
		select {
		case size, ok := <-fileSizes:
			if !ok {
				break loop
			}
			nfiles++
			nbytes += size
		}
	}
	return float64(nbytes / 1024)
}

//获取目录dir下的文件大小
func walkDir(dir string, wg *sync.WaitGroup, fileSizes chan<- int64, concurrent chan int) {
	defer wg.Done()
	concurrent <- 1
	defer func() {
		<-concurrent
	}()
	for _, entry := range listDirNonSymlink(dir) {
		if entry.IsDir() { //目录
			wg.Add(1)
			subDir := filepath.Join(dir, entry.Name())
			go walkDir(subDir, wg, fileSizes, concurrent)
		} else {
			fileSizes <- entry.Size()
		}
	}
}

//sema is a counting semaphore for limiting concurrency in listDir
var sema = make(chan struct{}, 20)

//读取目录dir下的文件信息
func listDir(dir string) []os.FileInfo {
	sema <- struct{}{}
	defer func() { <-sema }()
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		logrus.Errorf("get file sizt: %v\n", err)
		return nil
	}
	return entries
}

// 列出指定目录下的非软链类型的所有条目
func listDirNonSymlink(dir string) []os.FileInfo {
	sema <- struct{}{}
	defer func() { <-sema }()
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		logrus.Errorf("get file sizt: %v\n", err)
		return nil
	}

	var result []os.FileInfo
	for i := range entries {
		if entries[i].Mode()&os.ModeSymlink == 0 {
			result = append(result, entries[i])
		}
	}
	return result
}

//RemoveSpaces 去除空格项
func RemoveSpaces(sources []string) (re []string) {
	for _, s := range sources {
		if s != " " && s != "" {
			re = append(re, s)
		}
	}
	return
}

//CmdExec CmdExec
func CmdExec(args string) (string, error) {
	out, err := exec.Command("bash", "-c", args).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

//Zip zip compressing source dir to target file
func Zip(source, target string) error {
	if err := CheckAndCreateDir(filepath.Dir(target)); err != nil {
		return err
	}
	zipfile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer zipfile.Close()

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	info, err := os.Stat(source)
	if err != nil {
		return nil
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(source)
	}

	filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		if baseDir != "" {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
		}
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}
		//set file uid and
		elem := reflect.ValueOf(info.Sys()).Elem()
		uid := elem.FieldByName("Uid").Uint()
		gid := elem.FieldByName("Gid").Uint()
		header.Comment = fmt.Sprintf("%d/%d", uid, gid)
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	})

	return err
}

//Unzip archive file to target dir
func Unzip(archive, target string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}

	for _, file := range reader.File {
		run := func() error {
			path := filepath.Join(target, file.Name)
			if file.FileInfo().IsDir() {
				os.MkdirAll(path, file.Mode())
				if file.Comment != "" && strings.Contains(file.Comment, "/") {
					guid := strings.Split(file.Comment, "/")
					if len(guid) == 2 {
						uid, _ := strconv.Atoi(guid[0])
						gid, _ := strconv.Atoi(guid[1])
						if err := os.Chown(path, uid, gid); err != nil {
							return err
						}
					}
				}
				return nil
			}

			fileReader, err := file.Open()
			if err != nil {
				return err
			}
			defer fileReader.Close()

			targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
			if err != nil {
				return err
			}
			defer targetFile.Close()

			if _, err := io.Copy(targetFile, fileReader); err != nil {
				return err
			}
			if file.Comment != "" && strings.Contains(file.Comment, "/") {
				guid := strings.Split(file.Comment, "/")
				if len(guid) == 2 {
					uid, _ := strconv.Atoi(guid[0])
					gid, _ := strconv.Atoi(guid[1])
					if err := os.Chown(path, uid, gid); err != nil {
						return err
					}
				}
			}
			return nil
		}
		if err := run(); err != nil {
			return err
		}
	}

	return nil
}

//GetParentDirectory GetParentDirectory
func GetParentDirectory(dirctory string) string {
	return substr(dirctory, 0, strings.LastIndex(dirctory, "/"))
}

func substr(s string, pos, length int) string {
	runes := []rune(s)
	l := pos + length
	if l > len(runes) {
		l = len(runes)
	}
	return string(runes[pos:l])
}

//Rename move file
func Rename(old, new string) error {
	_, err := os.Stat(GetParentDirectory(new))
	if err != nil {
		if err == os.ErrNotExist || strings.Contains(err.Error(), "no such file or directory") {
			if err := os.MkdirAll(GetParentDirectory(new), 0755); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return os.Rename(old, new)
}

//MergeDir MergeDir
//if Subdirectories already exist, Don't replace
func MergeDir(fromdir, todir string) error {
	files, err := ioutil.ReadDir(fromdir)
	if err != nil {
		return err
	}
	for _, f := range files {
		if err := os.Rename(path.Join(fromdir, f.Name()), path.Join(todir, f.Name())); err != nil {
			if !strings.Contains(err.Error(), "file exists") {
				return err
			}
		}
	}
	return nil
}

//CreateVersionByTime create version number
func CreateVersionByTime() string {
	now := time.Now()
	re := fmt.Sprintf("%d%d%d%d%d%d%d", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second(), now.Nanosecond())
	return re
}

// GetDirList get all lower level dir
func GetDirList(dirpath string, level int) ([]string, error) {
	var dirlist []string
	list, err := ioutil.ReadDir(dirpath)
	if err != nil {
		return nil, err
	}
	for _, f := range list {
		if f.IsDir() {
			if level <= 1 {
				dirlist = append(dirlist, filepath.Join(dirpath, f.Name()))
			} else {
				list, err := GetDirList(filepath.Join(dirpath, f.Name()), level-1)
				if err != nil {
					return nil, err
				}
				dirlist = append(dirlist, list...)
			}
		}
	}
	return dirlist, nil
}

func GetFileList(dirpath string, level int) ([]string, error) {
	var dirlist []string
	list, err := ioutil.ReadDir(dirpath)
	if err != nil {
		return nil, err
	}
	for _, f := range list {
		if !f.IsDir() && level <= 1 {
			dirlist = append(dirlist, filepath.Join(dirpath, f.Name()))
		} else if level > 1 && f.IsDir() {
			list, err := GetFileList(filepath.Join(dirpath, f.Name()), level-1)
			if err != nil {
				return nil, err
			}
			dirlist = append(dirlist, list...)
		}
	}
	return dirlist, nil
}

// GetDirNameList get all lower level dir
func GetDirNameList(dirpath string, level int) ([]string, error) {
	var dirlist []string
	list, err := ioutil.ReadDir(dirpath)
	if err != nil {
		return nil, err
	}
	for _, f := range list {
		if f.IsDir() {
			if level <= 1 {
				dirlist = append(dirlist, f.Name())
			} else {
				list, err := GetDirList(filepath.Join(dirpath, f.Name()), level-1)
				if err != nil {
					return nil, err
				}
				dirlist = append(dirlist, list...)
			}
		}
	}
	return dirlist, nil
}

//DiskUsage  disk usage of path/disk
func DiskUsage(path string) (tatol, free uint64) {
	fs := syscall.Statfs_t{}
	err := syscall.Statfs(path, &fs)
	if err != nil {
		return
	}
	return fs.Blocks * uint64(fs.Bsize), fs.Bfree * uint64(fs.Bsize)
}

//GetCurrentDir get current dir
func GetCurrentDir() string {
	dir, err := filepath.Abs("./")
	if err != nil {
		log.Fatal(err)
	}
	return strings.Replace(dir, "\\", "/", -1)
}
