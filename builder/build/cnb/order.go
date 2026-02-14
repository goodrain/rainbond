package cnb

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/sirupsen/logrus"
)

// orderBuildpack represents a buildpack entry in a custom order.toml.
type orderBuildpack struct {
	ID       string
	Version  string
	Optional bool
}

// writeCustomOrder writes a custom order.toml to the source directory.
// Returns the -order flag value for lifecycle creator, or empty string on failure.
func (b *Builder) writeCustomOrder(re *build.Request, buildpacks []orderBuildpack, desc string) string {
	var buf strings.Builder
	buf.WriteString("[[order]]\n")
	for _, bp := range buildpacks {
		buf.WriteString("  [[order.group]]\n")
		buf.WriteString(fmt.Sprintf("    id = \"%s\"\n", bp.ID))
		if bp.Version != "" {
			buf.WriteString(fmt.Sprintf("    version = \"%s\"\n", bp.Version))
		}
		if bp.Optional {
			buf.WriteString("    optional = true\n")
		}
	}

	orderPath := filepath.Join(re.SourceDir, ".cnb-order.toml")
	if err := os.WriteFile(orderPath, []byte(buf.String()), 0644); err != nil {
		logrus.Warnf("failed to write custom order.toml for %s: %v", desc, err)
		return ""
	}
	logrus.Infof("%s: using custom order.toml", desc)
	return "-order=/workspace/.cnb-order.toml"
}
