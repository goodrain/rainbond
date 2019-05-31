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
	"errors"
	"sync"
	"time"
)

// ErrReadLogsNotSupported is returned when the logger does not support reading logs.
var ErrReadLogsNotSupported = errors.New("configured logging driver does not support reading")

const (
	logWatcherBufferSize = 4096
)

var messagePool = &sync.Pool{New: func() interface{} { return &Message{Line: make([]byte, 0, 256)} }}

// NewMessage returns a new message from the message sync.Pool
func NewMessage() *Message {
	return messagePool.Get().(*Message)
}

// PutMessage puts the specified message back n the message pool.
// The message fields are reset before putting into the pool.
func PutMessage(msg *Message) {
	msg.reset()
	messagePool.Put(msg)
}

// Message is datastructure that represents piece of output produced by some
// container.  The Line member is a slice of an array whose contents can be
// changed after a log driver's Log() method returns.
// Any changes made to this struct must also be updated in the `reset` function
type Message struct {
	Line      []byte
	Source    string
	Timestamp time.Time
	Attrs     LogAttributes
	Partial   bool
}

// reset sets the message back to default values
// This is used when putting a message back into the message pool.
// Any changes to the `Message` struct should be reflected here.
func (m *Message) reset() {
	m.Line = m.Line[:0]
	m.Source = ""
	m.Attrs = nil
	m.Partial = false
}

// LogAttributes is used to hold the extra attributes available in the log message
// Primarily used for converting the map type to string and sorting.
type LogAttributes map[string]string

// LogWatcher is used when consuming logs read from the LogReader interface.
type LogWatcher struct {
	// For sending log messages to a reader.
	Msg chan *Message
	// For sending error messages that occur while while reading logs.
	Err           chan error
	closeOnce     sync.Once
	closeNotifier chan struct{}
	consumerGone  chan struct{}
	consumerOnce  sync.Once
	producerOnce  sync.Once
	producerGone  chan struct{}
}

// NewLogWatcher returns a new LogWatcher.
func NewLogWatcher() *LogWatcher {
	return &LogWatcher{
		Msg:           make(chan *Message, logWatcherBufferSize),
		Err:           make(chan error, 1),
		closeNotifier: make(chan struct{}),
		producerGone:  make(chan struct{}),
		consumerGone:  make(chan struct{}),
	}
}

// Close notifies the underlying log reader to stop.
func (w *LogWatcher) Close() {
	// only close if not already closed
	w.closeOnce.Do(func() {
		close(w.closeNotifier)
	})
}

// WatchConsumerGone returns a channel receiver that receives notification
// when the log watcher consumer is gone.
func (w *LogWatcher) WatchConsumerGone() <-chan struct{} {
	return w.consumerGone
}

// ConsumerGone notifies that the logs consumer is gone.
func (w *LogWatcher) ConsumerGone() {
	// only close if not already closed
	w.consumerOnce.Do(func() {
		close(w.consumerGone)
	})
}

// WatchClose returns a channel receiver that receives notification
// when the watcher has been closed. This should only be called from
// one goroutine.
func (w *LogWatcher) WatchClose() <-chan struct{} {
	return w.closeNotifier
}

// ProducerGone notifies the underlying log reader that
// the logs producer (a container) is gone.
func (w *LogWatcher) ProducerGone() {
	// only close if not already closed
	w.producerOnce.Do(func() {
		close(w.producerGone)
	})
}

// WatchProducerGone returns a channel receiver that receives notification
// once the logs producer (a container) is gone.
func (w *LogWatcher) WatchProducerGone() <-chan struct{} {
	return w.producerGone
}

// Logger is the interface for docker logging drivers.
type Logger interface {
	Log(*Message) error
	Name() string
	Close() error
}
