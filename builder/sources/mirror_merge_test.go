package sources

import "testing"

// capability_id: rainbond.builder.mirror-merge-manual-priority
func TestMergeMirrors(t *testing.T) {
	tests := []struct {
		name    string
		manual  []string
		dynamic []string
		want    []string
	}{
		{
			name: "both empty",
		},
		{
			name:    "dynamic only",
			dynamic: []string{"https://a.example.com", "https://b.example.com"},
			want:    []string{"https://a.example.com", "https://b.example.com"},
		},
		{
			name:   "manual only",
			manual: []string{"https://a.example.com"},
			want:   []string{"https://a.example.com"},
		},
		{
			name:    "manual first then dynamic deduped by host",
			manual:  []string{"https://a.example.com", "http://c.example.com"},
			dynamic: []string{"https://b.example.com", "https://a.example.com"},
			want:    []string{"https://a.example.com", "http://c.example.com", "https://b.example.com"},
		},
		{
			name:    "dedup ignores scheme difference",
			manual:  []string{"http://a.example.com"},
			dynamic: []string{"https://a.example.com", "a.example.com"},
			want:    []string{"http://a.example.com"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeMirrors(tt.manual, tt.dynamic)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}
