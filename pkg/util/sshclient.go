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
	"errors"
	"io"
	"net"
	"os"
	"strconv"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

//SSHClient ssh client
type SSHClient struct {
	IP             string
	Port           int
	User           string
	Password       string
	Method         string
	Key            string
	Stdout, Stderr io.Writer
	Cmd            string
}

//NewSSHClient new ssh client
func NewSSHClient(ip, user, password, cmd string, port int, stdout, stderr io.Writer) *SSHClient {
	var method = "password"
	if password == "" {
		method = "publickey"
	}
	return &SSHClient{
		IP:       ip,
		User:     user,
		Password: password,
		Method:   method,
		Cmd:      cmd,
		Port:     port,
		Stderr:   stderr,
		Stdout:   stdout,
	}
}

//Connection 执行远程连接
func (server *SSHClient) Connection() error {
	auths, err := parseAuthMethods(server)
	if err != nil {
		return err
	}
	config := &ssh.ClientConfig{
		User:            server.User,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	addr := server.IP + ":" + strconv.Itoa(server.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	session.Stderr = server.Stderr
	session.Stdout = server.Stdout
	if err := session.Run(server.Cmd); err != nil {
		return err
	}
	return nil
}

// 解析鉴权方式
func parseAuthMethods(server *SSHClient) ([]ssh.AuthMethod, error) {
	sshs := []ssh.AuthMethod{}
	switch server.Method {
	case "password":
		sshs = append(sshs, ssh.Password(server.Password))
		break
	case "publickey":
		socket := os.Getenv("SSH_AUTH_SOCK")
		conn, err := net.Dial("unix", socket)
		if err != nil {
			return nil, err
		}
		agentClient := agent.NewClient(conn)
		sshs = append(sshs, ssh.PublicKeysCallback(agentClient.Signers))
		break
	default:
		return nil, errors.New("无效的密码方式: " + server.Method)
	}

	return sshs, nil
}
