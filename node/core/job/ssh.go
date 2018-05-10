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

package job

import (
	"net"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/node/api/model"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func UnifiedLogin(login *model.Login) (*ssh.Client, error) {
	logrus.Infof("login target host by info %v", login)
	if login.LoginType {
		return SSHClientTo(login.HostPort, "root", login.RootPwd)
	} else {
		return SSHClient(login.HostPort, "root")
	}
}
func SSHClientTo(hostport string, username, password string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}
	client, err := ssh.Dial("tcp", hostport, config)
	if err != nil {
		logrus.Warnf("failed to connect %s use username %s ,error: %s", hostport, username, err.Error())
		return new(ssh.Client), err
	}
	return client, nil
}
func SSHClient(hostport string, username string) (*ssh.Client, error) {
	sock, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		logrus.Infof("error login,details: %s", err.Error())
		return nil, err
	}

	agent := agent.NewClient(sock)

	signers, err := agent.Signers()
	if err != nil {
		logrus.Infof("error login,details: %s", err.Error())
		return nil, err
	}

	auths := []ssh.AuthMethod{ssh.PublicKeys(signers...)}

	cfg := &ssh.ClientConfig{
		User: username,
		Auth: auths,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
	}
	cfg.SetDefaults()
	logrus.Infof("tcp dial to %s", hostport)
	client, err := ssh.Dial("tcp", hostport, cfg)
	if err != nil {
		logrus.Infof("error login,details: %s", err.Error())
		return nil, err
	}
	return client, nil
}
