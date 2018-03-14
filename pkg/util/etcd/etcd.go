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
	"strings"
	"time"

	v3 "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
	"golang.org/x/net/context"
)

// RemoteKV is a key/revision pair created by the client and stored on etcd
type RemoteKV struct {
	kv  v3.KV
	key string
	rev int64
	val string
}

func newKey(ctx context.Context, kv v3.KV, key string, leaseID v3.LeaseID) (*RemoteKV, error) {
	return newKV(ctx, kv, key, "", leaseID)
}

func newKV(ctx context.Context, kv v3.KV, key, val string, leaseID v3.LeaseID) (*RemoteKV, error) {
	rev, err := putNewKV(ctx, kv, key, val, leaseID)
	if err != nil {
		return nil, err
	}
	return &RemoteKV{kv, key, rev, val}, nil
}

func newUniqueKV(ctx context.Context, kv v3.KV, prefix string, val string) (*RemoteKV, error) {
	for {
		newKey := fmt.Sprintf("%s/%v", prefix, time.Now().UnixNano())
		rev, err := putNewKV(ctx, kv, newKey, val, 0)
		if err == nil {
			return &RemoteKV{kv, newKey, rev, val}, nil
		}
		if err != ErrKeyExists {
			return nil, err
		}
	}
}

// putNewKV attempts to create the given key, only succeeding if the key did
// not yet exist.
func putNewKV(ctx context.Context, kv v3.KV, key, val string, leaseID v3.LeaseID) (int64, error) {
	cmp := v3.Compare(v3.Version(key), "=", 0)
	req := v3.OpPut(key, val, v3.WithLease(leaseID))
	txnresp, err := kv.Txn(ctx).If(cmp).Then(req).Commit()
	if err != nil {
		return 0, err
	}
	if !txnresp.Succeeded {
		return 0, ErrKeyExists
	}
	return txnresp.Header.Revision, nil
}

// newSequentialKV allocates a new sequential key <prefix>/nnnnn with a given
// prefix and value. Note: a bookkeeping node __<prefix> is also allocated.
func newSequentialKV(kv v3.KV, prefix, val string) (*RemoteKV, error) {
	resp, err := kv.Get(context.TODO(), prefix, v3.WithLastKey()...)
	if err != nil {
		return nil, err
	}

	// add 1 to last key, if any
	newSeqNum := 0
	if len(resp.Kvs) != 0 {
		fields := strings.Split(string(resp.Kvs[0].Key), "/")
		_, serr := fmt.Sscanf(fields[len(fields)-1], "%d", &newSeqNum)
		if serr != nil {
			return nil, serr
		}
		newSeqNum++
	}
	newKey := fmt.Sprintf("%s/%016d", prefix, newSeqNum)

	// base prefix key must be current (i.e., <=) with the server update;
	// the base key is important to avoid the following:
	// N1: LastKey() == 1, start txn.
	// N2: new Key 2, new Key 3, Delete Key 2
	// N1: txn succeeds allocating key 2 when it shouldn't
	baseKey := "__" + prefix

	// current revision might contain modification so +1
	cmp := v3.Compare(v3.ModRevision(baseKey), "<", resp.Header.Revision+1)
	reqPrefix := v3.OpPut(baseKey, "")
	reqnewKey := v3.OpPut(newKey, val)

	txn := kv.Txn(context.TODO())
	txnresp, err := txn.If(cmp).Then(reqPrefix, reqnewKey).Commit()
	if err != nil {
		return nil, err
	}
	if !txnresp.Succeeded {
		return newSequentialKV(kv, prefix, val)
	}
	return &RemoteKV{kv, newKey, txnresp.Header.Revision, val}, nil
}

func (rk *RemoteKV) Key() string     { return rk.key }
func (rk *RemoteKV) Revision() int64 { return rk.rev }
func (rk *RemoteKV) Value() string   { return rk.val }

func (rk *RemoteKV) Delete() error {
	if rk.kv == nil {
		return nil
	}
	_, err := rk.kv.Delete(context.TODO(), rk.key)
	rk.kv = nil
	return err
}

func (rk *RemoteKV) Put(val string) error {
	_, err := rk.kv.Put(context.TODO(), rk.key, val)
	return err
}

// EphemeralKV is a new key associated with a session lease
type EphemeralKV struct{ RemoteKV }

// newEphemeralKV creates a new key/value pair associated with a session lease
func newEphemeralKV(s *concurrency.Session, key, val string) (*EphemeralKV, error) {
	k, err := newKV(context.TODO(), s.Client(), key, val, s.Lease())
	if err != nil {
		return nil, err
	}
	return &EphemeralKV{*k}, nil
}

// newUniqueEphemeralKey creates a new unique valueless key associated with a session lease
func newUniqueEphemeralKey(s *concurrency.Session, prefix string) (*EphemeralKV, error) {
	return newUniqueEphemeralKV(s, prefix, "")
}

// newUniqueEphemeralKV creates a new unique key/value pair associated with a session lease
func newUniqueEphemeralKV(s *concurrency.Session, prefix, val string) (ek *EphemeralKV, err error) {
	for {
		newKey := fmt.Sprintf("%s/%v", prefix, time.Now().UnixNano())
		ek, err = newEphemeralKV(s, newKey, val)
		if err == nil || err != ErrKeyExists {
			break
		}
	}
	return ek, err
}
