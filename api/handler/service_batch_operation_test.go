package handler

import (
	"container/list"
	"strings"
	"testing"
)

func TestBuildLinkListByHead(t *testing.T) {
	tests := []struct {
		name        string
		l           *list.List
		sid2depsids map[string][]string
		want        []*list.List
	}{
		{
			name: "nil linked list",
			l:    nil,
			want: nil,
		},
		{
			name: "empty linked list",
			l: func() *list.List {
				return list.New()
			}(),
			want: nil,
		},
		{
			name: "no more children",
			l: func() *list.List {
				l := list.New()
				l.PushBack("apple")
				return l
			}(),
			want: func() []*list.List {
				l := list.New()
				l.PushBack("apple")
				return []*list.List{l}
			}(),
		},
		{
			name: "child node is already in the linked list",
			l: func() *list.List {
				l := list.New()
				l.PushBack("apple")
				l.PushBack("banana")
				l.PushBack("cat")
				l.PushBack("dog")
				return l
			}(),
			sid2depsids: map[string][]string{
				"dog": []string{"banana"},
			},
			want: func() []*list.List {
				l := list.New()
				l.PushBack("apple")
				l.PushBack("banana")
				l.PushBack("cat")
				l.PushBack("dog")
				return []*list.List{l}
			}(),
		},
		{
			name: "one child node is not already in the linked list, the other one is not",
			l: func() *list.List {
				l := list.New()
				l.PushBack("apple")
				l.PushBack("banana")
				l.PushBack("cat")
				l.PushBack("dog")
				return l
			}(),
			sid2depsids: map[string][]string{
				"dog": []string{"banana", "elephant"},
			},
			want: func() []*list.List {
				l := list.New()
				l.PushBack("apple")
				l.PushBack("banana")
				l.PushBack("cat")
				l.PushBack("dog")

				l2 := list.New()
				l2.PushBack("apple")
				l2.PushBack("banana")
				l2.PushBack("cat")
				l2.PushBack("dog")
				l2.PushBack("elephant")
				return []*list.List{l, l2}
			}(),
		},
		{
			name: "three sub lists",
			l: func() *list.List {
				l := list.New()
				l.PushBack("apple")
				return l
			}(),
			sid2depsids: map[string][]string{
				"apple":  []string{"banana"},
				"banana": []string{"cat", "cake", "candy"},
				"cat":    []string{"dog"},
				"cake":   []string{"dance"},
				"candy":  []string{"daughter"},
			},
			want: func() []*list.List {
				l1 := list.New()
				l1.PushBack("apple")
				l1.PushBack("banana")
				l1.PushBack("cat")
				l1.PushBack("dog")

				l2 := list.New()
				l2.PushBack("apple")
				l2.PushBack("banana")
				l2.PushBack("cake")
				l2.PushBack("dance")

				l3 := list.New()
				l3.PushBack("apple")
				l3.PushBack("banana")
				l3.PushBack("candy")
				l3.PushBack("daughter")
				return []*list.List{l1, l2, l3}
			}(),
		},
		{
			name: "single linked list",
			l: func() *list.List {
				l := list.New()
				l.PushBack("apple")
				return l
			}(),
			sid2depsids: map[string][]string{
				"apple":  []string{"banana"},
				"banana": []string{"cat"},
				"cat":    []string{"dog"},
			},
			want: func() []*list.List {
				l := list.New()
				l.PushBack("apple")
				l.PushBack("banana")
				l.PushBack("cat")
				l.PushBack("dog")
				return []*list.List{l}
			}(),
		},
		{
			name: "ring linked list",
			l: func() *list.List {
				l := list.New()
				l.PushBack("apple")
				return l
			}(),
			sid2depsids: map[string][]string{
				"apple":  []string{"banana"},
				"banana": []string{"cat"},
				"cat":    []string{"dog"},
				"dog":    []string{"apple"},
			},
			want: func() []*list.List {
				l := list.New()
				l.PushBack("apple")
				l.PushBack("banana")
				l.PushBack("cat")
				l.PushBack("dog")
				return []*list.List{l}
			}(),
		},
		{
			name: "spring cloud pig",
			l: func() *list.List {
				l := list.New()
				l.PushBack("banana")
				return l
			}(),
			sid2depsids: map[string][]string{
				"apple":  []string{"banana"},
				"banana": []string{"apple", "cat", "dog"},
			},
			want: func() []*list.List {
				l1 := list.New()
				l1.PushBack("banana")

				l2 := list.New()
				l2.PushBack("banana")
				l2.PushBack("cat")

				l3 := list.New()
				l3.PushBack("banana")
				l3.PushBack("dog")

				return []*list.List{l1, l2, l3}
			}(),
		},
	}

	for idx := range tests {
		tc := tests[idx]
		t.Run(tc.name, func(t *testing.T) {
			sd := &ServiceDependency{
				sid2depsids: tc.sid2depsids,
			}
			got := sd.buildLinkListByHead(tc.l)
			if !listsEqual(got, tc.want) {
				t.Errorf("expected %#v, but got %#v", linkedLists2String(tc.want), linkedLists2String(got))
			}
		})
	}
}

func listsEqual(got, want []*list.List) bool {
	if len(got) != len(want) {
		return false
	}

	listEqual := func(g, w *list.List) bool {
		gele := g.Front()
		wele := w.Front()
		for gele != nil {
			if gele.Value != wele.Value {
				return false
			}
			gele = gele.Next()
			wele = wele.Next()
		}

		return true
	}

	for _, g := range got {
		flag := false
		for _, w := range want {
			if g.Len() != w.Len() {
				continue
			}
			// check if linked list g is equals to w
			if listEqual(g, w) {
				flag = true
				break
			}
		}
		if !flag {
			return false
		}
	}

	return true
}

func linkedLists2String(lists []*list.List) string {
	var strs []string
	for _, l := range lists {
		var lstrs []string
		cur := l.Front()
		for cur != nil {
			lstrs = append(lstrs, cur.Value.(string))
			cur = cur.Next()
		}
		strs = append(strs, strings.Join(lstrs, "->"))
	}

	return strings.Join(strs, "; ")
}

func TestServiceStartupSequence(t *testing.T) {
	tests := []struct {
		name        string
		serviceIDS  []string
		sid2depsids map[string][]string
		depsid2sids map[string][]string
		want        map[string][]string
	}{
		{
			name:       "one to two",
			serviceIDS: []string{"apple", "banana", "cat"},
			sid2depsids: map[string][]string{
				"apple": []string{
					"banana",
					"cat",
				},
			},
			depsid2sids: map[string][]string{
				"banana": []string{
					"apple",
				},
				"cat": []string{
					"apple",
				},
			},
			want: map[string][]string{
				"apple": []string{
					"banana",
					"cat",
				},
			},
		},
		{
			name:       "a circle",
			serviceIDS: []string{"apple", "banana", "cat"},
			sid2depsids: map[string][]string{
				"apple": []string{
					"banana",
				},
				"banana": []string{
					"cat",
				},
				"cat": []string{
					"apple",
				},
			},
			depsid2sids: map[string][]string{
				"banana": []string{
					"apple",
				},
				"cat": []string{
					"banana",
				},
				"apple": []string{
					"cat",
				},
			},
			want: map[string][]string{
				"apple": []string{
					"banana",
				},
				"banana": []string{
					"cat",
				},
			},
		},
	}

	equal := func(want, got map[string][]string) bool {
		if len(want) != len(got) {
			return false
		}

		for wk, wv := range want {
			gv := got[wk]
			if len(wv) != len(gv) {
				return false
			}

			flag := false
			for _, wsid := range wv {
				for _, gsid := range gv {
					if wsid == gsid {
						flag = true
						break
					}
				}
				if !flag {
					return false
				}
			}
		}

		return true
	}

	for idx := range tests {
		tc := tests[idx]
		t.Run(tc.name, func(t *testing.T) {
			sd := &ServiceDependency{
				serviceIDs:  tc.serviceIDS,
				sid2depsids: tc.sid2depsids,
				depsid2sids: tc.depsid2sids,
			}
			got := sd.serviceStartupSequence()
			if !equal(tc.want, got) {
				t.Errorf("expected %v, but got %v", tc.want, got)
			}
		})
	}
}
