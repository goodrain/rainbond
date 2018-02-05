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
	"fmt"
	"time"
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
	//data, err := ioutil.ReadFile(file)
	//if err :=  sc.Stor(path, data); err != nil {
	//	return err
	//}
	return nil
}



