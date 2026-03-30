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
	"testing"
)

// capability_id: rainbond.util.ssh.auth-method-selection
func TestNewSSHClientSelectsAuthMethod(t *testing.T) {
	passwordClient := NewSSHClient("127.0.0.1", "root", "secret", "whoami", 22, nil, nil)
	if passwordClient.Method != "password" {
		t.Fatalf("expected password auth, got %q", passwordClient.Method)
	}

	publicKeyClient := NewSSHClient("127.0.0.1", "root", "", "whoami", 22, nil, nil)
	if publicKeyClient.Method != "publickey" {
		t.Fatalf("expected publickey auth, got %q", publicKeyClient.Method)
	}
}

// capability_id: rainbond.util.ssh.auth-method-selection
func TestParseAuthMethodsRejectsInvalidMethod(t *testing.T) {
	_, err := parseAuthMethods(&SSHClient{Method: "token"})
	if err == nil {
		t.Fatal("expected invalid auth method error")
	}
}
