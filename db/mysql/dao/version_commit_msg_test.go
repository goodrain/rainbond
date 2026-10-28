package dao

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// capability_id: rainbond.version.commit-message-utf8
func TestTruncateVersionCommitMsg(t *testing.T) {
	tests := []struct{ name, input, want string }{
		{"empty", "", ""},
		{"short Chinese", "修复构建：更新版本信息", "修复构建：更新版本信息"},
		{"ASCII boundary", strings.Repeat("a", 1024), strings.Repeat("a", 1024)},
		{"long ASCII", strings.Repeat("a", 1025), strings.Repeat("a", 1024)},
		{"split fullwidth punctuation", strings.Repeat("a", 1022) + "：提交信息", strings.Repeat("a", 1022)},
		{"split Chinese", strings.Repeat("中", 342), strings.Repeat("中", 341)},
		{"split four byte character", strings.Repeat("a", 1023) + "😀tail", strings.Repeat("a", 1023)},
		{"complete multibyte boundary", strings.Repeat("a", 1021) + "中tail", strings.Repeat("a", 1021) + "中"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateVersionCommitMsg(tt.input)
			if got != tt.want {
				t.Fatalf("unexpected retained message: got %d bytes, want %d bytes", len(got), len(tt.want))
			}
			if !utf8.ValidString(got) {
				t.Fatal("truncation produced invalid UTF-8")
			}
			if len(got) > 1024 {
				t.Fatalf("message exceeds byte limit: %d", len(got))
			}
		})
	}
}
