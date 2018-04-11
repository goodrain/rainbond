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

package etcd

import (
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"golang.org/x/net/context"
)

// WaitEvents waits on a key until it observes the given events and returns the final one.
func WaitEvents(c *clientv3.Client, key string, rev int64, evs []mvccpb.Event_EventType) (*clientv3.Event, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wc := c.Watch(ctx, key, clientv3.WithRev(rev))
	if wc == nil {
		return nil, ErrNoWatcher
	}
	return waitEvents(wc, evs), nil
}

//WatchPrefixEvents watch prefix
func WatchPrefixEvents(c *clientv3.Client, prefix string, evs []mvccpb.Event_EventType) (*clientv3.Event, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wc := c.Watch(ctx, prefix, clientv3.WithPrefix())
	if wc == nil {
		return nil, ErrNoWatcher
	}
	return waitEvents(wc, evs), nil
}

//WaitPrefixEvents WaitPrefixEvents
func WaitPrefixEvents(c *clientv3.Client, prefix string, rev int64, evs []mvccpb.Event_EventType) (*clientv3.Event, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wc := c.Watch(ctx, prefix, clientv3.WithPrefix(), clientv3.WithRev(rev))
	if wc == nil {
		return nil, ErrNoWatcher
	}
	return waitEvents(wc, evs), nil
}

//waitEvents this will return nil
func waitEvents(wc clientv3.WatchChan, evs []mvccpb.Event_EventType) *clientv3.Event {
	i := 0
	for wresp := range wc {
		for _, ev := range wresp.Events {
			if ev.Type == evs[i] {
				i++
				if i == len(evs) {
					return ev
				}
			}
		}
	}
	return nil
}
