// Copyright 2016 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"encoding/json"
	"fmt"
	"time"

	etcd "github.com/coreos/etcd/clientv3"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/naming"
	"google.golang.org/grpc/status"

	"golang.org/x/net/context"
)

var ErrWatcherClosed = fmt.Errorf("naming: watch closed")

// GRPCResolver creates a grpc.Watcher for a target to track its resolution changes.
type GRPCResolver struct {
	// Client is an initialized etcd client.
	Client *etcd.Client
}

//Update Update
func (gr *GRPCResolver) Update(ctx context.Context, target string, nm naming.Update, opts ...etcd.OpOption) (err error) {
	switch nm.Op {
	case naming.Add:
		var v []byte
		if v, err = json.Marshal(nm); err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		_, err = gr.Client.KV.Put(ctx, target+"/"+nm.Addr, string(v), opts...)
	case naming.Delete:
		if gr.Client != nil {
			_, err = gr.Client.Delete(ctx, target+"/"+nm.Addr, opts...)
		}
	default:
		return status.Error(codes.InvalidArgument, "naming: bad naming op")
	}
	return err
}

//Resolve Resolve
func (gr *GRPCResolver) Resolve(target string) (naming.Watcher, error) {
	ctx, cancel := context.WithCancel(context.Background())
	w := &gRPCWatcher{c: gr.Client, target: target + "/", ctx: ctx, cancel: cancel}
	return w, nil
}

type gRPCWatcher struct {
	c           *etcd.Client
	target      string
	ctx         context.Context
	cancel      context.CancelFunc
	watchCancel context.CancelFunc
	wch         etcd.WatchChan
	err         error
}

// Next gets the next set of updates from the etcd resolver.
// Calls to Next should be serialized; concurrent calls are not safe since
// there is no way to reconcile the update ordering.
func (gw *gRPCWatcher) Next() ([]*naming.Update, error) {

	if gw.wch == nil {
		// first Next() returns all addresses
		ctx, cancel := context.WithCancel(gw.ctx)
		gw.watchCancel = cancel
		all, err := gw.firstNext(ctx)
		return all, err
	}
	if gw.err != nil {
		return nil, gw.err
	}

	// process new events on target/*
	timer := time.NewTimer(time.Second * 20)
	defer timer.Stop()
	select {
	case wr, ok := <-gw.wch:
		if !ok {
			gw.err = status.Error(codes.Unavailable, ErrWatcherClosed.Error())
			return nil, gw.err
		}
		if gw.err = wr.Err(); gw.err != nil {
			return nil, gw.err
		}
		updates := make([]*naming.Update, 0, len(wr.Events))
		for _, e := range wr.Events {
			var jupdate naming.Update
			var err error
			switch e.Type {
			case etcd.EventTypePut:
				err = json.Unmarshal(e.Kv.Value, &jupdate)
				jupdate.Op = naming.Add
			case etcd.EventTypeDelete:
				err = json.Unmarshal(e.PrevKv.Value, &jupdate)
				jupdate.Op = naming.Delete
			}
			if err == nil {
				updates = append(updates, &jupdate)
			}
		}
		return updates, nil
	case <-timer.C:
		gw.watchCancel()
		gw.wch = nil
		return gw.Next()
	}
}

func (gw *gRPCWatcher) firstNext(ctx context.Context) ([]*naming.Update, error) {
	// Use serialized request so resolution still works if the target etcd
	// server is partitioned away from the quorum.
	resp, err := gw.c.Get(ctx, gw.target, etcd.WithPrefix(), etcd.WithSerializable())
	if gw.err = err; err != nil {
		return nil, err
	}
	updates := make([]*naming.Update, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var jupdate naming.Update
		if err := json.Unmarshal(kv.Value, &jupdate); err != nil {
			continue
		}
		updates = append(updates, &jupdate)
	}
	opts := []etcd.OpOption{etcd.WithRev(resp.Header.Revision + 1), etcd.WithPrefix(), etcd.WithPrevKV()}
	gw.wch = gw.c.Watch(ctx, gw.target, opts...)
	return updates, nil
}

func (gw *gRPCWatcher) Close() { gw.cancel() }
