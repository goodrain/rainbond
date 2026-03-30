package watch

import (
	"context"
	"net/http"
	"testing"

	"github.com/coreos/etcd/clientv3"
	v3rpc "github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	"github.com/coreos/etcd/mvcc/mvccpb"
)

// capability_id: rainbond.watch.resource-version-parse
func TestParseWatchResourceVersion(t *testing.T) {
	if got, err := ParseWatchResourceVersion(""); err != nil || got != 0 {
		t.Fatalf("expected empty resource version to map to 0, got %d err=%v", got, err)
	}
	if got, err := ParseWatchResourceVersion("0"); err != nil || got != 0 {
		t.Fatalf("expected zero resource version to map to 0, got %d err=%v", got, err)
	}
	if got, err := ParseWatchResourceVersion("123"); err != nil || got != 123 {
		t.Fatalf("expected 123, got %d err=%v", got, err)
	}
	if _, err := ParseWatchResourceVersion("bad"); err == nil {
		t.Fatal("expected parse error for invalid resource version")
	}
}

// capability_id: rainbond.watch.event-accessors
func TestEventAccessors(t *testing.T) {
	e := Event{
		Type: Modified,
		Source: &event{
			key:       "/demo/key",
			value:     []byte("new"),
			prevValue: []byte("old"),
		},
	}

	if e.GetKey() != "/demo/key" {
		t.Fatalf("unexpected key: %q", e.GetKey())
	}
	if e.GetValueString() != "new" || e.GetPreValueString() != "old" {
		t.Fatalf("unexpected values: new=%q old=%q", e.GetValueString(), e.GetPreValueString())
	}
}

// capability_id: rainbond.watch.event-byte-accessors
func TestEventByteAccessors(t *testing.T) {
	e := Event{
		Source: &event{
			value:     []byte("current"),
			prevValue: []byte("previous"),
		},
	}
	if string(e.GetValue()) != "current" || string(e.GetPreValue()) != "previous" {
		t.Fatalf("unexpected byte values: current=%q previous=%q", string(e.GetValue()), string(e.GetPreValue()))
	}
}

// capability_id: rainbond.watch.status-error-format
func TestStatusError(t *testing.T) {
	s := Status{Code: 500, Status: "Failure", Message: "boom", Reason: "Test"}
	if got := s.Error(); got != "(500)Status:Failure Message:boom Reason:Test" {
		t.Fatalf("unexpected status error string: %q", got)
	}
}

// capability_id: rainbond.watch.synthetic-create-event
func TestParseKVMarksCreateEvent(t *testing.T) {
	kv := &mvccpb.KeyValue{
		Key:         []byte("/demo/key"),
		Value:       []byte("value"),
		ModRevision: 9,
	}
	e := parseKV(kv)
	if e == nil || !e.isCreated || e.isDeleted || e.rev != 9 {
		t.Fatalf("unexpected parsed kv event: %+v", e)
	}
}

// capability_id: rainbond.watch.etcd-event-parse
func TestParseEvent(t *testing.T) {
	putEvent := &clientv3.Event{
		Type: clientv3.EventTypePut,
		Kv: &mvccpb.KeyValue{
			Key:            []byte("/demo/key"),
			Value:          []byte("new"),
			ModRevision:    10,
			CreateRevision: 10,
		},
		PrevKv: &mvccpb.KeyValue{Value: []byte("old")},
	}
	parsed := parseEvent(putEvent)
	if parsed == nil || parsed.key != "/demo/key" || parsed.rev != 10 || !parsed.isCreated || parsed.isDeleted {
		t.Fatalf("unexpected parsed put event: %+v", parsed)
	}
	if string(parsed.prevValue) != "old" {
		t.Fatalf("unexpected previous value: %q", string(parsed.prevValue))
	}

	deleteEvent := &clientv3.Event{
		Type: clientv3.EventTypeDelete,
		Kv: &mvccpb.KeyValue{
			Key:         []byte("/demo/key"),
			ModRevision: 11,
		},
	}
	parsed = parseEvent(deleteEvent)
	if parsed == nil || !parsed.isDeleted || parsed.isCreated {
		t.Fatalf("unexpected parsed delete event: %+v", parsed)
	}
}

// capability_id: rainbond.watch.event-type-transform
func TestWatchChanTransform(t *testing.T) {
	wc := &watchChan{}
	if wc.transform(nil) != nil {
		t.Fatal("expected nil event to stay nil")
	}

	added := wc.transform(&event{isCreated: true})
	if added == nil || added.Type != Added {
		t.Fatalf("expected added event, got %+v", added)
	}
	deleted := wc.transform(&event{isDeleted: true})
	if deleted == nil || deleted.Type != Deleted {
		t.Fatalf("expected deleted event, got %+v", deleted)
	}
	modified := wc.transform(&event{})
	if modified == nil || modified.Type != Modified {
		t.Fatalf("expected modified event, got %+v", modified)
	}
}

// capability_id: rainbond.watch.error-parse
func TestParseError(t *testing.T) {
	compacted := parseError(v3rpc.ErrCompacted)
	if compacted == nil || compacted.Type != Error || compacted.Error.Code != http.StatusGone || compacted.Error.Reason != "Expired" {
		t.Fatalf("unexpected compacted error event: %+v", compacted)
	}

	internal := parseError(v3rpc.ErrFutureRev)
	if internal == nil || internal.Type != Error || internal.Error.Code != http.StatusInternalServerError || internal.Error.Reason != "InternalError" {
		t.Fatalf("unexpected internal error event: %+v", internal)
	}
}

// capability_id: rainbond.watch.error-dispatch
func TestWatchChanSendError(t *testing.T) {
	wc := &watchChan{
		errChan: make(chan error, 1),
		ctx:     context.TODO(),
	}
	err := v3rpc.ErrCompacted
	wc.sendError(err)
	if got := <-wc.errChan; got != err {
		t.Fatalf("expected compacted error, got %v", got)
	}
}

// capability_id: rainbond.watch.event-dispatch
func TestWatchChanSendEvent(t *testing.T) {
	wc := &watchChan{
		incomingEventChan: make(chan *event, 1),
		ctx:               context.TODO(),
	}
	e := &event{key: "/demo/key"}
	wc.sendEvent(e)
	if got := <-wc.incomingEventChan; got != e {
		t.Fatalf("expected same event pointer, got %+v", got)
	}
}
