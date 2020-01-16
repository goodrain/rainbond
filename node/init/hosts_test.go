package init

import (
	"reflect"
	"testing"
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
			lines: []string{"127.0.0.1 localhost", "255.255.255.255 broadcasthost", StartOfSection, "foobar", EndOfSection},
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
			lines: []string{"127.0.0.1 localhost", "1.2.3.4 foobar", StartOfSection, "1.2.3.4 goodrain.me", EndOfSection},
			want:  []string{"127.0.0.1 localhost", "1.2.3.4 foobar", StartOfSection, "1.2.3.4 goodrain.me", EndOfSection},
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
