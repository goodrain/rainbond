package conversion

import (
	"reflect"
	"testing"
)

// capability_id: rainbond.worker.conversion.cmd-args-yaml
func TestParseStringSequenceAttribute(t *testing.T) {
	longShell := "/entrypoint.sh;while true;do echo hello >/dev/null;sleep 1;done"
	tests := []struct {
		name    string
		value   string
		want    []string
		wantErr bool
	}{
		{
			name:  "yaml sequence keeps one shell line as one argv item",
			value: "- " + longShell + "\n",
			want:  []string{longShell},
		},
		{
			name:  "yaml sequence preserves spaces inside items",
			value: "- /bin/sh\n- -c\n- echo hello world\n",
			want:  []string{"/bin/sh", "-c", "echo hello world"},
		},
		{
			name:  "json sequence remains supported",
			value: `["/bin/sh","-c","echo hello world"]`,
			want:  []string{"/bin/sh", "-c", "echo hello world"},
		},
		{
			name:  "legacy plain string is one argv item",
			value: "npm start -- --port 3000",
			want:  []string{"npm start -- --port 3000"},
		},
		{
			name:    "yaml sequence rejects non string items",
			value:   "- /bin/sh\n- 123\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseStringSequenceAttribute(tt.value)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseStringSequenceAttribute() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
