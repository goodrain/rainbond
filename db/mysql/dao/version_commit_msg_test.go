package dao

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/goodrain/rainbond/db/model"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

// capability_id: rainbond.version.commit-message-utf8
func TestTruncateVersionCommitMsg(t *testing.T) {
	tests := []struct{ name, input, want string }{
		{"empty", "", ""},
		{"short Chinese", "修复构建：更新版本信息", "修复构建：更新版本信息"},
		{"ASCII boundary", strings.Repeat("a", 1024), strings.Repeat("a", 1024)},
		{"long ASCII", strings.Repeat("a", 1025), strings.Repeat("a", 1024)},
		{"split fullwidth punctuation", strings.Repeat("a", 1022) + "：提交信息", strings.Repeat("a", 1022)},
		{"split two byte character", strings.Repeat("a", 1023) + "é", strings.Repeat("a", 1023)},
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

// capability_id: rainbond.version.commit-message-utf8
func TestTruncateVersionCommitMsgMalformedInputDoesNotPanic(t *testing.T) {
	// Truncation must not introduce a panic when upstream data has invalid encoding.
	input := strings.Repeat("\x80", 1025)
	got := truncateVersionCommitMsg(input)
	if len(got) > 1024 || !strings.HasPrefix(input, got) {
		t.Fatal("truncation did not return a bounded prefix")
	}
}

// capability_id: rainbond.version.commit-message-utf8
func TestVersionCommitMsgPersistence(t *testing.T) {
	database, err := gorm.Open("sqlite3", filepath.Join(t.TempDir(), "commit-messages.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	database.LogMode(false)
	if err := database.AutoMigrate(&model.CleanupStorage{}, &model.CleanupOperation{}, &model.KeyValue{}, &model.VersionInfo{}).Error; err != nil {
		t.Fatal(err)
	}
	versions := &VersionInfoDaoImpl{DB: database}
	cases := []struct{ name, input, want string }{
		{"empty", "", ""},
		{"short multiline", "fix: 修复构建\n\n保留详情和标点：完成。", "fix: 修复构建\n\n保留详情和标点：完成。"},
		{"exact limit", strings.Repeat("a", 1021) + "中", strings.Repeat("a", 1021) + "中"},
		{"long ASCII", strings.Repeat("a", 4096), strings.Repeat("a", 1024)},
		{"split two byte", strings.Repeat("a", 1023) + "é", strings.Repeat("a", 1023)},
		{"reported error", strings.Repeat("a", 1022) + "：提交信息", strings.Repeat("a", 1022)},
		{"long Chinese", strings.Repeat("中", 2048), strings.Repeat("中", 341)},
		{"split four byte", strings.Repeat("a", 1023) + "😀", strings.Repeat("a", 1023)},
	}
	for index, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			record := &model.VersionInfo{ServiceID: "service", BuildVersion: fmt.Sprintf("build-%d", index), EventID: fmt.Sprintf("event-%d", index), CommitMsg: tt.input, FinalStatus: "building", ImageName: "goodrain.me/team/component:v1"}
			if err := versions.AddModel(record); err != nil {
				t.Fatalf("create version: %v", err)
			}
			stored, err := versions.GetVersionByDeployVersion(record.BuildVersion, record.ServiceID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.CommitMsg != tt.want || !utf8.ValidString(stored.CommitMsg) {
				t.Fatal("create persisted an incorrect commit message")
			}
			revision := stored.ActivationRevision
			// Reset the full message so UpdateModel cannot rely on AddModel's truncation.
			record.CommitMsg = tt.input
			record.FinalStatus = "success"
			if err := versions.UpdateModel(record); err != nil {
				t.Fatalf("finish version: %v", err)
			}
			stored, err = versions.GetVersionByDeployVersion(record.BuildVersion, record.ServiceID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.CommitMsg != tt.want || !utf8.ValidString(stored.CommitMsg) {
				t.Fatal("update persisted an incorrect commit message")
			}
			if stored.FinalStatus != "success" || stored.ActivationRevision != revision {
				t.Fatal("build completion or activation revision was changed")
			}
		})
	}
}

// capability_id: rainbond.version.commit-message-utf8
func FuzzTruncateVersionCommitMsg(f *testing.F) {
	for _, seed := range []string{"", "修复构建", strings.Repeat("a", 1022) + "：提交信息", strings.Repeat("中", 342), strings.Repeat("a", 1023) + "😀", strings.Repeat("\x80", 1025)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		got := truncateVersionCommitMsg(input)
		if len(got) > 1024 || !strings.HasPrefix(input, got) {
			t.Fatal("result is not a bounded prefix")
		}
		if !utf8.ValidString(input) {
			return
		}
		if !utf8.ValidString(got) {
			t.Fatal("truncation broke UTF-8")
		}
		if truncateVersionCommitMsg(got) != got {
			t.Fatal("truncation is not idempotent")
		}
		if len(got) < len(input) {
			_, nextSize := utf8.DecodeRuneInString(input[len(got):])
			if len(got)+nextSize <= 1024 {
				t.Fatal("truncation discarded a complete character that fits")
			}
		}
	})
}
