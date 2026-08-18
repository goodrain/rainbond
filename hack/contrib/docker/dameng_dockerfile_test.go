package docker

import (
	"os"
	"strings"
	"testing"
)

// capability_id: rainbond.database.dameng-uncompressed-images
func TestDamengImagesSkipUPXCompression(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "api", path: "api/Dockerfile"},
		{name: "worker", path: "worker/Dockerfile"},
		{name: "chaos", path: "chaos/Dockerfile"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contents, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatalf("read Dockerfile: %v", err)
			}

			text := string(contents)
			for _, expected := range []string{
				"ARG ENABLE_DM=false",
				"sh scripts/prepare-dameng-go-driver.sh",
				"go mod edit -require=dm@v0.0.0 -replace=dm=./third_party/dameng/dm",
				`-tags "dm sqlite_omit_load_extension netgo"`,
			} {
				if !strings.Contains(text, expected) {
					t.Fatalf("Dockerfile must build a Dameng variant containing %q", expected)
				}
			}
			compressStageIndex := strings.Index(text, "FROM ubuntu:24.04 AS compress")
			if compressStageIndex < 0 {
				t.Fatal("Dockerfile must have a compression stage")
			}
			compressStage := text[compressStageIndex:]
			if !strings.HasPrefix(compressStage, "FROM ubuntu:24.04 AS compress\nARG ENABLE_DM=false") {
				t.Fatal("compress stage must receive ENABLE_DM")
			}
			guardIndex := strings.Index(compressStage, "if [ \"$ENABLE_DM\" = \"true\" ]; then")
			if guardIndex < 0 {
				t.Fatal("Dockerfile must guard compression for a Dameng image")
			}
			upxIndex := strings.Index(compressStage, "upx --best --lzma")
			if upxIndex < 0 {
				t.Fatal("default image must retain UPX compression")
			}
			elseIndex := strings.Index(compressStage[guardIndex:], "else")
			if elseIndex < 0 || guardIndex+elseIndex > upxIndex {
				t.Fatal("UPX compression must remain in the non-Dameng branch")
			}
		})
	}
}
