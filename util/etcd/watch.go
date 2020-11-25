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
	"fmt"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

//ErrNoUpdateForLongTime no update for long time , can reobservation of synchronous data
var ErrNoUpdateForLongTime = fmt.Errorf("not updated for a long time")

//WaitPrefixEvents WaitPrefixEvents
func WaitPrefixEvents(c *clientv3.Client, prefix string, rev int64, evs []mvccpb.Event_EventType) (*clientv3.Event, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logrus.Debug("start watch message from etcd queue")
	wc := clientv3.NewWatcher(c).Watch(ctx, prefix, clientv3.WithPrefix(), clientv3.WithRev(rev))
	if wc == nil {
		return nil, ErrNoWatcher
	}
	event := waitEvents(wc, evs)
	if event != nil {
		return event, nil
	}
	logrus.Debug("queue watcher sync, because of not updated for a long time")
	return nil, ErrNoUpdateForLongTime
}

//waitEvents this will return nil
func waitEvents(wc clientv3.WatchChan, evs []mvccpb.Event_EventType) *clientv3.Event {
	i := 0
	timer := time.NewTimer(time.Second * 30)
	defer timer.Stop()
	for {
		select {
		case wresp := <-wc:
			if wresp.Err() != nil {
				logrus.Errorf("watch event failure %s", wresp.Err().Error())
				return nil
			}
			if len(wresp.Events) == 0 {
				return nil
			}
			for _, ev := range wresp.Events {
				if ev.Type == evs[i] {
					i++
					if i == len(evs) {
						return ev
					}
				}
			}
		case <-timer.C:
			return nil
		}
	}
}
