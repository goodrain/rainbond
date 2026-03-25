package cnb

import (
	"fmt"
	"strings"

	"github.com/goodrain/rainbond/builder/build"
	"github.com/goodrain/rainbond/builder/parser/code"
)

type versionPolicyRule struct {
	policyKey    string
	explicitKeys []string
	bpKey        string
	setKeys      []string
	ossDefault   string
}

var versionPolicyRules = map[code.Lang]versionPolicyRule{
	code.JavaMaven: {policyKey: "java", explicitKeys: []string{"BUILD_RUNTIMES", "RUNTIMES", "BP_JVM_VERSION"}, bpKey: "BP_JVM_VERSION", setKeys: []string{"BUILD_RUNTIMES", "BP_JVM_VERSION"}, ossDefault: "17"},
	code.JaveWar:   {policyKey: "java", explicitKeys: []string{"BUILD_RUNTIMES", "RUNTIMES", "BP_JVM_VERSION"}, bpKey: "BP_JVM_VERSION", setKeys: []string{"BUILD_RUNTIMES", "BP_JVM_VERSION"}, ossDefault: "17"},
	code.JavaJar:   {policyKey: "java", explicitKeys: []string{"BUILD_RUNTIMES", "RUNTIMES", "BP_JVM_VERSION"}, bpKey: "BP_JVM_VERSION", setKeys: []string{"BUILD_RUNTIMES", "BP_JVM_VERSION"}, ossDefault: "17"},
	code.Gradle:    {policyKey: "java", explicitKeys: []string{"BUILD_RUNTIMES", "RUNTIMES", "BP_JVM_VERSION"}, bpKey: "BP_JVM_VERSION", setKeys: []string{"BUILD_RUNTIMES", "BP_JVM_VERSION"}, ossDefault: "17"},
	code.Python:    {policyKey: "python", explicitKeys: []string{"BUILD_RUNTIMES", "RUNTIMES", "BP_CPYTHON_VERSION"}, bpKey: "BP_CPYTHON_VERSION", setKeys: []string{"BUILD_RUNTIMES", "BP_CPYTHON_VERSION"}, ossDefault: "3.11"},
	code.Golang:    {policyKey: "golang", explicitKeys: []string{"BUILD_GOVERSION", "GOVERSION", "BP_GO_VERSION"}, bpKey: "BP_GO_VERSION", setKeys: []string{"BUILD_GOVERSION", "BP_GO_VERSION"}, ossDefault: "1.23"},
	code.PHP:       {policyKey: "php", explicitKeys: []string{"BUILD_RUNTIMES", "RUNTIMES", "BP_PHP_VERSION"}, bpKey: "BP_PHP_VERSION", setKeys: []string{"BUILD_RUNTIMES", "BP_PHP_VERSION"}, ossDefault: "8.2"},
	code.Nodejs:    {policyKey: "nodejs", explicitKeys: []string{"CNB_NODE_VERSION", "BUILD_RUNTIMES", "RUNTIMES", "BP_NODE_VERSION"}, bpKey: "BP_NODE_VERSION", setKeys: []string{"CNB_NODE_VERSION", "BP_NODE_VERSION"}, ossDefault: "24.13.0"},
}

func applyVersionPolicy(re *build.Request) error {
	if strings.TrimSpace(re.BuildStrategy) != "cnb" {
		return nil
	}

	rule, ok := versionPolicyRules[re.Lang]
	if !ok {
		return nil
	}

	resolved, err := resolveVersionWithPolicy(re, rule)
	if err != nil {
		return err
	}
	if resolved == "" {
		return nil
	}

	if re.BuildEnvs == nil {
		re.BuildEnvs = map[string]string{}
	}
	re.BuildEnvs[rule.bpKey] = resolved
	for _, key := range rule.setKeys {
		re.BuildEnvs[key] = resolved
	}
	return nil
}

func resolveVersionWithPolicy(re *build.Request, rule versionPolicyRule) (string, error) {
	policy := getLanguagePolicy(re.CNBVersionPolicy, rule.policyKey)

	if explicit := firstNonEmptyEnv(re.BuildEnvs, rule.explicitKeys...); explicit != "" {
		resolved, err := normalizeVersionByLanguage(re.Lang, explicit)
		if err != nil {
			return "", err
		}
		if err := ensureAllowedByPolicy(rule.policyKey, resolved, policy); err != nil {
			return "", err
		}
		return resolved, nil
	}

	detected, err := detectVersionFromSource(re)
	if err != nil {
		return "", err
	}
	if detected != "" {
		if err := ensureAllowedByPolicy(rule.policyKey, detected, policy); err != nil {
			return "", err
		}
		return detected, nil
	}

	if policy != nil && policy.DefaultVersion != "" {
		if err := ensureAllowedByPolicy(rule.policyKey, policy.DefaultVersion, policy); err != nil {
			return "", err
		}
		return policy.DefaultVersion, nil
	}

	return rule.ossDefault, nil
}

func detectVersionFromSource(re *build.Request) (string, error) {
	runtimeInfo, err := code.CheckRuntimeByStrategy(re.SourceDir, re.Lang, "cnb")
	if err != nil {
		return "", err
	}
	switch re.Lang {
	case code.Golang:
		return strings.TrimSpace(runtimeInfo["GOVERSION"]), nil
	default:
		return strings.TrimSpace(runtimeInfo["RUNTIMES"]), nil
	}
}

func getLanguagePolicy(policy *build.CNBVersionPolicy, key string) *build.CNBLanguagePolicy {
	if policy == nil || len(policy.Languages) == 0 {
		return nil
	}
	langPolicy, ok := policy.Languages[key]
	if !ok {
		return nil
	}
	return &langPolicy
}

func firstNonEmptyEnv(envs map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(envs[key]); value != "" {
			return value
		}
	}
	return ""
}

func ensureAllowedByPolicy(policyKey, version string, policy *build.CNBLanguagePolicy) error {
	if policy == nil || len(policy.AllowedVersions) == 0 || version == "" {
		return nil
	}
	for _, item := range policy.AllowedVersions {
		if item == version {
			return nil
		}
	}
	return fmt.Errorf("%s version %s is not allowed by cnb version policy", policyKey, version)
}

func normalizeVersionByLanguage(lang code.Lang, version string) (string, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return "", nil
	}

	switch lang {
	case code.JavaMaven, code.JaveWar, code.JavaJar, code.Gradle:
		if strings.HasPrefix(version, "1.") {
			version = version[2:]
		}
		parts := strings.Split(version, ".")
		if len(parts) < 1 || parts[0] == "" {
			return "", fmt.Errorf("invalid java cnb version %q", version)
		}
		return parts[0], nil
	case code.Python:
		if strings.HasPrefix(version, "python-") {
			version = version[len("python-"):]
		}
		parts := strings.Split(version, ".")
		if len(parts) < 2 {
			return "", fmt.Errorf("invalid python cnb version %q", version)
		}
		return strings.Join(parts[:2], "."), nil
	case code.Golang:
		if strings.HasPrefix(version, "go") {
			version = version[2:]
		}
		parts := strings.Split(version, ".")
		if len(parts) < 2 {
			return "", fmt.Errorf("invalid golang cnb version %q", version)
		}
		return strings.Join(parts[:2], "."), nil
	case code.PHP:
		parts := strings.Split(version, ".")
		if len(parts) < 2 {
			return "", fmt.Errorf("invalid php cnb version %q", version)
		}
		return strings.Join(parts[:2], "."), nil
	case code.Nodejs:
		return normalizeNodeVersion(version)
	default:
		return version, nil
	}
}

func normalizeNodeVersion(version string) (string, error) {
	if strings.TrimSpace(version) == "" {
		return "", nil
	}
	if !strings.ContainsAny(version, "0123456789") {
		return "", fmt.Errorf("invalid nodejs cnb version %q", version)
	}
	resolved := code.MatchCNBVersion("nodejs", version)
	if resolved == "" {
		return "", fmt.Errorf("invalid nodejs cnb version %q", version)
	}
	return resolved, nil
}
