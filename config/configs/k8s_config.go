package configs

import "github.com/spf13/pflag"

type K8SConfig struct {
	KubeConfigPath string
	KubeAPIQPS     int
	KubeAPIBurst   int
}

func AddK8SFlags(fs *pflag.FlagSet, k8sc *K8SConfig) {
	fs.StringVar(&k8sc.KubeConfigPath, "kube-config", "", "kube config file path, No setup is required to run in a cluster.")
	fs.IntVar(&k8sc.KubeAPIQPS, "kube-api-qps", 50, "kube client qps")
	fs.IntVar(&k8sc.KubeAPIBurst, "kube-api-burst", 10, "kube clint burst")
}
