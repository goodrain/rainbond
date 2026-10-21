package cnb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/parser/code"
)

type cnbCachePolicy struct {
	registryCache bool
	reuseImage    bool
	reason        string
}

func resolveCNBCachePolicy(re *build.Request) cnbCachePolicy {
	if truthyBuildEnv(re.BuildEnvs["NO_CACHE"]) || truthyBuildEnv(re.BuildEnvs["BUILD_NO_CACHE"]) {
		return cnbCachePolicy{reason: "disabled by build configuration"}
	}
	policy := cnbCachePolicy{
		registryCache: true,
		reuseImage:    true,
		reason:        "reusable build cache retained",
	}
	if usesNPMInstall(re) {
		policy.registryCache = false
		policy.reason = "npm install rebuilds dependencies; avoiding remote cache restore and export"
	}
	return policy
}

// usesNPMInstall only bypasses caching when the project clearly selects the
// Paketo npm install path, which resets restored build-modules on every build.
func usesNPMInstall(re *build.Request) bool {
	if re.SourceDir == "" || (re.Lang != "" && re.Lang != code.Nodejs) {
		return false
	}
	dir := re.SourceDir
	if sub := strings.TrimSpace(re.BuildEnvs["BP_NODE_PROJECT_PATH"]); sub != "" {
		if !filepath.IsLocal(sub) {
			return false
		}
		dir = filepath.Join(dir, sub)
	}
	contents, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil || !json.Valid(contents) {
		return false
	}
	// Keep existing behavior for lock files, vendored modules, other package
	// managers, and inspection errors instead of assuming the cache is useless.
	for _, name := range []string{"package-lock.json", "npm-shrinkwrap.json", "node_modules", "yarn.lock", "pnpm-lock.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			return false
		}
	}
	return code.DetectPackageManager(dir).Manager == code.PackageManagerNPM
}

func (p cnbCachePolicy) log(re *build.Request) {
	if re.Logger == nil {
		return
	}
	registry, image := "disabled", "disabled"
	if p.registryCache {
		registry = "enabled"
	}
	if p.reuseImage {
		image = "enabled"
	}
	re.Logger.Info(fmt.Sprintf("CNB cache strategy: registry cache %s; image layer reuse %s (%s)", registry, image, p.reason),
		map[string]string{"step": "build-cache"})
}
