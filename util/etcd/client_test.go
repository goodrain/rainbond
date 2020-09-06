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

package etcd

import (
	"fmt"
	"path"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"golang.org/x/net/context"
)

func TestNewETCDClient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	basePath := "/Users/fanyangyang/Downloads/etcdcas"
	caPath := path.Join(basePath, "etcdca.crt")
	certPath := path.Join(basePath, "apiserver-etcd-client.crt")
	keyPath := path.Join(basePath, "apiserver-etcd-client.key")
	// client, err := NewETCDClient(ctx, 10*time.Second, []string{"192.168.2.203:2379"}, "", "", "") // connection no tls success
	/**
	curl --cacert etcdca.crt  --cert apiserver-etcd-client.crt --key apiserver-etcd-client.key -L https://192.168.2.63:2379/v2/keys/foo -XGET
	cacert 指定服务器所使用的签发机构根证书，所以需要使用etcd签发机构的根证书，而非kubernetes的签发机构根证书。该文件路径为/etc/kubernetes/pki/etcd/ca.crt
	cert 指定客户端证书，这里使用的是kube-apiserver的证书， 该文件路径为：/etc/kubernetes/pki/apiserver-etcd-client.crt， 也可以使用etcd的节点证书/etc/kubernetes/pki/etcd/peer.crt
	cert 指定客户端证书秘钥，这里使用的是kube-apiserver的证书的秘钥， 该文件路径为：/etc/kubernetes/pki/apiserver-etcd-client.key
	*/
	clientArgs := ClientArgs{
		Endpoints: []string{"192.168.2.63:2379"},
		CaFile:    caPath,
		CertFile:  certPath,
		KeyFile:   keyPath,
	}
	client, err := NewClient(ctx, &clientArgs)
	if err != nil {
		t.Fatal("create client error: ", err)
	}
	resp, err := client.Get(ctx, "/foo")
	if err != nil {
		t.Fatal("get key error", err)
	}
	t.Logf("resp is : %+v", resp)
	time.Sleep(30)
}

func TestEtcd(t *testing.T) {
	// test etcd retry connection
	fmt.Println("yes")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	etcdClientArgs := &ClientArgs{
		Endpoints: []string{"http://127.0.0.1:2359"},
	}
	etcdcli, err := NewClient(ctx, etcdClientArgs)
	if err != nil {
		logrus.Errorf("create etcd client v3 error, %v", err)
		t.Fatal(err)
	}
	memberList, err := etcdcli.MemberList(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(memberList.Members) == 0 {
		fmt.Println("no members")
		return
	}
	t.Logf("members is: %s", memberList.Members[0].Name)
}
