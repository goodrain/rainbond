package rbdcomponent

import (
	"github.com/spf13/pflag"
	"path"
)

type WorkerConfig struct {
	ClusterName             string
	EventLogServers         []string
	MaxTasks                int
	NodeName                string
	Listen                  string
	ServerPort              int
	SharedStorageClass      string
	LeaderElectionNamespace string
	LeaderElectionIdentity  string
	Helm                    Helm
}

// Helm helm configuration.
type Helm struct {
	DataDir    string
	RepoFile   string
	RepoCache  string
	ChartCache string
}

func AddWorkerFlags(fs *pflag.FlagSet, wc *WorkerConfig) {
	fs.StringVar(&wc.Listen, "listen", ":6369", "prometheus listen host and port")
	fs.IntVar(&wc.MaxTasks, "max-tasks", 50, "the max tasks for per node")
	fs.StringVar(&wc.NodeName, "node-name", "", "the name of this worker,it must be global unique name")
	fs.IntVar(&wc.ServerPort, "server-port", 6535, "the listen port that app runtime server")
	fs.StringVar(&wc.LeaderElectionNamespace, "leader-election-namespace", "rbd-system", "Namespace where this attacher runs.")
	fs.StringVar(&wc.LeaderElectionIdentity, "leader-election-identity", "", "Unique idenity of this attcher. Typically name of the pod where the attacher runs.")
	fs.StringVar(&wc.Helm.DataDir, "/grdata/helm", "/grdata/helm", "The data directory of Helm.")
	fs.StringVar(&wc.SharedStorageClass, "shared-storageclass", "", "custom shared storage class.use the specified storageclass to create shared storage, if this parameter is not specified, it will use rainbondsssc by default")
	wc.Helm.RepoFile = path.Join(wc.Helm.DataDir, "repo/repositories.yaml")
	wc.Helm.RepoCache = path.Join(wc.Helm.DataDir, "cache")
	wc.Helm.ChartCache = path.Join(wc.Helm.DataDir, "chart")
}
