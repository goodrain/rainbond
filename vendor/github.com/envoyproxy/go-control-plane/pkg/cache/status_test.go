// Copyright 2018 Envoyproxy Authors
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package cache

import (
	"reflect"
	"testing"

	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
)

func TestIDHash(t *testing.T) {
	node := &core.Node{Id: "test"}
	if got := (IDHash{}).ID(node); got != "test" {
		t.Errorf("IDHash.ID(%v) => got %s, want %s", node, got, node.Id)
	}
	if got := (IDHash{}).ID(nil); got != "" {
		t.Errorf("IDHash.ID(nil) => got %s, want empty", got)
	}
}

func TestNewStatusInfo(t *testing.T) {
	node := &core.Node{Id: "test"}
	info := newStatusInfo(node)

	if got := info.GetNode(); !reflect.DeepEqual(got, node) {
		t.Errorf("GetNode() => got %#v, want %#v", got, node)
	}

	if got := info.GetNumWatches(); got != 0 {
		t.Errorf("GetNumWatches() => got %d, want 0", got)
	}

	if got := info.GetLastWatchRequestTime(); !got.IsZero() {
		t.Errorf("GetLastWatchRequestTime() => got %v, want zero time", got)
	}

}
