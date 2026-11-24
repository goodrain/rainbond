package copyio

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runnerPlan(t *testing.T) (Plan, string) {
	t.Helper()
	base := t.TempDir()
	for _, name := range []string{"input/0", "previous/0", "target/0"} {
		if err := os.MkdirAll(filepath.Join(base, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(base, "input/0/disk.img")
	previous := filepath.Join(base, "previous/0/disk.img")
	sampleFile(t, source, 3*ChunkBytes+77)
	sampleFile(t, previous, 4*ChunkBytes+99)
	plan := Plan{Protocol: Protocol, OperationID: "operation-a", ManifestHash: strings.Repeat("a", 64), FrameHash: strings.Repeat("b", 64), Mode: "inspect", DeadlineSeconds: 60, TargetUID: uint32(os.Getuid()), TargetGID: uint32(os.Getgid()), Disks: []DiskPlan{{VolumeID: 7, SourcePVCUID: "rox-uid", PreviousPVCUID: "source-b-uid", TargetPVCUID: "target-a-uid", SourceMaxBytes: 4 * ChunkBytes, PreviousMaxBytes: 5 * ChunkBytes}}}
	plan, err := SealPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	return plan, base
}
func copyPlan(t *testing.T, plan Plan, inspection Report) Plan {
	t.Helper()
	plan.Mode = "copy"
	disk := inspection.Disks[0]
	plan.Disks[0].ExpectedSourceBytes = disk.Source.LogicalBytes
	plan.Disks[0].ExpectedSourceSHA256 = disk.Source.SHA256
	plan.Disks[0].ExpectedPreviousBytes = disk.PreviousBefore.LogicalBytes
	plan.Disks[0].ExpectedPreviousSHA256 = disk.PreviousBefore.SHA256
	plan.Disks[0].WriteLimitBytes = disk.Source.NonzeroChunkBytes
	sealed, err := SealPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	return sealed
}

// capability_id: rainbond.vm-restore.copy-results-before-assert
func TestInspectCopyStoresTypedEvidenceBeforeCheckingCrossStageHashes(t *testing.T) {
	plan, base := runnerPlan(t)
	saved := []Report{}
	save := func(report Report) error { saved = append(saved, report); return nil }
	inspection, err := Run(context.Background(), plan, base, save)
	if err != nil || !inspection.OK {
		t.Fatal("inspection failed")
	}
	plan = copyPlan(t, plan, inspection)
	plan.Disks[0].ExpectedSourceSHA256 = strings.Repeat("c", 64)
	plan, _ = SealPlan(plan)
	report, err := Run(context.Background(), plan, base, save)
	if !errors.Is(err, ErrDigest) || report.OK || len(saved) < 3 {
		t.Fatal("mismatched source was accepted or evidence lost")
	}
	last := saved[len(saved)-1]
	if last.ErrorCode != "copy_full_digest_mismatch" || last.Disks[0].Source.SHA256 == "" || last.Disks[0].PreviousBefore.SHA256 == "" {
		t.Fatal("assertion occurred before full source/previous evidence persisted")
	}
	if _, err := os.Stat(filepath.Join(base, "target/0/disk.img.partial")); !os.IsNotExist(err) {
		t.Fatal("source hash mismatch already wrote target")
	}
}

func TestFullCopyPublishesOnlyVerifiedTargetAndKeepsB(t *testing.T) {
	plan, base := runnerPlan(t)
	save := func(Report) error { return nil }
	inspection, err := Run(context.Background(), plan, base, save)
	if err != nil {
		t.Fatal(err)
	}
	plan = copyPlan(t, plan, inspection)
	result, err := Run(context.Background(), plan, base, save)
	if err != nil || !result.OK || !result.Disks[0].Published {
		t.Fatal("verified copy did not publish")
	}
	disk := result.Disks[0]
	if disk.Source.SHA256 != disk.Target.SHA256 || disk.PreviousBefore.SHA256 != disk.PreviousAfter.SHA256 {
		t.Fatal("copy or retained B digest changed")
	}
	target, err := os.Stat(filepath.Join(base, "target/0/disk.img"))
	if err != nil || target.Mode().Perm() != 0660 {
		t.Fatal("KubeVirt target mode not applied")
	}
}

func TestPlanTamperingAndNilPersisterFailClosed(t *testing.T) {
	plan, base := runnerPlan(t)
	plan.Disks[0].SourceMaxBytes++
	if err := plan.Validate(); !errors.Is(err, ErrInvalidPlan) {
		t.Fatal("plan hash did not bind read/write limits")
	}
	plan, _ = SealPlan(plan)
	if _, err := Run(context.Background(), plan, base, nil); !errors.Is(err, ErrInvalidPlan) {
		t.Fatal("missing result persistence was accepted")
	}
}

// capability_id: rainbond.vm-restore.copy-retry-verification
func TestRetryVerifiesPublishedTargetWithoutOverwritingAndDetectsCorruption(t *testing.T) {
	plan, base := runnerPlan(t)
	save := func(Report) error { return nil }
	inspection, err := Run(context.Background(), plan, base, save)
	if err != nil {
		t.Fatal(err)
	}
	plan = copyPlan(t, plan, inspection)
	if _, err := Run(context.Background(), plan, base, save); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), plan, base, save); !errors.Is(err, ErrIdentity) {
		t.Fatal("copy retry overwrote target")
	}
	plan.Mode = "verify"
	plan, _ = SealPlan(plan)
	verified, err := Run(context.Background(), plan, base, save)
	if err != nil || !verified.OK || !verified.Disks[0].Published || verified.Disks[0].WrittenBytes != 0 {
		t.Fatal("bounded verification of published target failed")
	}
	file, err := os.OpenFile(filepath.Join(base, "target/0/disk.img"), os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteAt([]byte("CORRUPTED_FIXTURE"), 8192); err != nil {
		t.Fatal(err)
	}
	file.Close()
	saved := Report{}
	_, err = Run(context.Background(), plan, base, func(r Report) error { saved = r; return nil })
	if !errors.Is(err, ErrDigest) || saved.Disks[0].Target.SHA256 == "" || saved.Disks[0].PreviousAfter.SHA256 == "" {
		t.Fatal("verify accepted corruption or lost full evidence")
	}
}

func TestCancellationAndPersistenceFailureRetainTypedFailure(t *testing.T) {
	plan, base := runnerPlan(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	saved := Report{}
	_, err := Run(ctx, plan, base, func(r Report) error { saved = r; return nil })
	if !errors.Is(err, context.Canceled) || saved.ErrorCode != "copy_cancelled" {
		t.Fatal("cancel did not persist bounded typed failure")
	}
	rejected := errors.New("fixture persistence failed")
	_, err = Run(context.Background(), plan, base, func(Report) error { return rejected })
	if !errors.Is(err, rejected) {
		t.Fatal("failed durable result store was ignored")
	}
}

func TestRunCancellationBeforePublicationLeavesOnlyPartialTarget(t *testing.T) {
	plan, base := runnerPlan(t)
	inspection, err := Run(context.Background(), plan, base, func(Report) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	plan = copyPlan(t, plan, inspection)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	last := Report{}
	result, err := Run(ctx, plan, base, func(r Report) error {
		last = r
		if r.Phase == "previous-after" {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) || result.OK || last.OK || last.ErrorCode != "copy_cancelled" {
		t.Fatal("cancellation was accepted as success")
	}
	if _, err := os.Lstat(filepath.Join(base, "target/0/disk.img")); !os.IsNotExist(err) {
		t.Fatal("cancelled operation published its partial target")
	}
}

func TestRunNeverFollowsReplacementPartialSymlinkDuringPublish(t *testing.T) {
	plan, base := runnerPlan(t)
	inspection, err := Run(context.Background(), plan, base, func(Report) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	plan = copyPlan(t, plan, inspection)
	victim := filepath.Join(base, "victim")
	if err := os.WriteFile(victim, []byte("untouched victim fixture"), 0400); err != nil {
		t.Fatal(err)
	}
	_, err = Run(context.Background(), plan, base, func(r Report) error {
		if r.Phase == "previous-after" && r.ErrorCode == "" {
			partial := filepath.Join(base, "target/0/disk.img.partial")
			if err := os.Remove(partial); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(victim, partial); err != nil {
				t.Fatal(err)
			}
		}
		return nil
	})
	if err == nil {
		t.Fatal("replacement partial symlink accepted")
	}
	info, err := os.Stat(victim)
	if err != nil || info.Mode().Perm() != 0400 {
		t.Fatal("publish followed a symlink and changed an unrelated file")
	}
}

func TestRunPersistenceFailureBeforePublishNeverReturnsSuccess(t *testing.T) {
	plan, base := runnerPlan(t)
	inspection, err := Run(context.Background(), plan, base, func(Report) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	plan = copyPlan(t, plan, inspection)
	rejected := errors.New("fixture persistence rejected")
	var persisted [][]byte
	report, err := Run(context.Background(), plan, base, func(r Report) error {
		data, marshalErr := json.Marshal(r)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if r.Phase == "previous-after" && r.ErrorCode == "" {
			return rejected
		}
		persisted = append(persisted, data)
		return nil
	})
	if !errors.Is(err, rejected) || report.OK || len(persisted) == 0 {
		t.Fatal("failed evidence persistence became success")
	}
	var last Report
	if json.Unmarshal(persisted[len(persisted)-1], &last) != nil || last.OK || last.ErrorCode == "" || last.Disks[0].Target.SHA256 == "" || last.Disks[0].PreviousAfter.SHA256 == "" {
		t.Fatal("failure lost complete typed pre-assertion evidence")
	}
	if _, err := os.Lstat(filepath.Join(base, "target/0/disk.img")); !os.IsNotExist(err) {
		t.Fatal("evidence failure published a target")
	}
}

func TestRunProtectsTargetCollisionDuringPublication(t *testing.T) {
	plan, base := runnerPlan(t)
	inspection, err := Run(context.Background(), plan, base, func(Report) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	plan = copyPlan(t, plan, inspection)
	target := filepath.Join(base, "target/0/disk.img")
	report, err := Run(context.Background(), plan, base, func(r Report) error {
		if r.Phase == "previous-after" && r.ErrorCode == "" {
			if err := os.WriteFile(target, []byte("existing target fixture"), 0400); err != nil {
				t.Fatal(err)
			}
		}
		return nil
	})
	if !errors.Is(err, ErrIdentity) || report.OK {
		t.Fatal("late target collision became success")
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "existing target fixture" {
		t.Fatal("publication overwrote an existing target")
	}
}

func TestRunRejectsPartialIdentityChangeAfterMetadataSync(t *testing.T) {
	plan, base := runnerPlan(t)
	inspection, err := Run(context.Background(), plan, base, func(Report) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	plan = copyPlan(t, plan, inspection)
	ctx := &beforeReadContext{Context: context.Background()}
	report, err := Run(ctx, plan, base, func(r Report) error {
		if r.Phase == "previous-after" && r.ErrorCode == "" {
			// The first publish cancellation check precedes FD metadata updates;
			// the second occurs after syncing that FD and before publication.
			ctx.hook = func() {
				ctx.hook = func() {
					partial := filepath.Join(base, "target/0/disk.img.partial")
					if err := os.Rename(partial, filepath.Join(base, "owned-original-partial")); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(partial, []byte("replacement fixture"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
		return nil
	})
	if !errors.Is(err, ErrIdentity) || report.OK {
		t.Fatal("replacement partial inode became success")
	}
	if _, err := os.Lstat(filepath.Join(base, "target/0/disk.img")); !os.IsNotExist(err) {
		t.Fatal("publication used a different partial inode")
	}
}

func TestRunMultiDiskFailureNeverReportsPartialSetAsSuccess(t *testing.T) {
	plan, base := runnerPlan(t)
	for _, name := range []string{"input/1", "previous/1", "target/1"} {
		if err := os.MkdirAll(filepath.Join(base, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"input/1/disk.img", "previous/1/disk.img"} {
		if err := os.WriteFile(filepath.Join(base, name), []byte("second disk fixture"), 0400); err != nil {
			t.Fatal(err)
		}
	}
	second := plan.Disks[0]
	second.VolumeID, second.SourcePVCUID, second.PreviousPVCUID, second.TargetPVCUID = 8, "source-a-2", "previous-b-2", "target-a-2"
	plan.Disks = append(plan.Disks, second)
	plan, _ = SealPlan(plan)
	inspection, err := Run(context.Background(), plan, base, func(Report) error { return nil })
	if err != nil || len(inspection.Disks) != 2 {
		t.Fatal("multi-disk inspection did not report complete disk set")
	}
	plan.Mode = "copy"
	for i, disk := range inspection.Disks {
		plan.Disks[i].ExpectedSourceBytes, plan.Disks[i].ExpectedSourceSHA256 = disk.Source.LogicalBytes, disk.Source.SHA256
		plan.Disks[i].ExpectedPreviousBytes, plan.Disks[i].ExpectedPreviousSHA256 = disk.PreviousBefore.LogicalBytes, disk.PreviousBefore.SHA256
		plan.Disks[i].WriteLimitBytes = disk.Source.NonzeroChunkBytes
	}
	plan.Disks[1].WriteLimitBytes = 0
	plan, _ = SealPlan(plan)
	var last []byte
	report, err := Run(context.Background(), plan, base, func(r Report) error {
		var marshalErr error
		last, marshalErr = json.Marshal(r)
		return marshalErr
	})
	if !errors.Is(err, ErrWriteBudget) || report.OK || len(report.Disks) != 2 || !report.Disks[0].Published || report.Disks[1].Published {
		t.Fatal("multi-disk failure was collapsed into partial success")
	}
	var persisted Report
	if json.Unmarshal(last, &persisted) != nil || persisted.OK || persisted.ErrorCode != "copy_write_budget_exceeded" || len(persisted.Disks) != 2 || persisted.Disks[1].WrittenBytes != 0 {
		t.Fatal("multi-disk failure lost complete typed result and actual write count")
	}
	data, err := os.ReadFile(filepath.Join(base, "previous/1/disk.img"))
	if err != nil || string(data) != "second disk fixture" {
		t.Fatal("multi-disk failure changed retained B")
	}
	if _, err := os.Lstat(filepath.Join(base, "target/1/disk.img")); !os.IsNotExist(err) {
		t.Fatal("failed disk was published")
	}
}
