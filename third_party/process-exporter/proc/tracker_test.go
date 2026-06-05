package proc

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// Verify that the tracker finds and tracks or ignores procs based on the
// namer, and that it can distinguish between two procs with the same pid
// but different start time.
func TestTrackerBasic(t *testing.T) {
	p1, p2, p3 := 1, 2, 3
	n1, n2, n3, n4 := "g1", "g2", "g3", "g4"
	t1, t2, t3 := time.Unix(1, 0).UTC(), time.Unix(2, 0).UTC(), time.Unix(3, 0).UTC()

	tests := []struct {
		procs []IDInfo
		want  []Update
	}{
		{
			[]IDInfo{newProcStart(p1, n1, 1), newProcStart(p3, n3, 1)},
			[]Update{{GroupName: n1, Start: t1, Wchans: msi{}}},
		},
		{
			// p3 (ignored) has exited and p2 has appeared
			[]IDInfo{newProcStart(p1, n1, 1), newProcStart(p2, n2, 2)},
			[]Update{{GroupName: n1, Start: t1, Wchans: msi{}}, {GroupName: n2, Start: t2, Wchans: msi{}}},
		},
		{
			// p1 has exited and a new proc with a new name has taken its pid
			[]IDInfo{newProcStart(p1, n4, 3), newProcStart(p2, n2, 2)},
			[]Update{{GroupName: n4, Start: t3, Wchans: msi{}}, {GroupName: n2, Start: t2, Wchans: msi{}}},
		},
	}
	// Note that n3 should not be tracked according to our namer.
	tr := NewTracker(newNamer(n1, n2, n4), false, false, false, false)

	opts := cmpopts.SortSlices(lessUpdateGroupName)
	for i, tc := range tests {
		_, got, err := tr.Update(procInfoIter(tc.procs...))
		noerr(t, err)
		if diff := cmp.Diff(got, tc.want, opts); diff != "" {
			t.Errorf("%d: update differs: (-got +want)\n%s", i, diff)
		}
	}
}

// TestTrackerChildren verifies that when the tracker is asked to track
// children, processes not selected by the namer are still tracked if
// they're children of ones that are.
func TestTrackerChildren(t *testing.T) {
	p1, p2, p3 := 1, 2, 3
	n1, n2, n3 := "g1", "g2", "g3"
	// In this test everything starts at time t1 for simplicity
	t1 := time.Unix(0, 0).UTC()

	tests := []struct {
		procs []IDInfo
		want  []Update
	}{
		{
			[]IDInfo{
				newProcParent(p1, n1, 0),
				newProcParent(p2, n2, p1),
			},
			[]Update{{GroupName: n2, Start: t1, Wchans: msi{}}},
		},
		{
			[]IDInfo{
				newProcParent(p1, n1, 0),
				newProcParent(p2, n2, p1),
				newProcParent(p3, n3, p2),
			},
			[]Update{{GroupName: n2, Start: t1, Wchans: msi{}}, {GroupName: n2, Start: t1, Wchans: msi{}}},
		},
	}
	// Only n2 and children of n2s should be tracked
	tr := NewTracker(newNamer(n2), true, false, false, false)

	for i, tc := range tests {
		_, got, err := tr.Update(procInfoIter(tc.procs...))
		noerr(t, err)
		if diff := cmp.Diff(got, tc.want); diff != "" {
			t.Errorf("%d: update differs: (-got +want)\n%s", i, diff)
		}
	}
}

// TestTrackerMetrics verifies that the updates returned by the tracker
// match the input we're giving it.
func TestTrackerMetrics(t *testing.T) {
	p, n, tm := 1, "g1", time.Unix(0, 0).UTC()

	tests := []struct {
		proc IDInfo
		want Update
	}{
		{
			piinfost(p, n, Counts{1, 2, 3, 4, 5, 6, 0, 0}, Memory{7, 8, 0, 0, 0},
				Filedesc{1, 10}, 9, States{Sleeping: 1}),
			Update{n, Delta{}, Memory{7, 8, 0, 0, 0}, Filedesc{1, 10}, tm,
				9, States{Sleeping: 1}, msi{}, nil},
		},
		{
			piinfost(p, n, Counts{2, 3, 4, 5, 6, 7, 0, 0}, Memory{1, 2, 0, 0, 0},
				Filedesc{2, 20}, 1, States{Running: 1}),
			Update{n, Delta{1, 1, 1, 1, 1, 1, 0, 0}, Memory{1, 2, 0, 0, 0},
				Filedesc{2, 20}, tm, 1, States{Running: 1}, msi{}, nil},
		},
	}
	tr := NewTracker(newNamer(n), false, false, false, false)

	for i, tc := range tests {
		_, got, err := tr.Update(procInfoIter(tc.proc))
		noerr(t, err)
		if diff := cmp.Diff(got, []Update{tc.want}); diff != "" {
			t.Errorf("%d: update differs: (-got +want)\n%s", i, diff)
		}
	}
}

func TestTrackerThreads(t *testing.T) {
	p, n, tm := 1, "g1", time.Unix(0, 0).UTC()

	tests := []struct {
		proc IDInfo
		want Update
	}{
		{
			piinfo(p, n, Counts{}, Memory{}, Filedesc{1, 1}, 1),
			Update{n, Delta{}, Memory{}, Filedesc{1, 1}, tm, 1, States{}, msi{}, nil},
		}, {
			piinfot(p, n, Counts{}, Memory{}, Filedesc{1, 1}, []Thread{
				{ThreadID(ID{p, 0}), "t1", Counts{1, 2, 3, 4, 5, 6, 0, 0}, "", States{}},
				{ThreadID(ID{p + 1, 0}), "t2", Counts{1, 1, 1, 1, 1, 1, 0, 0}, "", States{}},
			}),
			Update{n, Delta{}, Memory{}, Filedesc{1, 1}, tm, 2, States{}, msi{},
				[]ThreadUpdate{
					{"t1", Delta{}},
					{"t2", Delta{}},
				},
			},
		}, {
			piinfot(p, n, Counts{}, Memory{}, Filedesc{1, 1}, []Thread{
				{ThreadID(ID{p, 0}), "t1", Counts{2, 3, 4, 5, 6, 7, 0, 0}, "", States{}},
				{ThreadID(ID{p + 1, 0}), "t2", Counts{2, 2, 2, 2, 2, 2, 0, 0}, "", States{}},
				{ThreadID(ID{p + 2, 0}), "t2", Counts{1, 1, 1, 1, 1, 1, 0, 0}, "", States{}},
			}),
			Update{n, Delta{}, Memory{}, Filedesc{1, 1}, tm, 3, States{}, msi{},
				[]ThreadUpdate{
					{"t1", Delta{1, 1, 1, 1, 1, 1, 0, 0}},
					{"t2", Delta{1, 1, 1, 1, 1, 1, 0, 0}},
					{"t2", Delta{}},
				},
			},
		}, {
			piinfot(p, n, Counts{}, Memory{}, Filedesc{1, 1}, []Thread{
				{ThreadID(ID{p, 0}), "t1", Counts{2, 3, 4, 5, 6, 7, 0, 0}, "", States{}},
				{ThreadID(ID{p + 2, 0}), "t2", Counts{1, 2, 3, 4, 5, 6, 0, 0}, "", States{}},
			}),
			Update{n, Delta{}, Memory{}, Filedesc{1, 1}, tm, 2, States{}, msi{},
				[]ThreadUpdate{
					{"t1", Delta{}},
					{"t2", Delta{0, 1, 2, 3, 4, 5, 0, 0}},
				},
			},
		},
	}
	tr := NewTracker(newNamer(n), false, true, false, false)

	opts := cmpopts.SortSlices(lessThreadUpdate)
	for i, tc := range tests {
		_, got, err := tr.Update(procInfoIter(tc.proc))
		noerr(t, err)
		if diff := cmp.Diff(got, []Update{tc.want}, opts); diff != "" {
			t.Errorf("%d: update differs: (-got +want)\n%s, %v, %v", i, diff, got[0].Threads, tc.want.Threads)
		}
	}
}
