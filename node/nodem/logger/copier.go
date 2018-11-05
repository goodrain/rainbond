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
	"bytes"
	"io"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
)

const (
	bufSize  = 16 * 1024
	readSize = 2 * 1024
)

// Copier can copy logs from specified sources to Logger and attach Timestamp.
// Writes are concurrent, so you need implement some sync in your logger.
type Copier struct {
	// srcs is map of name -> reader pairs, for example "stdout", "stderr"
	srcs      map[string]io.Reader
	dst       Logger
	copyJobs  sync.WaitGroup
	closeOnce sync.Once
	closed    chan struct{}
}

// NewCopier creates a new Copier
func NewCopier(srcs map[string]io.Reader, dst Logger) *Copier {
	return &Copier{
		srcs:   srcs,
		dst:    dst,
		closed: make(chan struct{}),
	}
}

// Run starts logs copying
func (c *Copier) Run() {
	for src, w := range c.srcs {
		c.copyJobs.Add(1)
		go c.copySrc(src, w)
	}
}

func (c *Copier) copySrc(name string, src io.Reader) {
	defer c.copyJobs.Done()
	buf := make([]byte, bufSize)
	n := 0
	eof := false

	for {
		select {
		case <-c.closed:
			return
		default:
			// Work out how much more data we are okay with reading this time.
			upto := n + readSize
			if upto > cap(buf) {
				upto = cap(buf)
			}
			// Try to read that data.
			if upto > n {
				read, err := src.Read(buf[n:upto])
				if err != nil {
					if err != io.EOF {
						logrus.Errorf("Error scanning log stream: %s", err)
						return
					}
					eof = true
				}
				n += read
			}
			// If we have no data to log, and there's no more coming, we're done.
			if n == 0 && eof {
				return
			}
			// Break up the data that we've buffered up into lines, and log each in turn.
			p := 0
			for q := bytes.IndexByte(buf[p:n], '\n'); q >= 0; q = bytes.IndexByte(buf[p:n], '\n') {
				select {
				case <-c.closed:
					return
				default:
					msg := NewMessage()
					msg.Source = name
					msg.Timestamp = time.Now().UTC()
					msg.Line = append(msg.Line, buf[p:p+q]...)

					if logErr := c.dst.Log(msg); logErr != nil {
						logrus.Errorf("Failed to log msg %q for logger %s: %s", msg.Line, c.dst.Name(), logErr)
					}
				}
				p += q + 1
			}
			// If there's no more coming, or the buffer is full but
			// has no newlines, log whatever we haven't logged yet,
			// noting that it's a partial log line.
			if eof || (p == 0 && n == len(buf)) {
				if p < n {
					msg := NewMessage()
					msg.Source = name
					msg.Timestamp = time.Now().UTC()
					msg.Line = append(msg.Line, buf[p:n]...)
					msg.Partial = true

					if logErr := c.dst.Log(msg); logErr != nil {
						logrus.Errorf("Failed to log msg %q for logger %s: %s", msg.Line, c.dst.Name(), logErr)
					}
					p = 0
					n = 0
				}
				if eof {
					return
				}
			}
			// Move any unlogged data to the front of the buffer in preparation for another read.
			if p > 0 {
				copy(buf[0:], buf[p:n])
				n -= p
			}
		}
	}
}

// Wait waits until all copying is done
func (c *Copier) Wait() {
	c.copyJobs.Wait()
}

// Close closes the copier
func (c *Copier) Close() {
	c.closeOnce.Do(func() {
		close(c.closed)
	})
}
