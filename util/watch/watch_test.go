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
	"context"
	"fmt"
	"testing"

	"github.com/coreos/etcd/clientv3"
)

func TestWatch(t *testing.T) {
	client, err := clientv3.New(clientv3.Config{Endpoints: []string{"http://127.0.0.1:2379"}})
	if err != nil {
		t.Fatal(err)
	}
	store := New(client, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w, err := store.WatchList(ctx, "/store", "")
	if err != nil {
		t.Fatal(err)
	}
	defer w.Stop()
	for event := range w.ResultChan() {
		fmt.Println(event.Source)
	}
}
