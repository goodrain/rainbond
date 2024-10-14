// RAINBOND, Application Management Platform
// Copyright (C) 2021-2024 Goodrain Co., Ltd.

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

package eventlog

import (
	"context"
	"github.com/goodrain/rainbond/api/eventlog/entry"
	"github.com/goodrain/rainbond/api/eventlog/exit/web"
	"github.com/goodrain/rainbond/api/eventlog/store"
	"github.com/goodrain/rainbond/config/configs"
	"github.com/goodrain/rainbond/config/configs/rbdcomponent"
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

var defaultEventlogComponent *EventlogComponent

// EventlogComponent -
type EventlogComponent struct {
	Entry          *entry.Entry
	SocketServer   *web.SocketServer
	EventLogConfig *rbdcomponent.EventLogConfig
}

// New -
func New() *EventlogComponent {
	defaultEventlogComponent = &EventlogComponent{
		EventLogConfig: configs.Default().EventLogConfig,
	}
	return defaultEventlogComponent
}

// Start -
func (s *EventlogComponent) Start(ctx context.Context) error {
	go func() {
		err := func() error {
			logrus.Debug("Start run server.")

			storeManager, err := store.NewManager(s.EventLogConfig.Conf.EventStore, logrus.WithField("module", "MessageStore"))
			if err != nil {
				return err
			}
			healthInfo := storeManager.HealthCheck()
			if err := storeManager.Run(); err != nil {
				return err
			}
			defer storeManager.Stop()

			s.SocketServer = web.NewSocket(s.EventLogConfig.Conf.WebSocket, s.EventLogConfig.Conf.Cluster.Discover,
				logrus.WithField("module", "SocketServer"), storeManager, healthInfo)
			if err := s.SocketServer.Run(); err != nil {
				return err
			}
			defer s.SocketServer.Stop()

			s.Entry = entry.NewEntry(s.EventLogConfig.Conf.Entry, logrus.WithField("module", "EntryServer"), storeManager)
			if err := s.Entry.Start(); err != nil {
				return err
			}
			defer s.Entry.Stop()
			term := make(chan os.Signal)
			signal.Notify(term, os.Interrupt, syscall.SIGTERM)
			select {
			case <-term:
				logrus.Warn("Received SIGTERM, exiting gracefully...")
			case err := <-s.SocketServer.ListenError():
				logrus.Errorln("Error listen web socket server, exiting gracefully:", err)
			case err := <-storeManager.Error():
				logrus.Errorln("Store receive a error, exiting gracefully:", err)
			}
			logrus.Info("See you next time!")
			return nil
		}()
		if err != nil {
			panic(err)
		}
	}()
	eventlogTimeout := 2
	if os.Getenv("EVENTLOG_TIMEOUT") != "" {
		t, err := strconv.Atoi(os.Getenv("EVENTLOG_TIMEOUT"))
		if err == nil {
			eventlogTimeout = t
		}
	}
	startTime := time.Now()
	for {
		if defaultEventlogComponent.Entry != nil && defaultEventlogComponent.SocketServer != nil {
			logrus.Infof("eventlog server is running...")
			break
		}
		logrus.Info("waiting for eventlog server to start...")
		time.Sleep(5 * time.Second)
		if time.Since(startTime) > time.Duration(eventlogTimeout)*time.Minute {
			logrus.Error("eventlog server start timeout")
			break
		}
	}
	return nil
}

// CloseHandle -
func (r *EventlogComponent) CloseHandle() {

}

// Default -
func Default() *EventlogComponent {
	return defaultEventlogComponent
}
