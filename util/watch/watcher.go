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

package watch

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/coreos/etcd/clientv3"
	etcdrpc "github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	"golang.org/x/net/context"
)

const (
	// We have set a buffer in order to reduce times of context switches.
	incomingBufSize = 100
	outgoingBufSize = 100
)

type watcher struct {
	client *clientv3.Client
}

// watchChan implements Interface.
type watchChan struct {
	watcher           *watcher
	key               string
	initialRev        int64
	recursive         bool
	ctx               context.Context
	cancel            context.CancelFunc
	incomingEventChan chan *event
	resultChan        chan Event
	errChan           chan error
}

func newWatcher(client *clientv3.Client) *watcher {
	return &watcher{
		client: client,
	}
}

// Watch watches on a key and returns a Interface that transfers relevant notifications.
// If rev is zero, it will return the existing object(s) and then start watching from
// the maximum revision+1 from returned objects.
// If rev is non-zero, it will watch events happened after given revision.
// If recursive is false, it watches on given key.
// If recursive is true, it watches any children and directories under the key, excluding the root key itself.
// pred must be non-nil. Only if pred matches the change, it will be returned.
func (w *watcher) Watch(ctx context.Context, key string, rev int64, recursive bool) (Interface, error) {
	if recursive && !strings.HasSuffix(key, "/") {
		key += "/"
	}
	wc := w.createWatchChan(ctx, key, rev, recursive)
	go wc.run()
	return wc, nil
}

func (w *watcher) createWatchChan(ctx context.Context, key string, rev int64, recursive bool) *watchChan {
	wc := &watchChan{
		watcher:           w,
		key:               key,
		initialRev:        rev,
		recursive:         recursive,
		incomingEventChan: make(chan *event, incomingBufSize),
		resultChan:        make(chan Event, outgoingBufSize),
		errChan:           make(chan error, 1),
	}
	wc.ctx, wc.cancel = context.WithCancel(ctx)
	return wc
}

func (wc *watchChan) run() {
	watchClosedCh := make(chan struct{})
	go wc.startWatching(watchClosedCh)

	var resultChanWG sync.WaitGroup
	resultChanWG.Add(1)
	go wc.processEvent(&resultChanWG)

	select {
	case err := <-wc.errChan:
		if err == context.Canceled {
			break
		}
		errResult := parseError(err)
		if errResult != nil {
			// error result is guaranteed to be received by user before closing ResultChan.
			select {
			case wc.resultChan <- *errResult:
			case <-wc.ctx.Done(): // user has given up all results
			}
		}
	case <-watchClosedCh:
	case <-wc.ctx.Done(): // user cancel
	}

	// We use wc.ctx to reap all goroutines. Under whatever condition, we should stop them all.
	// It's fine to double cancel.
	wc.cancel()

	// we need to wait until resultChan wouldn't be used anymore
	resultChanWG.Wait()
	close(wc.resultChan)
}

func (wc *watchChan) Stop() {
	wc.cancel()
}

func (wc *watchChan) ResultChan() <-chan Event {
	return wc.resultChan
}

// sync tries to retrieve existing data and send them to process.
// The revision to watch will be set to the revision in response.
// All events sent will have isCreated=true
func (wc *watchChan) sync() error {
	opts := []clientv3.OpOption{}
	if wc.recursive {
		opts = append(opts, clientv3.WithPrefix())
	}
	getResp, err := wc.watcher.client.Get(wc.ctx, wc.key, opts...)
	if err != nil {
		return err
	}
	wc.initialRev = getResp.Header.Revision
	for _, kv := range getResp.Kvs {
		wc.sendEvent(parseKV(kv))
	}
	return nil
}

// startWatching does:
// - get current objects if initialRev=0; set initialRev to current rev
// - watch on given key and send events to process.
func (wc *watchChan) startWatching(watchClosedCh chan struct{}) {
	if wc.initialRev == 0 {
		if err := wc.sync(); err != nil {
			logrus.Errorf("failed to sync with latest state: %v", err)
			wc.sendError(err)
			close(watchClosedCh)
			return
		}
	}
	opts := []clientv3.OpOption{
		clientv3.WithRev(wc.initialRev + 1),
		clientv3.WithPrevKV(),
	}
	if wc.recursive {
		opts = append(opts, clientv3.WithPrefix())
	}
	ctx, cancel := context.WithCancel(wc.ctx)
	defer cancel()
	wch := wc.watcher.client.Watch(ctx, wc.key, opts...)
	err := func() error {
		timer := time.NewTimer(time.Second * 20)
		defer timer.Stop()
		for {
			select {
			case wres := <-wch:
				if err := wres.Err(); err != nil {
					// If there is an error on server (e.g. compaction), the channel will return it before closed.
					logrus.Errorf("watch chan error: %v", err)
					wc.sendError(err)
					close(watchClosedCh)
					return err
				}
				logrus.Debugf("watch event %+v", wres)
				// If you return a structure with no events
				// It is considered that this watch is no longer effective
				// Return nil redo watch
				if len(wres.Events) == 0 {
					return nil
				}
				for _, e := range wres.Events {
					wc.sendEvent(parseEvent(e))
				}
				timer.Reset(time.Second * 20)
			case <-timer.C:
				return nil
			}
		}
	}()
	if err == nil {
		wc.initialRev = 0
		logrus.Debugf("watcher sync, because of not updated for a long time")
		go wc.startWatching(watchClosedCh)
	}
}

// processEvent processes events from etcd watcher and sends results to resultChan.
func (wc *watchChan) processEvent(wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case e := <-wc.incomingEventChan:
			res := wc.transform(e)
			if res == nil {
				continue
			}
			if len(wc.resultChan) == outgoingBufSize {
				logrus.Warningf("Fast watcher, slow processing. Number of buffered events: %d."+
					"Probably caused by slow dispatching events to watchers", outgoingBufSize)
			}
			// If user couldn't receive results fast enough, we also block incoming events from watcher.
			// Because storing events in local will cause more memory usage.
			// The worst case would be closing the fast watcher.
			select {
			case wc.resultChan <- *res:
			case <-wc.ctx.Done():
				return
			}
		case <-wc.ctx.Done():
			return
		}
	}
}

// transform transforms an event into a result for user if not filtered.
func (wc *watchChan) transform(e *event) (res *Event) {
	if e == nil {
		return nil
	}
	switch {
	case e.isDeleted:
		res = &Event{
			Type:   Deleted,
			Source: e,
		}
	case e.isCreated:
		res = &Event{
			Type:   Added,
			Source: e,
		}
	default:
		res = &Event{
			Type:   Modified,
			Source: e,
		}
	}
	return res
}

func parseError(err error) *Event {
	var status Status
	switch {
	case err == etcdrpc.ErrCompacted:
		status = Status{
			Status:  "Failure",
			Message: err.Error(),
			Code:    http.StatusGone,
			Reason:  "Expired",
		}
	default:
		status = Status{
			Status:  "Failure",
			Message: err.Error(),
			Code:    http.StatusInternalServerError,
			Reason:  "InternalError",
		}
	}

	return &Event{
		Type:  Error,
		Error: status,
	}
}

func (wc *watchChan) sendError(err error) {
	select {
	case wc.errChan <- err:
	case <-wc.ctx.Done():
	}
}

func (wc *watchChan) sendEvent(e *event) {
	if len(wc.incomingEventChan) == incomingBufSize {
		logrus.Warningf("Fast watcher, slow processing. Number of buffered events: %d."+
			"Probably caused by slow decoding, user not receiving fast, or other processing logic",
			incomingBufSize)
	}
	select {
	case wc.incomingEventChan <- e:
	case <-wc.ctx.Done():
	}
}
