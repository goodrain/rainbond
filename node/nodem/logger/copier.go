// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

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

package logger

import (
	"context"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	bufSize  = 16 * 1024
	readSize = 2 * 1024
)

// Copier can copy logs from specified sources to Logger and attach Timestamp.
// Writes are concurrent, so you need implement some sync in your logger.
type Copier struct {
	logfile       *LogFile
	dst           []Logger
	closed        chan struct{}
	reader        *LogWatcher
	since         time.Time
	once          sync.Once
	containerID   string
	runtimeClient *runtimeapi.RuntimeServiceClient
}

// NewCopier creates a new Copier
func NewCopier(logfile *LogFile, dst []Logger, since time.Time, containerID string, runtimeClient *runtimeapi.RuntimeServiceClient) *Copier {
	return &Copier{
		logfile:       logfile,
		reader:        NewLogWatcher(),
		dst:           dst,
		since:         since,
		containerID:   containerID,
		runtimeClient: runtimeClient,
	}
}

// Run starts logs copying
func (c *Copier) Run() {
	c.closed = make(chan struct{})
	if c.runtimeClient != nil {
		// When using the CRI runtime, use this interface to get non-json logs
		go ReadLogs(context.Background(), c.logfile.logPath, c.containerID, &ReadConfig{Follow: true, Since: c.since, Tail: -1}, *c.runtimeClient, c.reader)
	} else {
		go c.logfile.ReadLogs(ReadConfig{Follow: true, Since: c.since, Tail: 0}, c.reader)
	}
	go c.copySrc()
}

func (c *Copier) copySrc() {
	defer c.reader.ConsumerGone()
lool:
	for {
		select {
		case <-c.closed:
			return
		case err := <-c.reader.Err:
			logrus.Errorf("read container log file error %s, will retry after 5 seconds", err.Error())
			//If there is an error in the collection log process,
			//the collection should be restarted and not stopped
			time.Sleep(time.Second * 5)
			go c.logfile.ReadLogs(ReadConfig{Follow: true, Since: c.since, Tail: 0}, c.reader)
			continue
		case msg, ok := <-c.reader.Msg:
			if !ok {
				break lool
			}
			for _, d := range c.dst {
				if err := d.Log(msg); err != nil {
					logrus.Debugf("copy container log failure %s", err.Error())
				}
			}
		}
	}
}

// Close closes the copier
func (c *Copier) Close() {
	c.once.Do(func() {
		if c.dst != nil {
			for _, d := range c.dst {
				if err := d.Close(); err != nil {
					logrus.Errorf("close log driver failure %s", err.Error())
				}
			}
		}
		close(c.closed)
	})
}
