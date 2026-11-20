package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/pkg/vmrestore/copyio"
)

func commandFixture(t *testing.T) (runtimePaths, copyio.Plan) {
	t.Helper()
	base := t.TempDir()
	paths := runtimePaths{plan: filepath.Join(base, "plan.json"), volumes: filepath.Join(base, "volumes"), work: filepath.Join(base, "work"), termination: filepath.Join(base, "termination")}
	for _, dir := range []string{"input/0", "previous/0", "target/0"} {
		if err := os.MkdirAll(filepath.Join(paths.volumes, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"input/0/disk.img", "previous/0/disk.img"} {
		if err := os.WriteFile(filepath.Join(paths.volumes, name), []byte("guest fixture bytes"), 0400); err != nil {
			t.Fatal(err)
		}
	}
	plan := copyio.Plan{Protocol: copyio.Protocol, OperationID: "operation-a", ManifestHash: strings.Repeat("a", 64), FrameHash: strings.Repeat("b", 64), Mode: "inspect", DeadlineSeconds: 60, TargetUID: uint32(os.Getuid()), TargetGID: uint32(os.Getgid()), Disks: []copyio.DiskPlan{{VolumeID: 7, SourcePVCUID: "source-a", PreviousPVCUID: "previous-b", TargetPVCUID: "target-a", SourceMaxBytes: 100, PreviousMaxBytes: 100}}}
	plan, err := copyio.SealPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	writePlan(t, paths, plan)
	return paths, plan
}

func writePlan(t *testing.T, paths runtimePaths, plan copyio.Plan) {
	t.Helper()
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.plan, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCommandEmitsOneCompletePersistedReportAndExactSummary(t *testing.T) {
	paths, plan := commandFixture(t)
	var output bytes.Buffer
	if executeWithPaths(context.Background(), paths, &output) != 0 {
		t.Fatal("inspect command failed")
	}
	if bytes.Count(output.Bytes(), []byte{'\n'}) != 1 {
		t.Fatal("stdout must contain exactly one report line")
	}
	payload := bytes.TrimSuffix(output.Bytes(), []byte{'\n'})
	persisted, err := os.ReadFile(filepath.Join(paths.work, "report.json"))
	if err != nil || !bytes.Equal(payload, persisted) {
		t.Fatal("stdout report differs from persisted complete payload")
	}
	var report copyio.Report
	if json.Unmarshal(payload, &report) != nil || !report.OK || report.Phase != "complete" || len(report.Disks) != 1 {
		t.Fatal("stdout is not a complete typed result")
	}
	if bytes.Contains(payload, []byte("guest fixture bytes")) || bytes.Contains(payload, []byte(paths.volumes)) {
		t.Fatal("report disclosed guest data or runtime paths")
	}
	summaryData, err := os.ReadFile(paths.termination)
	if err != nil {
		t.Fatal(err)
	}
	var summary copyio.Summary
	digest := sha256.Sum256(payload)
	if json.Unmarshal(summaryData, &summary) != nil || len(summaryData) > 3500 || !summary.OK || summary.Protocol != copyio.Protocol || summary.OperationID != plan.OperationID || summary.PlanHash != plan.PlanHash || summary.ReportBytes != len(payload) || summary.ReportSHA256 != hex.EncodeToString(digest[:]) || summary.DiskCount != 1 {
		t.Fatal("termination summary does not bind complete report")
	}
}

func TestCommandCopyAndVerifyLeaveReadOnlySourcesUnchanged(t *testing.T) {
	paths, plan := commandFixture(t)
	var output bytes.Buffer
	if executeWithPaths(context.Background(), paths, &output) != 0 {
		t.Fatal("inspection failed")
	}
	var inspected copyio.Report
	if json.Unmarshal(output.Bytes(), &inspected) != nil {
		t.Fatal("inspection report missing")
	}
	plan.Mode = "copy"
	disk := inspected.Disks[0]
	plan.Disks[0].ExpectedSourceBytes, plan.Disks[0].ExpectedSourceSHA256 = disk.Source.LogicalBytes, disk.Source.SHA256
	plan.Disks[0].ExpectedPreviousBytes, plan.Disks[0].ExpectedPreviousSHA256 = disk.PreviousBefore.LogicalBytes, disk.PreviousBefore.SHA256
	plan.Disks[0].WriteLimitBytes = disk.Source.NonzeroChunkBytes
	plan, _ = copyio.SealPlan(plan)
	writePlan(t, paths, plan)
	output.Reset()
	if executeWithPaths(context.Background(), paths, &output) != 0 {
		t.Fatal("copy command failed")
	}
	target, err := os.ReadFile(filepath.Join(paths.volumes, "target/0/disk.img"))
	if err != nil || string(target) != "guest fixture bytes" {
		t.Fatal("published target content differs from source fixture")
	}
	if executeWithPaths(context.Background(), paths, io.Discard) == 0 {
		t.Fatal("copy retry overwrote occupied target")
	}
	plan.Mode = "verify"
	plan, _ = copyio.SealPlan(plan)
	writePlan(t, paths, plan)
	output.Reset()
	if executeWithPaths(context.Background(), paths, &output) != 0 {
		t.Fatal("read-only retry verification failed")
	}
	var verified copyio.Report
	if json.Unmarshal(output.Bytes(), &verified) != nil || !verified.OK || !verified.Disks[0].Published || verified.Disks[0].WrittenBytes != 0 {
		t.Fatal("verification rewrote target or returned incomplete result")
	}
	digest := sha256.Sum256([]byte("guest fixture bytes"))
	if verified.Disks[0].Target.SHA256 != hex.EncodeToString(digest[:]) || verified.Disks[0].Target.Mode != 0660 || verified.Disks[0].Target.UID != uint32(os.Getuid()) || verified.Disks[0].Target.GID != uint32(os.Getgid()) {
		t.Fatal("published target lacks exact content or ownership verification")
	}
	for _, name := range []string{"input/0/disk.img", "previous/0/disk.img"} {
		data, err := os.ReadFile(filepath.Join(paths.volumes, name))
		info, statErr := os.Stat(filepath.Join(paths.volumes, name))
		if err != nil || statErr != nil || string(data) != "guest fixture bytes" || info.Mode().Perm() != 0400 {
			t.Fatal("command changed read-only source or retained B")
		}
	}
}

func TestCommandRejectsUnboundedUnknownOrMultiplePlansWithoutOutput(t *testing.T) {
	for _, kind := range []string{"unknown", "multiple", "oversize", "tampered", "missing"} {
		t.Run(kind, func(t *testing.T) {
			paths, plan := commandFixture(t)
			body, _ := json.Marshal(plan)
			switch kind {
			case "unknown":
				body = append([]byte(`{"command":"untrusted",`), body[1:]...)
			case "multiple":
				body = append(body, []byte(`{}`)...)
			case "oversize":
				body = bytes.Repeat([]byte{'x'}, copyio.MaxPlanBytes+1)
			case "tampered":
				plan.FrameHash = strings.Repeat("c", 64)
				body, _ = json.Marshal(plan)
			case "missing":
				paths.plan = filepath.Join(t.TempDir(), "missing")
			}
			if kind != "missing" {
				if err := os.WriteFile(paths.plan, body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			var output bytes.Buffer
			if executeWithPaths(context.Background(), paths, &output) == 0 || output.Len() != 0 {
				t.Fatal("invalid plan executed or emitted an untrusted report")
			}
			entries, err := os.ReadDir(filepath.Join(paths.volumes, "target/0"))
			if err != nil || len(entries) != 0 {
				t.Fatal("invalid plan changed target")
			}
		})
	}
}

func TestCommandCancellationEmitsTypedFailureAndNonzeroExit(t *testing.T) {
	paths, _ := commandFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	if executeWithPaths(ctx, paths, &output) == 0 {
		t.Fatal("cancelled command exited successfully")
	}
	var report copyio.Report
	if json.Unmarshal(output.Bytes(), &report) != nil || report.OK || report.ErrorCode != "copy_cancelled" {
		t.Fatal("cancelled command did not emit typed failure")
	}
	var summary copyio.Summary
	data, err := os.ReadFile(paths.termination)
	if err != nil || json.Unmarshal(data, &summary) != nil || summary.OK {
		t.Fatal("cancelled command summary claims success")
	}
}

func TestPersistRejectsOversizeAndSymlinkTemporaryWithoutAlteringVictim(t *testing.T) {
	dir := t.TempDir()
	path, victim := filepath.Join(dir, "report.json"), filepath.Join(dir, "victim")
	if err := os.WriteFile(victim, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := persist(path, copyio.Report{ErrorCode: strings.Repeat("x", copyio.MaxReportBytes+1)}); !errors.Is(err, copyio.ErrInvalidPlan) {
		t.Fatal("oversized report accepted")
	}
	if err := os.Symlink(victim, path+".tmp"); err != nil {
		t.Fatal(err)
	}
	if _, err := persist(path, copyio.Report{}); err == nil {
		t.Fatal("persist followed temporary symlink")
	}
	data, err := os.ReadFile(victim)
	if err != nil || string(data) != "untouched" {
		t.Fatal("persist changed temporary symlink victim")
	}
}

type partialOutput struct{}

func (partialOutput) Write(data []byte) (int, error) { return len(data) - 1, nil }

func TestCommandCannotSucceedOnOutputOrPersistenceFailure(t *testing.T) {
	for _, kind := range []string{"stdout", "work", "termination"} {
		t.Run(kind, func(t *testing.T) {
			paths, _ := commandFixture(t)
			var output io.Writer = io.Discard
			switch kind {
			case "stdout":
				output = partialOutput{}
			case "work":
				paths.work = paths.plan
			case "termination":
				paths.termination = t.TempDir()
			}
			if executeWithPaths(context.Background(), paths, output) == 0 {
				t.Fatal("failed report delivery exited successfully")
			}
		})
	}
}
