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
	"github.com/Sirupsen/logrus"
	"os"
	"fmt"
	"time"
	"io"
	"bytes"
	"github.com/jlaffaye/ftp"
	"github.com/goodrain/rainbond/pkg/event"
)

//FTPBase ftp信息
type FTPBase struct {
	UserName string `json:"username"`
	PassWord string `json:"password"`
	Host string `json:"host"`
	Port int `json:"int"`
}

//NewFTPManager NewFTPManager
func NewFTPManager(username, password, host string, port...int) *FTPBase {
	fb := &FTPBase{
		UserName: username,
		PassWord: password,
		Host: host,
	}
	if len(port) != 0 {
		fb.Port = port[0]
	}else {
		fb.Port = 21
	}
	return fb
}

//UploadFile UploadFile
func (f *FTPBase)UploadFile(path, file string, logger event.Logger) error {
	sc, err:= ftp.DialTimeout(fmt.Sprintf("%s:%d", f.Host, f.Port), 5*time.Second)
	if err != nil {
		logger.Error("ftp服务器连接错误", map[string]string{"step":"slug-share", "status":"failure"})
		return err
	}
	if err := sc.Login(f.UserName, f.PassWord); err != nil {
		logger.Error("ftp服务器登录错误", map[string]string{"step":"slug-share", "status":"failure"})
		return err
	}
	defer sc.Logout()
	if err = sc.ChangeDir(path); err != nil {
		return err
	}
	fi, err := os.Open(file)
	if err != nil {
		return err
	}
	defer fi.Close()
	stat, err := fi.Stat()
	if err != nil {
		return err
	}
	var bufSize int64 = 1024 * 1024 * 24
	if stat.Size() < bufSize {
		bufSize = stat.Size()
		logrus.Debugf("file buf size is %d", bufSize)
	}
	buf := make([]byte, bufSize)
	var i int64
	for i = 0; i < 1024*1024*1024; i += bufSize{
		n, err := fi.Read(buf)
		if err != nil {
			if err.Error() == io.EOF.Error(){
				if n == 0 {
					break
				}else{
					if err := sc.StorFrom(path, bytes.NewReader(buf[:n]), uint64(i)); err != nil {
						return err
					}
				}
			}else {
				return err
			}
		} 
		if err := sc.StorFrom(path, bytes.NewReader(buf), uint64(i)); err != nil {
			return err
		}
	}
	return nil
}

//TransFile TransFile
func (f *FTPBase) TransFile(path, file string) error {
	fi, err := os.Open(file)
	if err != nil {
		return err
	}
	defer fi.Close()
	stat, err := fi.Stat()
	if err != nil {
		return err
	}
	var bufSize int64 = 1024 * 1024 * 5
	if stat.Size() < bufSize {
		bufSize = stat.Size()
		logrus.Debugf("file buf size is %d", bufSize)
	}
	buf := make([]byte, bufSize)
	var i int64
	for i = 0; i < 1024*1024*1024; i += bufSize{
		n, err := fi.Read(buf)
		if err != nil {
			if err.Error() == io.EOF.Error(){
				if n == 0 {
					break
				}else {
					f, err := os.OpenFile(path+"/mm.tar.gz", os.O_WRONLY, 0644)
					if err != nil {
						return err
					}
					defer f.Close()
					_, err = f.WriteAt(buf[:n], i)
					if err != nil {
						return err
					}
				}
			}else {
				return err
			}
		} 
		f, err := os.OpenFile(path+"/mm.tar.gz", os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteAt(buf, i)
		if err != nil {
			return err
		}
	}
	return nil	
}

