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

package statsd

import (
	"fmt"
	"net"
	"strconv"

	"github.com/goodrain/rainbond/node/statsd/prometheus"
	"github.com/howeyc/fsnotify"
	"github.com/prometheus/common/version"
	"github.com/sirupsen/logrus"

	"github.com/goodrain/rainbond/cmd/node-proxy/option"
	"github.com/goodrain/rainbond/node/statsd/exporter"
)

//Exporter receive statsd metric and export prometheus metric
type Exporter struct {
	statsdListenAddress string
	statsdListenUDP     string
	statsdListenTCP     string
	mappingConfig       string
	readBuffer          int
	exporter            *exporter.Exporter
	register            *prometheus.Registry
	mapper              *exporter.MetricMapper
}

//CreateExporter create a exporter
func CreateExporter(sc option.StatsdConfig, register *prometheus.Registry) *Exporter {
	exp := &Exporter{
		statsdListenAddress: sc.StatsdListenAddress,
		statsdListenTCP:     sc.StatsdListenTCP,
		statsdListenUDP:     sc.StatsdListenUDP,
		readBuffer:          sc.ReadBuffer,
		mappingConfig:       sc.MappingConfig,
		register:            register,
	}
	exporter.MetryInit(register)
	return exp
}

//Start Start
func (e *Exporter) Start() error {
	if e.statsdListenAddress != "" {
		logrus.Warnln("Warning: statsd.listen-address is DEPRECATED, please use statsd.listen-udp instead.")
		e.statsdListenUDP = e.statsdListenAddress
	}

	if e.statsdListenUDP == "" && e.statsdListenTCP == "" {
		logrus.Fatalln("At least one of UDP/TCP listeners must be specified.")
		return fmt.Errorf("At least one of UDP/TCP listeners must be specified")
	}

	logrus.Infoln("Starting StatsD -> Prometheus Exporter", version.Info())
	logrus.Infoln("Build context", version.BuildContext())
	logrus.Infof("Accepting StatsD Traffic: UDP %v, TCP %v", e.statsdListenUDP, e.statsdListenTCP)

	events := make(chan exporter.Events, 1024)

	if e.statsdListenUDP != "" {
		udpListenAddr := udpAddrFromString(e.statsdListenUDP)
		uconn, err := net.ListenUDP("udp", udpListenAddr)
		if err != nil {
			return err
		}
		if e.readBuffer != 0 {
			err = uconn.SetReadBuffer(e.readBuffer)
			if err != nil {
				return err
			}
		}
		ul := &exporter.StatsDUDPListener{Conn: uconn}
		go ul.Listen(events)
	}

	if e.statsdListenTCP != "" {
		tcpListenAddr := tcpAddrFromString(e.statsdListenTCP)
		tconn, err := net.ListenTCP("tcp", tcpListenAddr)
		if err != nil {
			return err
		}
		tl := &exporter.StatsDTCPListener{Conn: tconn}
		go tl.Listen(events)
	}

	mapper, err := exporter.InitMapping()
	if err != nil {
		return err
	}
	exporter := exporter.NewExporter(mapper, e.register)
	e.exporter = exporter
	e.mapper = mapper

	go exporter.Listen(events)
	go exporter.GCollector()

	return nil
}

// Describe implements the prometheus.Collector interface.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {

}

// Collect implements the prometheus.Collector interface.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
}

//GetRegister GetRegister
func (e *Exporter) GetRegister() *prometheus.Registry {
	return e.register
}

func ipPortFromString(addr string) (*net.IPAddr, int) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		logrus.Fatal("Bad StatsD listening address", addr)
	}

	if host == "" {
		host = "0.0.0.0"
	}
	ip, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		logrus.Fatalf("Unable to resolve %s: %s", host, err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port < 0 || port > 65535 {
		logrus.Fatalf("Bad port %s: %s", portStr, err)
	}

	return ip, port
}

func udpAddrFromString(addr string) *net.UDPAddr {
	ip, port := ipPortFromString(addr)
	return &net.UDPAddr{
		IP:   ip.IP,
		Port: port,
		Zone: ip.Zone,
	}
}

func tcpAddrFromString(addr string) *net.TCPAddr {
	ip, port := ipPortFromString(addr)
	return &net.TCPAddr{
		IP:   ip.IP,
		Port: port,
		Zone: ip.Zone,
	}
}

func watchConfig(fileName string, mapper *exporter.MetricMapper) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logrus.Fatal(err)
	}

	err = watcher.WatchFlags(fileName, fsnotify.FSN_MODIFY)
	if err != nil {
		logrus.Fatal(err)
	}

	for {
		select {
		case ev := <-watcher.Event:
			logrus.Infof("Config file changed (%s), attempting reload", ev)
			err = mapper.InitFromFile(fileName)
			if err != nil {
				logrus.Errorln("Error reloading config:", err)
				exporter.ConfigLoads.WithLabelValues("failure").Inc()
			} else {
				logrus.Infoln("Config reloaded successfully")
				exporter.ConfigLoads.WithLabelValues("success").Inc()
			}
			// Re-add the file watcher since it can get lost on some changes. E.g.
			// saving a file with vim results in a RENAME-MODIFY-DELETE event
			// sequence, after which the newly written file is no longer watched.
			err = watcher.WatchFlags(fileName, fsnotify.FSN_MODIFY)
		case err := <-watcher.Error:
			logrus.Errorln("Error watching config:", err)
		}
	}
}

//ReloadConfig reload mapper config file
func (e *Exporter) ReloadConfig() (err error) {
	logrus.Infof("Config file changed, attempting reload")
	err = e.mapper.InitFromFile(e.mappingConfig)
	if err != nil {
		logrus.Errorln("Error reloading config:", err)
		exporter.ConfigLoads.WithLabelValues("failure").Inc()
	} else {
		logrus.Infoln("Config reloaded successfully")
		exporter.ConfigLoads.WithLabelValues("success").Inc()
	}
	return
}
