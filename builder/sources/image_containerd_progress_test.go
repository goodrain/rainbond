package sources

import (
	"testing"

	ctrcontent "github.com/containerd/containerd/cmd/ctr/commands/content"
)

func TestStatusInfoChanged(t *testing.T) {
	tests := []struct {
		name string
		prev ctrcontent.StatusInfo
		cur  ctrcontent.StatusInfo
		want bool
	}{
		{
			name: "first emission for a ref",
			prev: ctrcontent.StatusInfo{},
			cur:  ctrcontent.StatusInfo{Ref: "img", Status: "resolving"},
			want: true,
		},
		{
			name: "unchanged resolving status is suppressed",
			prev: ctrcontent.StatusInfo{Ref: "img", Status: "resolving"},
			cur:  ctrcontent.StatusInfo{Ref: "img", Status: "resolving"},
			want: false,
		},
		{
			name: "status transition is emitted",
			prev: ctrcontent.StatusInfo{Ref: "img", Status: "resolving"},
			cur:  ctrcontent.StatusInfo{Ref: "img", Status: "downloading"},
			want: true,
		},
		{
			name: "download progress advance is emitted",
			prev: ctrcontent.StatusInfo{Ref: "layer", Status: "downloading", Offset: 100, Total: 1000},
			cur:  ctrcontent.StatusInfo{Ref: "layer", Status: "downloading", Offset: 200, Total: 1000},
			want: true,
		},
		{
			name: "stalled download is suppressed",
			prev: ctrcontent.StatusInfo{Ref: "layer", Status: "downloading", Offset: 100, Total: 1000},
			cur:  ctrcontent.StatusInfo{Ref: "layer", Status: "downloading", Offset: 100, Total: 1000},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statusInfoChanged(tt.prev, tt.cur); got != tt.want {
				t.Errorf("statusInfoChanged() = %v, want %v", got, tt.want)
			}
		})
	}
}
