package configs

import (
	"github.com/goodrain/rainbond-operator/util/constants"
	utils "github.com/goodrain/rainbond/util"
	"github.com/spf13/pflag"
)

type PublicConfig struct {
	RbdNamespace  string
	GrdataPVCName string
	HostIP        string
}

func AddPublicFlags(fs *pflag.FlagSet, pc *PublicConfig) {
	fs.StringVar(&pc.RbdNamespace, "rbd-namespace", utils.GetenvDefault("RBD_NAMESPACE", constants.Namespace), "rbd component namespace")
	fs.StringVar(&pc.GrdataPVCName, "grdata-pvc-name", "rbd-cpt-grdata", "The name of grdata persistent volume claim")
	fs.StringVar(&pc.HostIP, "hostIP", "", "Current node Intranet IP")
}
