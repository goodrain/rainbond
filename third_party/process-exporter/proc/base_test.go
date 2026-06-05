package proc

import (
	"fmt"
	"time"

	common "github.com/ncabatoff/process-exporter"
)

type msi map[string]int

// procinfo reads the ProcIdInfo for a proc and returns it or a zero value plus
// an error.
func procinfo(p Proc) (IDInfo, error) {
	id, err := p.GetProcID()
	if err != nil {
		return IDInfo{}, err
	}
	static, err := p.GetStatic()
	if err != nil {
		return IDInfo{}, err
	}
	metrics, _, err := p.GetMetrics()
	if err != nil {
		return IDInfo{}, err
	}
	return IDInfo{id, static, metrics, nil}, nil
}

// read everything in the iterator
func consumeIter(pi Iter) ([]IDInfo, error) {
	infos := []IDInfo{}
	for pi.Next() {
		info, err := procinfo(pi)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

type namer map[string]struct{}

func newNamer(names ...string) namer {
	nr := make(namer, len(names))
	for _, name := range names {
		nr[name] = struct{}{}
	}
	return nr
}

func (n namer) String() string {
	var ss = make([]string, 0, len(n))
	for s := range n {
		ss = append(ss, s)
	}
	return fmt.Sprintf("%v", ss)
}

func (n namer) MatchAndName(nacl common.ProcAttributes) (bool, string) {
	if _, ok := n[nacl.Name]; ok {
		return true, nacl.Name
	}
	return false, ""
}

func newProcIDStatic(pid, ppid int, startTime uint64, name string, cmdline []string) (ID, Static) {
	return ID{pid, startTime},
		Static{name, cmdline, ppid, time.Unix(int64(startTime), 0).UTC(), 1000}
}

func newProc(pid int, name string, m Metrics) IDInfo {
	id, static := newProcIDStatic(pid, 0, 0, name, nil)
	return IDInfo{id, static, m, nil}
}

func newProcStart(pid int, name string, startTime uint64) IDInfo {
	id, static := newProcIDStatic(pid, 0, startTime, name, nil)
	return IDInfo{id, static, Metrics{}, nil}
}

func newProcParent(pid int, name string, ppid int) IDInfo {
	id, static := newProcIDStatic(pid, ppid, 0, name, nil)
	return IDInfo{id, static, Metrics{}, nil}
}

func piinfot(pid int, name string, c Counts, m Memory, f Filedesc, threads []Thread) IDInfo {
	pii := piinfo(pid, name, c, m, f, len(threads))
	pii.Threads = threads
	return pii
}

func piinfo(pid int, name string, c Counts, m Memory, f Filedesc, t int) IDInfo {
	return piinfost(pid, name, c, m, f, t, States{})
}

func piinfost(pid int, name string, c Counts, m Memory, f Filedesc, t int, s States) IDInfo {
	id, static := newProcIDStatic(pid, 0, 0, name, nil)
	return IDInfo{
		ID:      id,
		Static:  static,
		Metrics: Metrics{c, m, f, uint64(t), s, ""},
	}
}
