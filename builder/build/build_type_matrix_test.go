package build_test

import (
	"testing"

	buildpkg "github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/build/cnb"
	"github.com/goodrain/rainbond/builder/parser/code"
)

func TestGetBuildByType_SourceBuildLanguageMatrix(t *testing.T) {
	tests := []struct {
		name    string
		lang    code.Lang
		buildTy string
		wantCNB bool
	}{
		{name: "nodejs cnb", lang: code.Nodejs, buildTy: "cnb", wantCNB: true},
		{name: "static cnb", lang: code.Static, buildTy: "cnb", wantCNB: true},
		{name: "composite dockerfile nodejs cnb", lang: code.Lang("dockerfile,Node.js"), buildTy: "cnb", wantCNB: true},
		{name: "java-maven cnb", lang: code.JavaMaven, buildTy: "cnb", wantCNB: true},
		{name: "python cnb", lang: code.Python, buildTy: "cnb", wantCNB: true},
		{name: "php cnb", lang: code.PHP, buildTy: "cnb", wantCNB: true},
		{name: "go cnb", lang: code.Golang, buildTy: "cnb", wantCNB: true},
		{name: "netcore cnb", lang: code.NetCore, buildTy: "cnb", wantCNB: true},
		{name: "dockerfile cnb falls back", lang: code.Dockerfile, buildTy: "cnb", wantCNB: false},
		{name: "nodejs slug", lang: code.Nodejs, buildTy: "slug", wantCNB: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder, err := buildpkg.GetBuildByType(tt.lang, tt.buildTy)
			if err != nil {
				t.Fatalf("GetBuildByType(%q, %q) error = %v", tt.lang, tt.buildTy, err)
			}
			_, isCNB := builder.(*cnb.Builder)
			if isCNB != tt.wantCNB {
				t.Fatalf("GetBuildByType(%q, %q) CNB = %v, want %v", tt.lang, tt.buildTy, isCNB, tt.wantCNB)
			}
		})
	}
}
