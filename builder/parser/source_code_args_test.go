package parser

import (
	"strings"
	"testing"

	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/goodrain/rainbond/builder/parser/types"
)

func TestGetArgs_MultiLanguage(t *testing.T) {
	tests := []struct {
		name    string
		lang    code.Lang
		wantNil bool
	}{
		// 单语言
		{"pure Node.js returns nil", code.Nodejs, true},
		{"pure static returns nil", code.Static, true},
		{"pure Python returns args", code.Python, false},
		{"pure dockerfile returns args", code.Dockerfile, false},
		{"Java-maven returns args", code.JavaMaven, false},
		// 多语言（逗号分隔）
		{"dockerfile,Node.js returns nil", code.Lang("dockerfile,Node.js"), true},
		{"dockerfile,static returns nil", code.Lang("dockerfile,static"), true},
		{"Node.js,dockerfile returns nil", code.Lang("Node.js,dockerfile"), true},
		// 不应误匹配的语言（大小写敏感）
		{"NodeJSStatic not matched (no regression)", code.NodeJSStatic, false},
		{"NodeJSDockerfile not matched", code.NodeJSDockerfile, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &SourceCodeParse{
				Lang: tt.lang,
				args: []string{"start", "web"},
			}
			got := d.GetArgs()
			if tt.wantNil && got != nil {
				t.Errorf("GetArgs() = %v; want nil", got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("GetArgs() = nil; want non-nil")
			}
		})
	}
}

func TestCNBDefaultPorts_MultiLanguage(t *testing.T) {
	tests := []struct {
		name        string
		lang        code.Lang
		runtimeType string // "" means no runtimeInfo
		wantPort    int    // 0 means no CNB port expected
	}{
		// 纯语言
		{"static gets 8080", code.Static, "", 8080},
		{"Node.js static gets 8080", code.Nodejs, "static", 8080},
		{"Node.js dynamic gets 3000", code.Nodejs, "dynamic", 3000},
		{"dockerfile gets no port", code.Dockerfile, "", 0},
		// 多语言
		{"dockerfile,Node.js static gets 8080", code.Lang("dockerfile,Node.js"), "static", 8080},
		{"dockerfile,Node.js dynamic gets 3000", code.Lang("dockerfile,Node.js"), "dynamic", 3000},
		{"dockerfile,static gets 8080", code.Lang("dockerfile,static"), "", 8080},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &SourceCodeParse{
				Lang:  tt.lang,
				ports: make(map[int]*types.Port),
			}
			applyCNBDefaultPorts(d, tt.runtimeType)

			if tt.wantPort == 0 {
				if len(d.ports) != 0 {
					t.Errorf("expected no ports, got %v", d.ports)
				}
			} else {
				if _, ok := d.ports[tt.wantPort]; !ok {
					t.Errorf("expected port %d, got %v", tt.wantPort, d.ports)
				}
			}
		})
	}
}

// applyCNBDefaultPorts mirrors the port logic in source_code.go Parse() for testability.
func applyCNBDefaultPorts(d *SourceCodeParse, runtimeType string) {
	var runtimeInfo map[string]string
	if runtimeType != "" {
		runtimeInfo = map[string]string{"RUNTIME_TYPE": runtimeType}
	}

	langStr := string(d.Lang)
	hasNodejs := strings.Contains(langStr, string(code.Nodejs))
	hasStatic := strings.Contains(langStr, string(code.Static))
	if hasStatic || (hasNodejs && runtimeInfo != nil && runtimeInfo["RUNTIME_TYPE"] == "static") {
		if _, ok := d.ports[8080]; !ok {
			d.ports[8080] = &types.Port{ContainerPort: 8080, Protocol: "http"}
		}
	}
	if hasNodejs && runtimeInfo != nil && runtimeInfo["RUNTIME_TYPE"] == "dynamic" {
		if _, ok := d.ports[3000]; !ok {
			d.ports[3000] = &types.Port{ContainerPort: 3000, Protocol: "http"}
		}
	}
}
