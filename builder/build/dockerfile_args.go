package build

import (
	"fmt"
	"sort"
	"strings"
)

// Console stores BUILD_ARG_* keys. Older callers send ARG_* directly; accept
// both, with Console's explicit value taking precedence even when it is empty.
func dockerfileBuildArgs(envs map[string]string) []string {
	values := make(map[string]string)
	for _, prefix := range []string{"ARG_", "BUILD_ARG_"} {
		for key, value := range envs {
			if strings.HasPrefix(key, prefix) {
				name := strings.TrimPrefix(key, prefix)
				if name != "" {
					values[name] = value
				}
			}
		}
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	args := make([]string, 0, len(names))
	for _, name := range names {
		args = append(args, fmt.Sprintf("--opt=build-arg:%s=%s", name, values[name]))
	}
	return args
}
