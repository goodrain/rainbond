package initiate

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFstab_Add(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  []*fstabLine
	}{
		{
			name:  "ok",
			lines: []string{"host.myserver.com:/home /mnt/home nfs rw,hard,intr,rsize=8192,wsize=8192,timeo=14 0 0"},
			want: []*fstabLine{
				{
					Raw:        "host.myserver.com:/home /mnt/home nfs rw,hard,intr,rsize=8192,wsize=8192,timeo=14 0 0",
					FileSystem: "host.myserver.com:/home",
					MountPoint: "/mnt/home",
					Type:       "nfs",
					Options:    "rw,hard,intr,rsize=8192,wsize=8192,timeo=14",
					Dump:       0,
					Pass:       0,
				},
			},
		},
		{
			name:  "wrong format",
			lines: []string{"host.myserver.com:/home /mnt/home nfs rw,hard,intr,rsize=8192,wsize=8192,timeo=14 0"},
			want: []*fstabLine{
				{
					Raw: "host.myserver.com:/home /mnt/home nfs rw,hard,intr,rsize=8192,wsize=8192,timeo=14 0",
					Err: errors.New(fmt.Sprintf("Bad fstab line: %q", "host.myserver.com:/home /mnt/home nfs rw,hard,intr,rsize=8192,wsize=8192,timeo=14 0")),
				},
			},
		},
		{
			name:  "wrong number",
			lines: []string{"host.myserver.com:/home /mnt/home nfs rw,hard,intr,rsize=8192,wsize=8192,timeo=14 0 abc"},
			want: []*fstabLine{
				{
					Raw:        "host.myserver.com:/home /mnt/home nfs rw,hard,intr,rsize=8192,wsize=8192,timeo=14 0 abc",
					FileSystem: "host.myserver.com:/home",
					MountPoint: "/mnt/home",
					Type:       "nfs",
					Options:    "rw,hard,intr,rsize=8192,wsize=8192,timeo=14",
					Dump:       0,
					Pass:       0,
				},
			},
		},
	}

	for idx := range tests {
		tc := tests[idx]
		t.Run(tc.name, func(t *testing.T) {
			f := Fstab{}
			for _, line := range tc.lines {
				f.Add(line)
			}
			assert.Equal(t, f.Lines, tc.want)
		})
	}

}
