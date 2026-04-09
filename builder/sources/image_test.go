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
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/docker/docker/api/types/registry"
)

// capability_id: rainbond.source-image.parse-name
func TestImageName(t *testing.T) {
	tests := []struct {
		input string
		host  string
		name  string
		tag   string
	}{
		{input: "hub.goodrain.com/nginx:v1", host: "hub.goodrain.com", name: "nginx", tag: "v1"},
		{input: "hub.goodrain.cn/nginx", host: "hub.goodrain.cn", name: "nginx", tag: "latest"},
		{input: "nginx:v2", host: "", name: "nginx", tag: "v2"},
		{input: "tomcat", host: "", name: "tomcat", tag: "latest"},
	}
	for _, tt := range tests {
		got := ImageNameHandle(tt.input)
		if got.Host != tt.host || got.Name != tt.name || got.Tag != tt.tag {
			t.Fatalf("ImageNameHandle(%q)=%+v, want host=%q name=%q tag=%q", tt.input, got, tt.host, tt.name, tt.tag)
		}
	}
}

// capability_id: rainbond.source-image.parse-name-with-namespace
func TestImageNameWithNamespace(t *testing.T) {
	got := ImageNameWithNamespaceHandle("registry.example.com/team/demo:v1")
	if got.Host != "registry.example.com" || got.Namespace != "team" || got.Name != "demo" || got.Tag != "v1" {
		t.Fatalf("unexpected parsed image: %+v", got)
	}

	got = ImageNameWithNamespaceHandle("demo")
	if got.Host != "" || got.Namespace != "" || got.Name != "demo" || got.Tag != "latest" {
		t.Fatalf("unexpected parsed image without namespace: %+v", got)
	}
}

// capability_id: rainbond.source-image.auth-base64-encode
func TestEncodeAuthToBase64(t *testing.T) {
	encoded, err := EncodeAuthToBase64(registry.AuthConfig{
		Username: "demo",
		Password: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var auth registry.AuthConfig
	if err := json.Unmarshal(raw, &auth); err != nil {
		t.Fatal(err)
	}
	if auth.Username != "demo" || auth.Password != "secret" {
		t.Fatalf("unexpected decoded auth config: %+v", auth)
	}
}

// capability_id: rainbond.source-image.trusted-registry-check
func TestCheckTrustedRepositories(t *testing.T) {
	t.Skip("requires remote registry access")
	err := CheckTrustedRepositories("hub.goodrain.com/zengqg-test/etcd2:v2.2.0", "zengqg-test", "zengqg-test")
	if err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.source-image.save
func TestImageSave(t *testing.T) {
	t.Skip("requires local docker daemon")
	/*
		dc, _ := client.NewEnvClient()
		if err := ImageSave(dc, "hub.goodrain.com/zengqg-test/etcd:v2.2.0", "/tmp/testsaveimage.tar", nil); err != nil {
			t.Fatal(err)
		}
	*/
}

// capability_id: rainbond.source-image.multi-save
func TestMulitImageSave(t *testing.T) {
	t.Skip("requires local docker daemon")
	/*
		dc, _ := client.NewEnvClient()
		if err := MultiImageSave(context.Background(), dc, "/tmp/testsaveimage.tar", nil,
			"registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-node:V5.3.0-cloud",
			"registry.cn-hangzhou.aliyuncs.com/goodrain/rbd-resource-proxy:V5.3.0-cloud"); err != nil {
			t.Fatal(err)
		}
	*/
}

// capability_id: rainbond.source-image.import
func TestImageImport(t *testing.T) {
	t.Skip("requires local docker daemon")
	/*
		dc, _ := client.NewEnvClient()
		if err := ImageImport(dc, "hub.goodrain.com/zengqg-test/etcd:v2.2.0", "/tmp/testsaveimage.tar", nil); err != nil {
			t.Fatal(err)
		}
	*/
}
