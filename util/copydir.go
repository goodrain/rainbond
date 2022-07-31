package util

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// FormatPath format path
func FormatPath(s string) string {
	log.Println("runtime.GOOS:", runtime.GOOS)
	switch runtime.GOOS {
	case "windows":
		return strings.Replace(s, "/", "\\", -1)
	case "darwin", "linux":
		return strings.Replace(s, "\\", "/", -1)
	default:
		logrus.Info("only support linux,windows,darwin, but os is " + runtime.GOOS)
		return s
	}
}

// MoveDir move dir
func MoveDir(src string, dest string) error {
	src = FormatPath(src)
	dest = FormatPath(dest)
	logrus.Info("src", src)
	logrus.Info("dest", dest)

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("xcopy", src, dest, "/I", "/E")
	case "darwin", "linux":
		cmd = exec.Command("cp", "-R", src, dest)
	}
	outPut, err := cmd.Output()
	if err != nil {
		logrus.Errorf("Output error: %s", err.Error())
		return err
	}
	fmt.Println(outPut)
	if err := os.RemoveAll(src); err != nil {
		logrus.Errorf("remove oldpath error: %s", err.Error())
		return err
	}
	return nil
}

// MD5 md5
func MD5(file string) string {
	f, err := os.Open(file)
	if err != nil {
		logrus.Error(err)
	}
	defer f.Close()

	h := md5.New()
	_, err = io.Copy(h, f)
	if err != nil {
		logrus.Error(err)
	}
	res := hex.EncodeToString(h.Sum(nil))
	logrus.Info("md5:", res)
	return res
}

// CopyDir move dir
func CopyDir(src string, dest string) error {
	_, err := os.Stat(dest)
	if err != nil {
		if !os.IsExist(err) {
			err := os.MkdirAll(dest, 0755)
			if err != nil {
				logrus.Error("make and copy dir error", err)
			}
		}
	}
	src = FormatPath(src)
	dest = FormatPath(dest)
	logrus.Info("src", src)
	logrus.Info("dest", dest)

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("xcopy", src, dest, "/I", "/E")
	case "darwin", "linux":
		cmd = exec.Command("cp", "-R", src, dest)
	}
	outPut, err := cmd.Output()
	if err != nil {
		logrus.Errorf("Output error: %s", err.Error())
		return err
	}
	fmt.Println(outPut)
	return nil
}

// 判断所给路径文件/文件夹是否存在
func PathExists(path string)(bool,error){
	_,err := os.Stat(path)
	if err == nil{
		return true,nil
	}
	//isnotexist来判断，是不是不存在的错误
	if os.IsNotExist(err){	//如果返回的错误类型使用os.isNotExist()判断为true，说明文件或者文件夹不存在
		return false,nil
	}
	return false,err//如果有错误了，但是不是不存在的错误，所以把这个错误原封不动的返回
}