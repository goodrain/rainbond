package rbdcomponent

import (
	"fmt"
	"time"
	_ "time/tzdata" // Validate IANA zones even in images without system tzdata.

	"github.com/spf13/pflag"
)

// ChaosConfig configures build execution and coordinated cleanup.
type ChaosConfig struct {
	RunMode          string
	ClusterName      string
	BuildKitImage    string
	BuildKitArgs     string
	BuildKitCache    bool
	MaxTasks         int
	DockerEndpoint   string
	CleanUp          bool // Deprecated: legacy cleanup is never executed.
	ScheduledCleanUp bool
	Topic            string
	RbdRepoName      string
	GRDataPVCName    string
	CachePVCName     string
	CacheMode        string
	CachePath        string
	ContainerRuntime string
	RuntimeEndpoint  string
	KeepCount        int
	CleanInterval    int // Deprecated: retained for flag compatibility; daily scheduling ignores it.
	CleanTime        string
	CleanTimeZone    string
	BRVersion        string
	SourceScanURL    string
	RegistryMirrors  string
}

// AddChaosFlags registers build and cleanup command-line options.
func AddChaosFlags(fs *pflag.FlagSet, cc *ChaosConfig) {
	fs.StringVar(&cc.BuildKitImage, "buildkit-image", "registry.cn-hangzhou.aliyuncs.com/goodrain/buildkit:v0.12.0", "buildkit image version")
	fs.IntVar(&cc.MaxTasks, "max-tasks", 50, "Maximum number of simultaneous build tasks")
	fs.StringVar(&cc.RunMode, "run", "sync", "sync data when worker start")
	fs.StringVar(&cc.DockerEndpoint, "dockerd", "127.0.0.1:2376", "dockerd endpoint")
	fs.BoolVar(&cc.CleanUp, "clean-up", false, "deprecated legacy cleanup switch; ignored")
	_ = fs.MarkDeprecated("clean-up", "ignored; use --scheduled-clean-up for coordinated daily cleanup")
	fs.BoolVar(&cc.ScheduledCleanUp, "scheduled-clean-up", true, "run coordinated cleanup once on startup and daily at the configured time")
	fs.StringVar(&cc.Topic, "topic", "builder", "Topic in mq,you coule choose `builder` or `windows_builder`")
	fs.StringVar(&cc.RbdRepoName, "rbd-repo", "rbd-repo", "rbd component repo's name")
	fs.StringVar(&cc.GRDataPVCName, "pvc-grdata-name", "grdata", "pvc name of grdata")
	fs.StringVar(&cc.CachePVCName, "pvc-cache-name", "cache", "pvc name of cache")
	fs.StringVar(&cc.CacheMode, "cache-mode", "sharefile", "volume cache mount type, can be hostpath and sharefile, default is sharefile, which mount using pvc")
	fs.StringVar(&cc.CachePath, "cache-path", "/cache", "volume cache mount path, when cache-mode using hostpath, default path is /cache")
	fs.StringVar(&cc.ContainerRuntime, "container-runtime", "containerd", "container runtime, support docker and containerd")
	fs.StringVar(&cc.RuntimeEndpoint, "runtime-endpoint", "/run/containerd/containerd.sock", "container runtime endpoint")
	fs.StringVar(&cc.BuildKitArgs, "buildkit-args", "", "buildkit build image container args config,need '&' split")
	fs.BoolVar(&cc.BuildKitCache, "buildkit-cache", false, "whether to enable the buildkit image cache")
	fs.IntVar(&cc.KeepCount, "keep-count", 5, "default number of reserved copies for images")
	fs.IntVar(&cc.CleanInterval, "clean-interval", 60, "deprecated cleanup interval; ignored by daily scheduling")
	_ = fs.MarkDeprecated("clean-interval", "ignored; use --clean-time and --clean-timezone")
	cc.CleanTime = "03:00"
	cc.CleanTimeZone = "Asia/Shanghai"
	fs.Var(&validatedCleanupString{value: &cc.CleanTime, validate: validateCleanupTime}, "clean-time", "daily cleanup wall-clock time in HH:MM")
	fs.Var(&validatedCleanupString{value: &cc.CleanTimeZone, validate: validateCleanupTimeZone}, "clean-timezone", "IANA time zone for daily cleanup")
	fs.StringVar(&cc.BRVersion, "br-version", "stable", "builder and runner version")
	fs.StringVar(&cc.SourceScanURL, "source-scan-url", "", "rainbond source scan service URL, eg: http://rainbond-sourcescan:8080")
	fs.StringVar(&cc.RegistryMirrors, "registry-mirrors", "", "comma-separated registry mirrors for docker.io base-image pulls in dockerfile builds, empty means no mirror; can be overridden by env REGISTRY_MIRRORS. prefix a value with http:// to mark it as a plain-HTTP mirror endpoint")
}

type validatedCleanupString struct {
	value    *string
	validate func(string) error
}

func (v *validatedCleanupString) String() string { return *v.value }
func (v *validatedCleanupString) Type() string   { return "string" }
func (v *validatedCleanupString) Set(value string) error {
	if err := v.validate(value); err != nil {
		return err
	}
	*v.value = value
	return nil
}

func validateCleanupTime(value string) error {
	if len(value) != 5 || value[2] != ':' ||
		value[0] < '0' || value[0] > '9' || value[1] < '0' || value[1] > '9' ||
		value[3] < '0' || value[3] > '9' || value[4] < '0' || value[4] > '9' {
		return fmt.Errorf("invalid cleanup time %q: expected HH:MM", value)
	}
	if _, err := time.Parse("15:04", value); err != nil {
		return fmt.Errorf("invalid cleanup time %q: %w", value, err)
	}
	return nil
}

func validateCleanupTimeZone(value string) error {
	if value == "" || value == "Local" {
		return fmt.Errorf("invalid cleanup time zone %q: use an explicit IANA zone or UTC", value)
	}
	if _, err := time.LoadLocation(value); err != nil {
		return fmt.Errorf("invalid cleanup time zone %q: %w", value, err)
	}
	return nil
}
