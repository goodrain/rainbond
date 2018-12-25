// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package server

import (
	"context"
	"fmt"
	"github.com/goodrain/rainbond/gateway/metric"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/goodrain/rainbond/cmd/gateway/option"
	"github.com/goodrain/rainbond/gateway/controller"
	"k8s.io/apiserver/pkg/server/healthz"
)

//Run start run
func Run(s *option.GWServer) error {
	errCh := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGoCollector())
	// TODO
	//reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{
	//	PidFn:        func() (int, error) { return os.Getpid(), nil },
	//	ReportErrors: true,
	//}))

	mc := metric.NewDummyCollector()
	var err error
	if s.Config.EnableMetrics {
		mc, err = metric.NewCollector(reg)
		if err != nil {
			logrus.Fatalf("Error creating prometheus collector:  %v", err)
		}
	}
	mc.Start()

	gwc, err := controller.NewGWController(ctx, &s.Config, mc)
	if err != nil {
		return err
	}
	if gwc == nil {
		return fmt.Errorf("Fail to new GWController")
	}
	if err := gwc.Start(errCh); err != nil {
		return err
	}
	defer gwc.Close()

	mux := http.NewServeMux()
	registerHealthz(gwc, mux)
	registerMetrics(reg, mux)
	go startHTTPServer(s.ListenPorts.Health, mux)

	logrus.Info("RBD app gateway start success!")

	term := make(chan os.Signal)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)
	select {
	case <-term:
		logrus.Warn("Received SIGTERM, exiting gracefully...")
	case err := <-errCh:
		logrus.Errorf("Received a error %s, exiting gracefully...", err.Error())
	}
	logrus.Info("See you next time!")

	return nil
}

func registerHealthz(gc *controller.GWController, mux *http.ServeMux) {
	// expose health check endpoint (/healthz)
	healthz.InstallHandler(mux,
		healthz.PingHealthz,
		gc,
	)
}

func registerMetrics(reg *prometheus.Registry, mux *http.ServeMux) {
	mux.Handle(
		"/metrics",
		promhttp.HandlerFor(reg, promhttp.HandlerOpts{}),
	)

}

func startHTTPServer(port int, mux *http.ServeMux) {
	server := &http.Server{
		Addr:              fmt.Sprintf(":%v", port),
		Handler:           mux,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      300 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	logrus.Fatal(server.ListenAndServe())
}
