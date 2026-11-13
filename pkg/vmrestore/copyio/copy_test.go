package copyio

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

type shortWriter struct {
	completed int
	max       int
	zero      bool
	invalid   bool
}

func primitivePlan() Plan {
	return Plan{Protocol: Protocol, OperationID: "operation-1", ManifestHash: strings.Repeat("a", 64), FrameHash: strings.Repeat("b", 64), Mode: "copy", DeadlineSeconds: 60,
		Disks: []DiskPlan{{VolumeID: 1, SourcePVCUID: "source", PreviousPVCUID: "previous", TargetPVCUID: "target", SourceMaxBytes: 1 << 40, PreviousMaxBytes: 1 << 40,
			ExpectedSourceBytes: 1 << 39, ExpectedPreviousBytes: 1 << 39, ExpectedSourceSHA256: strings.Repeat("c", 64), ExpectedPreviousSHA256: strings.Repeat("d", 64), WriteLimitBytes: 0}}}
}

func TestPlanSealsEveryIdentityAndRejectsInvalidStageBounds(t *testing.T) {
	for _, mode := range []string{"inspect", "copy", "verify"} {
		plan := primitivePlan()
		plan.Mode = mode
		sealed, err := SealPlan(plan)
		if err != nil || sealed.Validate() != nil {
			t.Fatalf("valid %s plan rejected", mode)
		}
		sealed.Disks[0].SourceMaxBytes--
		if !errors.Is(sealed.Validate(), ErrInvalidPlan) {
			t.Fatal("sealed bounds could be changed")
		}
	}
	mutations := map[string]func(*Plan){
		"protocol":                  func(p *Plan) { p.Protocol++ },
		"operation path":            func(p *Plan) { p.OperationID = "../source" },
		"manifest digest":           func(p *Plan) { p.ManifestHash = "invalid" },
		"frame digest":              func(p *Plan) { p.FrameHash = "invalid" },
		"mode":                      func(p *Plan) { p.Mode = "shell" },
		"deadline zero":             func(p *Plan) { p.DeadlineSeconds = 0 },
		"deadline excessive":        func(p *Plan) { p.DeadlineSeconds = 86401 },
		"no disks":                  func(p *Plan) { p.Disks = nil },
		"too many disks":            func(p *Plan) { p.Disks = make([]DiskPlan, 129) },
		"volume zero":               func(p *Plan) { p.Disks[0].VolumeID = 0 },
		"duplicate volume":          func(p *Plan) { p.Disks = append(p.Disks, p.Disks[0]) },
		"source uid path":           func(p *Plan) { p.Disks[0].SourcePVCUID = "../source" },
		"previous uid missing":      func(p *Plan) { p.Disks[0].PreviousPVCUID = "" },
		"target uid missing":        func(p *Plan) { p.Disks[0].TargetPVCUID = "" },
		"source bound":              func(p *Plan) { p.Disks[0].SourceMaxBytes = 0 },
		"previous bound":            func(p *Plan) { p.Disks[0].PreviousMaxBytes = 0 },
		"source length zero":        func(p *Plan) { p.Disks[0].ExpectedSourceBytes = 0 },
		"source length excessive":   func(p *Plan) { p.Disks[0].ExpectedSourceBytes = p.Disks[0].SourceMaxBytes + 1 },
		"source digest":             func(p *Plan) { p.Disks[0].ExpectedSourceSHA256 = "" },
		"previous length zero":      func(p *Plan) { p.Disks[0].ExpectedPreviousBytes = 0 },
		"previous length excessive": func(p *Plan) { p.Disks[0].ExpectedPreviousBytes = p.Disks[0].PreviousMaxBytes + 1 },
		"previous digest":           func(p *Plan) { p.Disks[0].ExpectedPreviousSHA256 = "" },
		"negative budget":           func(p *Plan) { p.Disks[0].WriteLimitBytes = -1 },
		"excessive budget":          func(p *Plan) { p.Disks[0].WriteLimitBytes = p.Disks[0].ExpectedSourceBytes + 1 },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			plan := primitivePlan()
			mutate(&plan)
			plan, err := SealPlan(plan)
			if err != nil || !errors.Is(plan.Validate(), ErrInvalidPlan) {
				t.Fatal("invalid plan accepted after sealing")
			}
		})
	}
	source, previous, target := Directories("/volumes", 2)
	if source != "/volumes/input/2" || previous != "/volumes/previous/2" || target != "/volumes/target/2" {
		t.Fatal("fixed mount directories changed")
	}
}

func TestScanRejectsFIFOWithoutWaitingForWriter(t *testing.T) {
	if os.Getenv("COPYIO_FIFO_TEST_CHILD") != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestScanRejectsFIFOWithoutWaitingForWriter$")
		child.Env = append(os.Environ(), "COPYIO_FIFO_TEST_CHILD=1")
		if err := child.Run(); err != nil {
			t.Fatal("FIFO scan did not reject the nonregular source within its deadline")
		}
		return
	}
	path := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(context.Background(), path, 100); !errors.Is(err, ErrIdentity) {
		t.Fatal("FIFO was not rejected as a nonregular source")
	}
}

type beforeReadContext struct {
	context.Context
	hook func()
}

func (c *beforeReadContext) Err() error {
	if c.hook != nil {
		hook := c.hook
		c.hook = nil
		hook()
	}
	return c.Context.Err()
}

func TestScanRecordsActualBytesOnSourceShortRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source")
	if err := os.WriteFile(path, []byte("original fixture contents"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx := &beforeReadContext{Context: context.Background(), hook: func() {
		if err := os.Truncate(path, 5); err != nil {
			t.Fatal(err)
		}
	}}
	result, err := Scan(ctx, path, 100)
	if !errors.Is(err, io.ErrUnexpectedEOF) || result.ReadBytes != 5 || result.SHA256 != "" {
		t.Fatal("short read lost actual byte count or was reported as a full digest")
	}
}

func TestScanGrowthProbeCountsEveryActualRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source")
	if err := os.WriteFile(path, []byte("abc"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx := &beforeReadContext{Context: context.Background(), hook: func() {
		if err := os.WriteFile(path, []byte("abcd"), 0600); err != nil {
			t.Fatal(err)
		}
	}}
	result, err := Scan(ctx, path, 10)
	if !errors.Is(err, ErrIdentity) || result.SHA256 != "" || result.ReadBytes != 4 {
		t.Fatal("growth probe lost actual bytes or accepted a changed source")
	}
}

func TestSourceGrowthAndMetadataChangeInvalidateScanAndCopy(t *testing.T) {
	for _, copying := range []bool{false, true} {
		for _, growing := range []bool{false, true} {
			directory := t.TempDir()
			source := filepath.Join(directory, "source")
			if err := os.WriteFile(source, []byte("original fixture contents"), 0600); err != nil {
				t.Fatal(err)
			}
			ctx := &beforeReadContext{Context: context.Background(), hook: func() {
				if growing {
					if err := os.Truncate(source, 100); err != nil {
						t.Fatal(err)
					}
				} else {
					changed := time.Unix(1, 0)
					if err := os.Chtimes(source, changed, changed); err != nil {
						t.Fatal(err)
					}
				}
			}}
			var err error
			if copying {
				_, _, err = ZeroScanCopy(ctx, source, filepath.Join(directory, "disk.img.partial"), 25, 25)
			} else {
				_, err = Scan(ctx, source, 100)
			}
			if !errors.Is(err, ErrIdentity) {
				t.Fatal("changed source identity was accepted")
			}
			if _, err := os.Lstat(filepath.Join(directory, "disk.img")); !os.IsNotExist(err) {
				t.Fatal("changed source was published")
			}
		}
	}
}

func TestCopyRejectsOccupiedOrLinkedTargetWithoutChangingEitherFile(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	victim := filepath.Join(directory, "existing")
	for _, path := range []string{source, victim} {
		if err := os.WriteFile(path, []byte("unchanged fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(directory, "linked-target")
	if err := os.Symlink(victim, link); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{victim, link, source} {
		written, skipped, err := ZeroScanCopy(context.Background(), source, target, 17, 17)
		if err == nil || written != 0 || skipped != 0 {
			t.Fatal("occupied or linked target accepted")
		}
	}
	for _, path := range []string{source, victim} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "unchanged fixture" {
			t.Fatal("rejected copy changed source or existing target")
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 2, syscall.ENOSPC }

func TestWriteBudgetCountsPartialCompletionBeforeIOError(t *testing.T) {
	var completed int64
	if err := writeBounded(failingWriter{}, []byte("data"), &completed, 4); !errors.Is(err, syscall.ENOSPC) || completed != 2 {
		t.Fatal("partial I/O failure lost actual completed bytes")
	}
}

func (w *shortWriter) Write(data []byte) (int, error) {
	if w.zero {
		return 0, nil
	}
	if w.invalid {
		return len(data) + 1, nil
	}
	n := len(data)
	if n > w.max {
		n = w.max
	}
	w.completed += n
	return n, nil
}
func sampleFile(t *testing.T, path string, size int64) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(size); err != nil {
		t.Fatal(err)
	}
	for _, offset := range []int64{4096, size - 4096} {
		if _, err := file.WriteAt([]byte("NON_SECRET_DISK_MARKER"), offset); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

// capability_id: rainbond.vm-restore.copy-write-budget
func TestWriteBudgetHandlesShortWritesAndStopsBeforeOverflow(t *testing.T) {
	writer := &shortWriter{max: 3}
	var completed int64
	if err := writeBounded(writer, []byte("abcdefghij"), &completed, 10); err != nil || completed != 10 || writer.completed != 10 {
		t.Fatal("short writes were not fully counted")
	}
	if err := writeBounded(writer, []byte("x"), &completed, 10); !errors.Is(err, ErrWriteBudget) || writer.completed != 10 {
		t.Fatal("write exceeded hard budget")
	}
	for _, writer := range []*shortWriter{{zero: true}, {invalid: true}} {
		completed = 0
		if err := writeBounded(writer, []byte("data"), &completed, 8); !errors.Is(err, io.ErrShortWrite) {
			t.Fatal("zero or invalid write count was accepted")
		}
	}
}

// capability_id: rainbond.vm-restore.zero-scan-copy
func TestZeroScanCopyPreservesFullContentAndArbitraryLogicalSize(t *testing.T) {
	for _, size := range []int64{2 << 20, 3*ChunkBytes + 127} {
		t.Run(string(rune(size%26+'a')), func(t *testing.T) {
			directory := t.TempDir()
			source := filepath.Join(directory, "source")
			target := filepath.Join(directory, "target")
			sampleFile(t, source, size)
			before, err := Scan(context.Background(), source, size)
			if err != nil {
				t.Fatal(err)
			}
			written, skipped, err := ZeroScanCopy(context.Background(), source, target, size, before.NonzeroChunkBytes)
			if err != nil {
				t.Fatal(err)
			}
			after, err := Scan(context.Background(), target, size)
			if err != nil {
				t.Fatal(err)
			}
			if before.SHA256 != after.SHA256 || after.LogicalBytes != size || written != before.NonzeroChunkBytes || written+skipped != size {
				t.Fatal("zero-scan changed content or logical length")
			}
			sourceAfter, err := Scan(context.Background(), source, size)
			if err != nil || sourceAfter.SHA256 != before.SHA256 {
				t.Fatal("copy changed source")
			}
		})
	}
}

func TestWriteBudgetFailureNeverPublishesOrChangesSource(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	partial := filepath.Join(directory, "disk.img.partial")
	sampleFile(t, source, 3*ChunkBytes)
	before, err := Scan(context.Background(), source, 3*ChunkBytes)
	if err != nil {
		t.Fatal(err)
	}
	written, _, err := ZeroScanCopy(context.Background(), source, partial, before.LogicalBytes, before.NonzeroChunkBytes-1)
	if !errors.Is(err, ErrWriteBudget) || written > before.NonzeroChunkBytes-1 {
		t.Fatal("failed write budget was exceeded")
	}
	after, err := Scan(context.Background(), source, 3*ChunkBytes)
	if err != nil || after.SHA256 != before.SHA256 {
		t.Fatal("failure changed original source")
	}
	if _, err := os.Stat(filepath.Join(directory, "disk.img")); !os.IsNotExist(err) {
		t.Fatal("failed copy was published")
	}
}

func TestScanRejectsSymlinksHardlinksAndLimits(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	sampleFile(t, source, 1<<20)
	if _, err := Scan(context.Background(), source, 100); !errors.Is(err, ErrIdentity) {
		t.Fatal("oversized source was accepted")
	}
	link := filepath.Join(directory, "symlink")
	if err := os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(context.Background(), link, 1<<20); err == nil {
		t.Fatal("symlink input was followed")
	}
	hard := filepath.Join(directory, "hard")
	if err := os.Link(source, hard); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(context.Background(), source, 1<<20); !errors.Is(err, ErrIdentity) {
		t.Fatal("hardlinked source was accepted")
	}
}

func TestScanCountsActualReadBytesAndHeadTailHashes(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "data")
	data := []byte("small plain fixture")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Scan(context.Background(), path, 100)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	expected := hex.EncodeToString(digest[:])
	if result.SHA256 != expected || result.HeadSHA256 != expected || result.TailSHA256 != expected || result.ReadBytes != int64(len(data)) || result.StatBlocksIsAllocation {
		t.Fatal("hash/count/report semantics changed")
	}
}
