package sources

import "testing"

// capability_id: rainbond.source-svn.branch-path
func TestGetBranchPath(t *testing.T) {
	tests := []struct {
		branch string
		url    string
		want   string
	}{
		{branch: "trunk", url: "https://svn.example.com/repo", want: "https://svn.example.com/repo/trunk"},
		{branch: "trunk/app", url: "https://svn.example.com/repo", want: "https://svn.example.com/repo/trunk/app"},
		{branch: "tag:v2.3.6", url: "https://svn.example.com/repo", want: "https://svn.example.com/repo/tags/v2.3.6"},
		{branch: "feature-x", url: "https://svn.example.com/repo", want: "https://svn.example.com/repo/branches/feature-x"},
	}

	for _, tt := range tests {
		if got := getBranchPath(tt.branch, tt.url); got != tt.want {
			t.Fatalf("getBranchPath(%q, %q)=%q, want %q", tt.branch, tt.url, got, tt.want)
		}
	}
}
