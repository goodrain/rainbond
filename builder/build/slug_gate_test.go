package build_test

import (
	"testing"

	buildpkg "github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/build/cnb"
	"github.com/goodrain/rainbond/builder/parser/code"
)

func TestGetBuildByType_SlugRemovalGate(t *testing.T) {
	t.Run("keeps slug source build available when legacy gate env is enabled", func(t *testing.T) {
		t.Setenv(buildpkg.SourceSlugRemovalGateEnv, "true")

		_, err := buildpkg.GetBuildByType(code.JavaMaven, "slug")
		if err != nil {
			t.Fatalf("expected legacy slug build to stay available, got %v", err)
		}
	})

	t.Run("keeps cnb source build available when removal gate is enabled", func(t *testing.T) {
		t.Setenv(buildpkg.SourceSlugRemovalGateEnv, "true")

		builder, err := buildpkg.GetBuildByType(code.Python, "cnb")
		if err != nil {
			t.Fatalf("expected cnb build to stay available, got error %v", err)
		}
		if _, ok := builder.(*cnb.Builder); !ok {
			t.Fatalf("expected CNB builder, got %T", builder)
		}
	})
}
