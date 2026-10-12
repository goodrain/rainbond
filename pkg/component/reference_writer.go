package component

import (
	"context"
	"os"

	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/pkg/cleanup/kubeidentity"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/goodrain/rainbond/pkg/rainbond"
	"github.com/sirupsen/logrus"
)

// ReferenceWriter records this executable's reference-guard protocol after DB
// and Kubernetes initialization. Failure leaves registry deletion uncertified;
// it does not prevent ordinary service startup or claim compatible old peers.
func ReferenceWriter(role string) rainbond.Component {
	return rainbond.FuncComponent(func(ctx context.Context) error {
		manager, cluster, configuration := db.GetManager(), k8s.Default(), configs.Default()
		hostname, err := os.Hostname()
		if err != nil || manager == nil || cluster == nil || configuration.PublicConfig == nil {
			logrus.Warn("Registry cleanup writer evidence unavailable during startup")
			return nil
		}
		if err := kubeidentity.RegisterReferenceWriterStartup(ctx, manager.DB(), cluster.Clientset, configuration.PublicConfig.RbdNamespace, hostname, role); err != nil {
			logrus.Warn("Registry cleanup writer evidence could not be verified during startup")
		}
		return nil
	})
}
