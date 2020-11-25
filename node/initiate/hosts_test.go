package initiate

import (
	"context"
	"fmt"
	"github.com/goodrain/rainbond/cmd/node/option"
	"github.com/goodrain/rainbond/discover.v2"
	"k8s.io/client-go/kubernetes"
	"reflect"
	"testing"

	k8sutil "github.com/goodrain/rainbond/util/k8s"
)

func TestHosts_Cleanup(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  []string
	}{
		{
			name:  "no rainbond hosts",
			lines: []string{"127.0.0.1 localhost", "255.255.255.255 broadcasthost"},
			want:  []string{"127.0.0.1 localhost", "255.255.255.255 broadcasthost"},
		},
		{
			name:  "have rainbond hosts",
			lines: []string{"127.0.0.1 localhost", "255.255.255.255 broadcasthost", startOfSection, "foobar", endOfSection},
			want:  []string{"127.0.0.1 localhost", "255.255.255.255 broadcasthost"},
		},
	}

	for i := range tests {
		tc := tests[i]
		t.Run(tc.name, func(t *testing.T) {
			hosts := new(Hosts)
			for _, line := range tc.lines {
				hosts.Lines = append(hosts.Lines, NewHostsLine(line))
			}
			var wantLines []HostsLine
			for _, line := range tc.want {
				wantLines = append(wantLines, NewHostsLine(line))
			}

			if err := hosts.Cleanup(); err != nil {
				t.Error(err)
			}
			if !reflect.DeepEqual(hosts.Lines, wantLines) {
				t.Errorf("want %#v, bug got %#v", wantLines, hosts.Lines)
			}
		})
	}
}

func TestHosts_Add(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		add   []string
		want  []string
	}{
		{
			name:  "no rainbond hosts",
			lines: []string{"127.0.0.1 localhost", "255.255.255.255 broadcasthost"},
			want:  []string{"127.0.0.1 localhost", "255.255.255.255 broadcasthost"},
		},
		{
			name:  "have rainbond hosts",
			lines: []string{"127.0.0.1 localhost", "1.2.3.4 foobar", startOfSection, "1.2.3.4 goodrain.me", endOfSection},
			want:  []string{"127.0.0.1 localhost", "1.2.3.4 foobar", startOfSection, "1.2.3.4 goodrain.me", endOfSection},
		},
	}

	for i := range tests {
		tc := tests[i]
		t.Run(tc.name, func(t *testing.T) {
			hosts := new(Hosts)
			for _, line := range tc.lines {
				hosts.Add(line)
			}
			var wantLines []HostsLine
			for _, line := range tc.want {
				wantLines = append(wantLines, NewHostsLine(line))
			}
			if !reflect.DeepEqual(hosts.Lines, wantLines) {
				t.Errorf("want %#v, bug got %#v", wantLines, hosts.Lines)
			}
		})
	}
}

func TestHostManager_Start(t *testing.T) {
	config, err := k8sutil.NewRestConfig("/Users/abewang/.kube/config")
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	ctx := context.Background()
	cfg := &option.Conf{
		RbdNamespace:        "rbd-system",
		ImageRepositoryHost: "goodrain.me",
	}
	k8sDiscover := discover.NewK8sDiscover(ctx, clientset, cfg)
	defer k8sDiscover.Stop()

	hostManager, err := NewHostManager(cfg, k8sDiscover)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	hostManager.Start()

	fmt.Println("oook")

	select {}
}
