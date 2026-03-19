package exector

import (
	"testing"

	"github.com/goodrain/rainbond/builder/build"
)

func TestNormalizeSourceBuildCmd(t *testing.T) {
	tests := []struct {
		name           string
		cmd            string
		lang           string
		buildType      string
		buildMode      string
		dockerfilePath string
		mediumType     build.MediumType
		want           string
	}{
		{
			name:       "keeps slug default command for slug medium",
			cmd:        "start web",
			lang:       "dockerfile,Python",
			mediumType: build.SlugMediumType,
			want:       "start web",
		},
		{
			name:       "clears dockerfile composite default command for image medium",
			cmd:        "start web",
			lang:       "dockerfile,Python",
			mediumType: build.ImageMediumType,
			want:       "",
		},
		{
			name:       "clears cnb default command for image medium",
			cmd:        "start web",
			lang:       "Node.js",
			buildType:  "cnb",
			mediumType: build.ImageMediumType,
			want:       "",
		},
		{
			name:       "clears dockerfile mode default command for image medium",
			cmd:        " start   web ",
			lang:       "Node.js",
			buildMode:  "DOCKERFILE",
			mediumType: build.ImageMediumType,
			want:       "",
		},
		{
			name:           "clears selected dockerfile path default command for image medium",
			cmd:            "start web",
			lang:           "dockerfile",
			dockerfilePath: "services/api/Dockerfile",
			mediumType:     build.ImageMediumType,
			want:           "",
		},
		{
			name:       "keeps unrelated image command",
			cmd:        "python app.py",
			lang:       "Python",
			mediumType: build.ImageMediumType,
			want:       "python app.py",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeSourceBuildCmd(
				tt.cmd,
				tt.lang,
				tt.buildType,
				tt.buildMode,
				tt.dockerfilePath,
				tt.mediumType,
			)
			if got != tt.want {
				t.Fatalf("normalizeSourceBuildCmd() = %q, want %q", got, tt.want)
			}
		})
	}
}
