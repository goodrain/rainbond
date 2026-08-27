package parser

import (
	"testing"

	"github.com/goodrain/rainbond/builder/parser/code"
	"github.com/goodrain/rainbond/builder/parser/types"
)

// capability_id: rainbond.source-args.multi-language
func TestGetArgs_MultiLanguage(t *testing.T) {
	tests := []struct {
		name          string
		lang          code.Lang
		buildStrategy string
		wantNil       bool
	}{
		// 单语言
		{"pure Node.js returns nil", code.Nodejs, "", true},
		{"pure static returns nil", code.Static, "", true},
		{"pure Python returns args", code.Python, "", false},
		{"pure Python cnb returns nil", code.Python, "cnb", true},
		{"pure dockerfile returns args", code.Dockerfile, "", false},
		{"Java-maven returns args", code.JavaMaven, "", false},
		{"Java-maven cnb returns nil", code.JavaMaven, "cnb", true},
		// 多语言（逗号分隔）
		{"dockerfile,Node.js returns nil", code.Lang("dockerfile,Node.js"), "", true},
		{"dockerfile,static returns nil", code.Lang("dockerfile,static"), "", true},
		{"Node.js,dockerfile returns nil", code.Lang("Node.js,dockerfile"), "", true},
		// 不应误匹配的语言（大小写敏感）
		{"NodeJSStatic not matched (no regression)", code.NodeJSStatic, "", false},
		{"NodeJSDockerfile not matched", code.NodeJSDockerfile, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &SourceCodeParse{
				Lang:          tt.lang,
				buildStrategy: tt.buildStrategy,
				args:          []string{"start", "web"},
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

// capability_id: rainbond.source-args.default-cnb-ports
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

func TestSourceCodeParseApplyCNBDefaultPorts_JavaWarUses8080(t *testing.T) {
	d := &SourceCodeParse{
		Lang:  code.JaveWar,
		ports: make(map[int]*types.Port),
	}

	d.applyCNBDefaultPorts(nil)

	port, ok := d.ports[8080]
	if !ok {
		t.Fatalf("expected Java-war CNB default port 8080, got %v", d.ports)
	}
	if port.Protocol != "http" {
		t.Fatalf("expected Java-war CNB default port protocol http, got %q", port.Protocol)
	}
}

// capability_id: rainbond.source-detect.multi-module-dockerfile
func TestGetServiceInfo_MultiModulesPreserveDockerfileDetection(t *testing.T) {
	d := &SourceCodeParse{
		ports:       make(map[int]*types.Port),
		volumes:     make(map[string]*types.Volume),
		envs:        make(map[string]*types.Env),
		image:       Image{},
		Lang:        code.Lang("dockerfile,Java-maven"),
		dockerfiles: []string{"Dockerfile"},
		isMulti:     true,
		services: []*types.Service{
			{Name: "service-a", Cname: "service-a", Packaging: "jar", ModuleRole: types.ModuleRoleRunnable},
			{Name: "service-b", Cname: "service-b", Packaging: "jar", ModuleRole: types.ModuleRolePossibleDependency},
		},
	}

	got := d.GetServiceInfo()
	if len(got) != 2 {
		t.Fatalf("GetServiceInfo() returned %d services, want 2", len(got))
	}
	for idx, info := range got {
		if info.Lang != code.Lang("dockerfile,Java-maven") {
			t.Fatalf("service %q language = %q, want dockerfile,Java-maven", info.Name, info.Lang)
		}
		if len(info.Dockerfiles) != 1 || info.Dockerfiles[0] != "Dockerfile" {
			t.Fatalf("service %q Dockerfiles = %v, want [Dockerfile]", info.Name, info.Dockerfiles)
		}
		if info.ModuleRole != d.services[idx].ModuleRole {
			t.Fatalf("service %q module role = %q, want %q", info.Name, info.ModuleRole, d.services[idx].ModuleRole)
		}
	}
}

func applyCNBDefaultPorts(d *SourceCodeParse, runtimeType string) {
	var runtimeInfo map[string]string
	if runtimeType != "" {
		runtimeInfo = map[string]string{"RUNTIME_TYPE": runtimeType}
	}

	d.applyCNBDefaultPorts(runtimeInfo)
}
