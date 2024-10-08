package rbdcomponent

import (
	"github.com/spf13/pflag"
)

type ChaosConfig struct {
	RunMode          string
	ClusterName      string
	BuildKitImage    string
	BuildKitArgs     string
	BuildKitCache    bool
	MaxTasks         int
	DockerEndpoint   string
	CleanUp          bool
	Topic            string
	RbdRepoName      string
	GRDataPVCName    string
	CachePVCName     string
	CacheMode        string
	CachePath        string
	ContainerRuntime string
	RuntimeEndpoint  string
	KeepCount        int
	CleanInterval    int
	BRVersion        string
}

func AddChaosFlags(fs *pflag.FlagSet, cc *ChaosConfig) {
	fs.StringVar(&cc.BuildKitImage, "buildkit-image", "registry.cn-hangzhou.aliyuncs.com/goodrain/buildkit:v0.12.0", "buildkit image version")
	fs.IntVar(&cc.MaxTasks, "max-tasks", 50, "Maximum number of simultaneous build tasks")
	fs.StringVar(&cc.RunMode, "run", "sync", "sync data when worker start")
	fs.StringVar(&cc.DockerEndpoint, "dockerd", "127.0.0.1:2376", "dockerd endpoint")
	fs.BoolVar(&cc.CleanUp, "clean-up", true, "Turn on build version cleanup")
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
	fs.IntVar(&cc.CleanInterval, "clean-interval", 60, "clean image interval,default 60 minute")
	fs.StringVar(&cc.BRVersion, "br-version", "v5.16.0-release", "builder and runner version")
}
