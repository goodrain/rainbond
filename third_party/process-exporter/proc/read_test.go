package proc

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

type (
	// procIDInfos implements procs using a slice of already
	// populated ProcIdInfo.  Used for testing.
	procIDInfos []IDInfo
)

func (p procIDInfos) get(i int) Proc {
	return &p[i]
}

func (p procIDInfos) length() int {
	return len(p)
}

func procInfoIter(ps ...IDInfo) *procIterator {
	return &procIterator{procs: procIDInfos(ps), idx: -1}
}

func allprocs(procpath string) Iter {
	fs, err := NewFS(procpath, false)
	if err != nil {
		cwd, _ := os.Getwd()
		panic("can't read " + procpath + ", cwd=" + cwd + ", err=" + fmt.Sprintf("%v", err))
	}
	return fs.AllProcs()
}

func TestReadFixture(t *testing.T) {
	procs := allprocs("../fixtures")
	var pii IDInfo

	count := 0
	for procs.Next() {
		count++
		var err error
		pii, err = procinfo(procs)
		noerr(t, err)
	}
	err := procs.Close()
	noerr(t, err)
	if count != 1 {
		t.Fatalf("got %d procs, want 1", count)
	}

	wantprocid := ID{Pid: 14804, StartTimeRel: 0x4f27b}
	if diff := cmp.Diff(pii.ID, wantprocid); diff != "" {
		t.Errorf("procid differs: (-got +want)\n%s", diff)
	}

	stime, _ := time.Parse(time.RFC3339Nano, "2017-10-19T22:52:51.19Z")
	wantstatic := Static{
		Name:         "process-exporte",
		Cmdline:      []string{"./process-exporter", "-procnames", "bash"},
		ParentPid:    10884,
		StartTime:    stime,
		EffectiveUID: 1000,
	}
	if diff := cmp.Diff(pii.Static, wantstatic); diff != "" {
		t.Errorf("static differs: (-got +want)\n%s", diff)
	}

	wantmetrics := Metrics{
		Counts: Counts{
			CPUUserTime:           0.1,
			CPUSystemTime:         0.04,
			ReadBytes:             1814455,
			WriteBytes:            0,
			MajorPageFaults:       0x2ff,
			MinorPageFaults:       0x643,
			CtxSwitchVoluntary:    72,
			CtxSwitchNonvoluntary: 6,
		},
		Memory: Memory{
			ResidentBytes: 0x7b1000,
			VirtualBytes:  0x1061000,
			VmSwapBytes:   0x2800,
		},
		Filedesc: Filedesc{
			Open:  5,
			Limit: 0x400,
		},
		NumThreads: 7,
		States:     States{Sleeping: 1},
	}
	if diff := cmp.Diff(pii.Metrics, wantmetrics); diff != "" {
		t.Errorf("metrics differs: (-got +want)\n%s", diff)
	}
}

func noerr(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("error: %v", err)
	}
}

// Basic test of proc reading: does AllProcs return at least two procs, one of which is us.
func TestAllProcs(t *testing.T) {
	procs := allprocs("/proc")
	count := 0
	for procs.Next() {
		count++
		if procs.GetPid() != os.Getpid() {
			continue
		}
		procid, err := procs.GetProcID()
		noerr(t, err)
		if procid.Pid != os.Getpid() {
			t.Errorf("got %d, want %d", procid.Pid, os.Getpid())
		}
		static, err := procs.GetStatic()
		noerr(t, err)
		if static.ParentPid != os.Getppid() {
			t.Errorf("got %d, want %d", static.ParentPid, os.Getppid())
		}
		metrics, _, err := procs.GetMetrics()
		noerr(t, err)
		if metrics.ResidentBytes == 0 {
			t.Errorf("got 0 bytes resident, want nonzero")
		}
		// All Go programs have multiple threads.
		if metrics.NumThreads < 2 {
			t.Errorf("got %d threads, want >1", metrics.NumThreads)
		}
		var zstates States
		if metrics.States == zstates {
			t.Errorf("got empty states")
		}
		threads, err := procs.GetThreads()
		if len(threads) < 2 {
			t.Errorf("got %d thread details, want >1", len(threads))
		}
	}
	err := procs.Close()
	noerr(t, err)
	if count == 0 {
		t.Errorf("got %d, want 0", count)
	}
}

// Test that we can observe the absence of a child process before it spawns and after it exits,
// and its presence during its lifetime.
func TestAllProcsSpawn(t *testing.T) {
	childprocs := func() []IDInfo {
		found := []IDInfo{}
		procs := allprocs("/proc")
		mypid := os.Getpid()
		for procs.Next() {
			procid, err := procs.GetProcID()
			if err != nil {
				continue
			}
			static, err := procs.GetStatic()
			if err != nil {
				continue
			}
			if static.ParentPid == mypid {
				found = append(found, IDInfo{procid, static, Metrics{}, nil})
			}
		}
		err := procs.Close()
		if err != nil {
			t.Fatalf("error closing procs iterator: %v", err)
		}
		return found
	}

	foundcat := func(procs []IDInfo) bool {
		for _, proc := range procs {
			if proc.Name == "cat" {
				return true
			}
		}
		return false
	}

	if foundcat(childprocs()) {
		t.Errorf("found cat before spawning it")
	}

	cmd := exec.Command("/bin/cat")
	wc, err := cmd.StdinPipe()
	noerr(t, err)
	err = cmd.Start()
	noerr(t, err)

	if !foundcat(childprocs()) {
		t.Errorf("didn't find cat after spawning it")
	}

	err = wc.Close()
	noerr(t, err)
	err = cmd.Wait()
	noerr(t, err)

	if foundcat(childprocs()) {
		t.Errorf("found cat after exit")
	}
}

func TestIterator(t *testing.T) {
	p1 := newProc(1, "p1", Metrics{})
	p2 := newProc(2, "p2", Metrics{})
	want := []IDInfo{p1, p2}
	pis := procInfoIter(want...)
	got, err := consumeIter(pis)
	noerr(t, err)
	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("procs differs: (-got +want)\n%s", diff)
	}
}
