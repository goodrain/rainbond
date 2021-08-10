package thirdcomponent

import (
	"context"

	"github.com/goodrain/rainbond/pkg/apis/rainbond/v1alpha1"
	dis "github.com/goodrain/rainbond/worker/master/controller/thirdcomponent/discover"
	"github.com/goodrain/rainbond/worker/master/controller/thirdcomponent/prober"
	"github.com/sirupsen/logrus"
)

// Worker -
type Worker struct {
	discover   dis.Discover
	cancel     context.CancelFunc
	ctx        context.Context
	updateChan chan *v1alpha1.ThirdComponent
	stoped     bool

	proberManager prober.Manager
}

// Start -
func (w *Worker) Start() {
	defer func() {
		logrus.Infof("discover endpoint list worker %s/%s stoed", w.discover.GetComponent().Namespace, w.discover.GetComponent().Name)
		w.stoped = true
		w.proberManager.Stop()
	}()
	w.stoped = false
	logrus.Infof("discover endpoint list worker %s/%s  started", w.discover.GetComponent().Namespace, w.discover.GetComponent().Name)
	for {
		w.discover.Discover(w.ctx, w.updateChan)
		select {
		case <-w.ctx.Done():
			return
		default:
		}
	}
}

// UpdateDiscover -
func (w *Worker) UpdateDiscover(discover dis.Discover) {
	discover.SetProberManager(w.proberManager)
	w.discover = discover
	w.proberManager.AddThirdComponent(discover.GetComponent())
}

// Stop -
func (w *Worker) Stop() {
	w.cancel()
	w.proberManager.Stop()
}

// IsStop -
func (w *Worker) IsStop() bool {
	return w.stoped
}
