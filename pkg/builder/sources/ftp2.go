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

package sources

import (
    "github.com/dutchcoders/goftp"
    "fmt"
	"github.com/goodrain/rainbond/pkg/event"
	"github.com/Sirupsen/logrus"
	"os"
	"strings"
)

//FTPConn ftp信息
type FTPConn struct {
	UserName string `json:"username"`
	PassWord string `json:"password"`
	Host string `json:"host"`
	Port int `json:"int"`
	FTP *goftp.FTP 
}

//NewFTPConnManager NewFTPManager
func NewFTPConnManager(logger event.Logger, username, password, host string, port...int) *FTPConn {
	fb := &FTPConn{
		UserName: username,
		PassWord: password,
		Host: host,
	}
	if len(port) != 0 {
		fb.Port = port[0]
	}else {
		fb.Port = 21
	}
	ftp, err := fb.FTPConnect()
	if err != nil {
		logger.Error("ftp服务器连接错误", map[string]string{"step":"slug-share", "status":"failure"})
	}
	logger.Debug("ftp服务器连接成功", map[string]string{"step":"slug-share", "status":"success"})
	fb.FTP = ftp
	return fb
}

//FTPConnect 连接FTP
func (f *FTPConn) FTPConnect() (*goftp.FTP, error) {
	connInfo := fmt.Sprintf("%s:%d", f.Host, f.Port)
	ftp, err := goftp.Connect(connInfo)
	if err != nil {
		logrus.Errorf("ftp connect error: %s", err.Error())
		return nil, err
	}
	return ftp, nil
}

//FTPLogin 登录FTP
func (f *FTPConn) FTPLogin(logger event.Logger) error {
	if err := f.FTP.Login(f.UserName, f.PassWord); err != nil {
		logger.Error("ftp服务器登录错误", map[string]string{"step":"slug-share", "status":"failure"})
		return err
	}
	logger.Debug("ftp服务器登录成功", map[string]string{"step":"slug-share", "status":"success"})
	return nil
}

//FTPCWD 修改目录
func (f *FTPConn) FTPCWD(logger event.Logger, path string) (string, error) {
	if err := f.FTP.Cwd(path); err != nil {
		logger.Error(fmt.Sprintf("登入目录 %s 失败", path), map[string]string{"step":"slug-share", "status":"failure"})
		return "", err
	}
	//TODO: 目录不存在的处理
	curpath, _ := f.FTP.Pwd()
	logger.Debug(fmt.Sprintf("登入的当前目录为：%s", curpath), map[string]string{"step":"slug-share"})
	return curpath, nil
}

//FTPUpload 文件上传
func (f *FTPConn) FTPUpload(logger event.Logger, path string, files ...string) error {
	for _, filepath := range files {
		//TODO: 检查文件在本地是否存在
		if !f.CheckFileExist(filepath) {
			logger.Error(fmt.Sprintf("文件 %s 不存在", filepath), map[string]string{"step":"slug-share", "status":"failure"})
			return fmt.Errorf("file %s not exist", filepath)
		}
		file, err := os.Open(filepath)
		if err != nil {
			logger.Error(fmt.Sprintf("读取文件%s错误", filepath), map[string]string{"step":"slug-share", "status":"failure"})
			return err
		}
		upPath := path + "/" + filepath
		filename := getFileName(filepath)
		if err := f.FTP.Cwd(upPath); err != nil {
			if strings.Contains(err.Error(), "550"){
				if err := f.FTP.Stor(upPath, file); err != nil {
					logrus.Debugf("ftp filepath is %s", upPath)
					logger.Error(fmt.Sprintf("上传文件%s到ftp服务器失败", filename), map[string]string{"step":"slug-share", "status":"failure"})
					return fmt.Errorf("ftp up error is %s", err.Error())
				}	
				logger.Debug(fmt.Sprintf("上传文件%s至ftp服务器成功", filename), map[string]string{"step":"slug-share", "status":"success"})
			}else {
				logger.Info(fmt.Sprintf("需要上传的文件 %s 已经在ftp服务器中存在", filename), map[string]string{"step":"slug-share", "status":"success"})
			}
		}
	}
	logger.Info("文件上传ftp成功", map[string]string{"step":"slug-share", "status":"success"})
	return nil
}

//CheckFileExist 检查文件在本地是否存在
func (f *FTPConn) CheckFileExist(filepath string) bool {
	return true
}

func getFileName(filepath string) string {
	if strings.Contains(filepath, "/") {
		mm := strings.Split(filepath, "/")
		return mm[len(mm)-1]
	}
	return filepath
} 