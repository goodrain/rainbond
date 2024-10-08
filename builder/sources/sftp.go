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

package sources

import (
	"fmt"
	"github.com/goodrain/rainbond/pkg/component/storage"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/pkg/sftp"
)

// SFTPClient sFTP客户端
type SFTPClient struct {
	UserName   string `json:"username"`
	PassWord   string `json:"password"`
	Host       string `json:"host"`
	Port       int    `json:"int"`
	conn       *ssh.Client
	sftpClient *sftp.Client
}

// NewSFTPClient NewSFTPClient
func NewSFTPClient(username, password, host, port string) (*SFTPClient, error) {
	fb := &SFTPClient{
		UserName: username,
		PassWord: password,
		Host:     host,
	}
	if len(port) != 0 {
		var err error
		fb.Port, err = strconv.Atoi(port)
		if err != nil {
			fb.Port = 21
		}
	} else {
		fb.Port = 21
	}
	var auths []ssh.AuthMethod
	if aconn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		auths = append(auths, ssh.PublicKeysCallback(agent.NewClient(aconn).Signers))
	}
	if fb.PassWord != "" {
		auths = append(auths, ssh.Password(fb.PassWord))
	}
	config := ssh.ClientConfig{
		User:            username,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	addr := fmt.Sprintf("%s:%d", host, fb.Port)
	conn, err := ssh.Dial("tcp", addr, &config)
	if err != nil {
		logrus.Errorf("unable to connect to [%s]: %v", addr, err)
		return nil, err
	}
	c, err := sftp.NewClient(conn, sftp.MaxPacket(1<<15))
	if err != nil {
		logrus.Errorf("unable to start sftp subsytem: %v", err)
		return nil, err
	}
	fb.conn = conn
	fb.sftpClient = c
	return fb, nil
}

// Close 关闭啊
func (s *SFTPClient) Close() {
	if s.sftpClient != nil {
		s.sftpClient.Close()
	}
	if s.conn != nil {
		s.conn.Close()
	}
}
func (s *SFTPClient) checkMd5(src, dst string, logger event.Logger) (bool, error) {
	if err := util.CreateFileHash(src, src+".md5"); err != nil {
		return false, err
	}
	existmd5, err := s.FileExist(dst + ".md5")
	if err != nil && err.Error() != "file does not exist" {
		return false, err
	}
	exist, err := s.FileExist(dst)
	if err != nil && err.Error() != "file does not exist" {
		return false, err
	}
	if exist && existmd5 {
		if err := s.DownloadFile(dst+".md5", src+".md5.old", logger); err != nil {
			return false, err
		}
		old, err := ioutil.ReadFile(src + ".md5.old")
		if err != nil {
			return false, err
		}
		os.Remove(src + ".md5.old")
		new, err := ioutil.ReadFile(src + ".md5")
		if err != nil {
			return false, err
		}
		if string(old) == string(new) {
			return true, nil
		}
	}
	return false, nil
}

// PushFile PushFile
func (s *SFTPClient) PushFile(src, dst string, logger event.Logger) error {
	logger.Info(fmt.Sprintf("开始上传代码包到FTP服务器"), map[string]string{"step": "slug-share"})
	ok, err := s.checkMd5(src, dst, logger)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	srcFile, err := os.OpenFile(src, os.O_RDONLY, 0644)
	if err != nil {
		if logger != nil {
			logger.Error("打开源文件失败", map[string]string{"step": "share"})
		}
		return err
	}
	defer srcFile.Close()
	srcStat, err := srcFile.Stat()
	if err != nil {
		if logger != nil {
			logger.Error("打开源文件失败", map[string]string{"step": "share"})
		}
		return err
	}
	// check or create dir
	dir := filepath.Dir(dst)
	_, err = s.sftpClient.Stat(dir)
	if err != nil {
		if err.Error() == "file does not exist" {
			err := s.MkdirAll(dir)
			if err != nil {
				if logger != nil {
					logger.Error("创建目标文件目录失败", map[string]string{"step": "share"})
				}
				return err
			}
		} else {
			if logger != nil {
				logger.Error("检测目标文件目录失败", map[string]string{"step": "share"})
			}
			return err
		}
	}
	// remove all file if exist
	s.sftpClient.Remove(dst)
	dstFile, err := s.sftpClient.Create(dst)
	if err != nil {
		if logger != nil {
			logger.Error("打开目标文件失败", map[string]string{"step": "share"})
		}
		return err
	}
	defer dstFile.Close()
	allSize := srcStat.Size()
	if err := storage.CopyWithProgress(srcFile, dstFile, allSize, logger); err != nil {
		return err
	}
	// write remote md5 file
	md5, _ := ioutil.ReadFile(src + ".md5")
	dstMd5File, err := s.sftpClient.Create(dst + ".md5")
	if err != nil {
		logrus.Errorf("create md5 file in sftp server error.%s", err.Error())
		return nil
	}
	defer dstMd5File.Close()
	if _, err := dstMd5File.Write(md5); err != nil {
		logrus.Errorf("write md5 file in sftp server error.%s", err.Error())
	}
	return nil
}

// DownloadFile DownloadFile
func (s *SFTPClient) DownloadFile(src, dst string, logger event.Logger) error {
	logger.Info(fmt.Sprintf("开始从FTP服务器下载代码包"), map[string]string{"step": "slug-share"})

	srcFile, err := s.sftpClient.OpenFile(src, 0644)
	if err != nil {
		if logger != nil {
			logger.Error("打开源文件失败", map[string]string{"step": "share"})
		}
		return err
	}
	defer srcFile.Close()
	srcStat, err := srcFile.Stat()
	if err != nil {
		if logger != nil {
			logger.Error("打开源文件失败", map[string]string{"step": "share"})
		}
		return err
	}
	// 验证并创建目标目录
	dir := filepath.Dir(dst)
	if err := util.CheckAndCreateDir(dir); err != nil {
		if logger != nil {
			logger.Error("检测并创建目标文件目录失败", map[string]string{"step": "share"})
		}
		return err
	}
	// 先删除文件如果存在
	os.Remove(dst)
	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if logger != nil {
			logger.Error("打开目标文件失败", map[string]string{"step": "share"})
		}
		return err
	}
	defer dstFile.Close()
	allSize := srcStat.Size()
	return storage.CopyWithProgress(srcFile, dstFile, allSize, logger)
}

// FileExist 文件是否存在
func (s *SFTPClient) FileExist(filepath string) (bool, error) {
	if _, err := s.sftpClient.Stat(filepath); err != nil {
		return false, err
	}
	return true, nil
}

// MkdirAll 创建目录，递归
func (s *SFTPClient) MkdirAll(dirpath string) error {
	parentDir := filepath.Dir(dirpath)
	_, err := s.sftpClient.Stat(parentDir)
	if err != nil {
		if err.Error() == "file does not exist" {
			err := s.MkdirAll(parentDir)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	err = s.sftpClient.Mkdir(dirpath)
	if err != nil {
		return err
	}
	return nil
}
