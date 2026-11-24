package restoreplan

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/goodrain/rainbond/pkg/vmrestore/copyio"
)

// capability_id: rainbond.vm-restore.copy-result-unique-members
func TestCaptureRejectsDuplicateJSONMembers(t *testing.T) {
	for _, kind := range []string{"report-prefix", "report-suffix", "same-value", "escaped-report", "nested-read", "escaped-nested", "nested-object", "summary", "escaped-summary"} {
		t.Run(kind, func(t *testing.T) {
			expected, _, log, end := captureFixture()
			payload := bytes.TrimSuffix(log, []byte{'\n'})
			switch kind {
			case "report-prefix":
				payload = append([]byte(`{"ok":false,`), payload[1:]...)
			case "report-suffix":
				payload = append(payload[:len(payload)-1], []byte(`,"ok":true}`)...)
			case "same-value":
				payload = append([]byte(`{"ok":true,`), payload[1:]...)
			case "escaped-report":
				payload = append([]byte(`{"o\u006b":false,`), payload[1:]...)
			case "nested-read":
				payload = bytes.Replace(payload, []byte(`"source":{`), []byte(`"source":{"read_bytes":0,`), 1)
			case "escaped-nested":
				payload = bytes.Replace(payload, []byte(`"source":{`), []byte(`"source":{"read_\u0062ytes":0,`), 1)
			case "nested-object":
				payload = bytes.Replace(payload, []byte(`"source":{`), []byte(`"source":{},"source":{`), 1)
			case "summary":
				end = append([]byte(`{"ok":false,`), end[1:]...)
			case "escaped-summary":
				end = append([]byte(`{"error_\u0063ode":"untrusted-detail","error_code":"",`), end[1:]...)
			}
			// Even a correctly checksummed envelope cannot legitimize ambiguous JSON.
			if kind != "summary" && kind != "escaped-summary" {
				var summary copyio.Summary
				if err := json.Unmarshal(end, &summary); err != nil {
					t.Fatal(err)
				}
				digest := sha256.Sum256(payload)
				summary.ReportBytes = len(payload)
				summary.ReportSHA256 = hex.EncodeToString(digest[:])
				end, _ = json.Marshal(summary)
			}
			var saved CopyEvidence
			calls := 0
			report, err := CaptureAndValidate(payload, end, expected.Identity, expected, func(e CopyEvidence) error { saved = e; calls++; return nil })
			digest := sha256.Sum256(payload)
			if !errors.Is(err, ErrResultIncomplete) || report.OK || calls != 1 || saved.ParseCode != "json_incomplete" || saved.Report != nil || saved.Summary != nil || saved.LogBytes != len(payload) || saved.LogSHA256 != hex.EncodeToString(digest[:]) {
				t.Fatalf("ambiguous JSON became success or lost safe failure evidence: err=%v calls=%d evidence=%+v", err, calls, saved)
			}
		})
	}
}

func TestCaptureAllowsDistinctObjectsAndEscapedUniqueMembers(t *testing.T) {
	expected, report, _, _ := captureFixture()
	disk := expected.Plan.Disks[0]
	disk.VolumeID++
	disk.SourcePVCUID, disk.PreviousPVCUID, disk.TargetPVCUID = "rox-2", "b-2", "a-2"
	expected.Plan.Disks = append(expected.Plan.Disks, disk)
	var err error
	expected.Plan, err = copyio.SealPlan(expected.Plan)
	if err != nil {
		t.Fatal(err)
	}
	result := report.Disks[0]
	result.VolumeID, result.SourcePVCUID, result.PreviousPVCUID, result.TargetPVCUID = disk.VolumeID, disk.SourcePVCUID, disk.PreviousPVCUID, disk.TargetPVCUID
	report.Disks = append(report.Disks, result)
	report.PlanHash = expected.Plan.PlanHash
	log, _ := captureWire(report)
	payload := bytes.TrimSuffix(log, []byte{'\n'})
	// Reorder a unique member and encode its name with a JSON escape.
	payload = bytes.Replace(payload, []byte(`"ok":true,`), nil, 1)
	payload = append([]byte(`{ "o\u006b" : true, `), payload[1:]...)
	digest := sha256.Sum256(payload)
	end, _ := json.Marshal(copyio.Summary{Protocol: copyio.Protocol, OperationID: report.OperationID, PlanHash: report.PlanHash, OK: true, ReportBytes: len(payload), ReportSHA256: hex.EncodeToString(digest[:]), DiskCount: 2})
	var saved CopyEvidence
	actual, err := CaptureAndValidate(payload, end, expected.Identity, expected, func(e CopyEvidence) error { saved = e; return nil })
	if err != nil || !actual.OK || saved.Report == nil || len(saved.Report.Disks) != 2 || saved.Summary == nil {
		t.Fatalf("unique members in separate objects/array entries were rejected: %v", err)
	}
}

// capability_id: rainbond.vm-restore.copy-result-canonical-fields
func TestCaptureRejectsNoncanonicalTypedFieldAliases(t *testing.T) {
	for _, kind := range []string{"uppercase-report", "unicode-report", "uppercase-summary", "uppercase-nested", "unicode-nested", "escaped-uppercase"} {
		t.Run(kind, func(t *testing.T) {
			expected, _, log, end := captureFixture()
			payload := bytes.TrimSuffix(log, []byte{'\n'})
			switch kind {
			case "uppercase-report":
				payload = bytes.Replace(payload, []byte(`"ok":`), []byte(`"OK":`), 1)
			case "unicode-report":
				payload = bytes.Replace(payload, []byte(`"ok":`), []byte(`"o\u212a":`), 1)
			case "uppercase-summary":
				end = bytes.Replace(end, []byte(`"ok":`), []byte(`"OK":`), 1)
			case "uppercase-nested":
				payload = bytes.Replace(payload, []byte(`"read_bytes":`), []byte(`"READ_BYTES":`), 1)
			case "unicode-nested":
				payload = bytes.Replace(payload, []byte(`"source":`), []byte(`"\u017fource":`), 1)
			case "escaped-uppercase":
				payload = bytes.Replace(payload, []byte(`"ok":`), []byte(`"\u004f\u004b":`), 1)
			}
			if kind != "uppercase-summary" {
				var summary copyio.Summary
				if err := json.Unmarshal(end, &summary); err != nil {
					t.Fatal(err)
				}
				digest := sha256.Sum256(payload)
				summary.ReportBytes, summary.ReportSHA256 = len(payload), hex.EncodeToString(digest[:])
				end, _ = json.Marshal(summary)
			}
			var saved CopyEvidence
			calls := 0
			result, err := CaptureAndValidate(payload, end, expected.Identity, expected, func(e CopyEvidence) error { saved = e; calls++; return nil })
			if !errors.Is(err, ErrResultIncomplete) || result.OK || calls != 1 || saved.ParseCode != "json_incomplete" || saved.Report != nil || saved.Summary != nil {
				t.Fatalf("noncanonical typed alias became accepted evidence: %v", err)
			}
		})
	}
}

func captureFixture() (ResultExpectation, copyio.Report, []byte, []byte) {
	identity := ResultIdentity{Namespace: "team", JobName: "restore-job", JobUID: "job-uid", PodName: "copy-pod", PodUID: "pod-uid", OwnerJobUID: "job-uid", ImageID: "pinned-tool-manifest", FrameHash: strings.Repeat("f", 64), LogsComplete: true}
	file := copyio.FileResult{LogicalBytes: 4096, ReadBytes: 4096, SHA256: strings.Repeat("a", 64), NonzeroChunkBytes: 4096, Mode: 0660}
	plan := copyio.Plan{Protocol: copyio.Protocol, OperationID: "operation-a", ManifestHash: strings.Repeat("b", 64), FrameHash: identity.FrameHash, Mode: "copy", DeadlineSeconds: 60, Disks: []copyio.DiskPlan{{VolumeID: 7, SourcePVCUID: "rox", PreviousPVCUID: "b", TargetPVCUID: "a", SourceMaxBytes: 4096, PreviousMaxBytes: 4096, ExpectedSourceBytes: 4096, ExpectedSourceSHA256: file.SHA256, ExpectedPreviousBytes: 4096, ExpectedPreviousSHA256: file.SHA256, WriteLimitBytes: 4096}}}
	plan, _ = copyio.SealPlan(plan)
	report := copyio.Report{Protocol: copyio.Protocol, OperationID: plan.OperationID, PlanHash: plan.PlanHash, Mode: "copy", OK: true, Phase: "complete", Disks: []copyio.DiskResult{{VolumeID: 7, SourcePVCUID: "rox", PreviousPVCUID: "b", TargetPVCUID: "a", Source: file, Target: file, PreviousBefore: file, PreviousAfter: file, WrittenBytes: 4096, Published: true}}}
	payload, _ := json.Marshal(report)
	sum := sha256.Sum256(payload)
	summary := copyio.Summary{Protocol: copyio.Protocol, OperationID: plan.OperationID, PlanHash: plan.PlanHash, OK: true, ReportBytes: len(payload), ReportSHA256: hex.EncodeToString(sum[:]), DiskCount: 1}
	termination, _ := json.Marshal(summary)
	return ResultExpectation{Identity: identity, Plan: plan}, report, append(payload, '\n'), termination
}

// capability_id: rainbond.vm-restore.copy-result-capture
func TestCapturePersistsFullTypedFailureBeforeCrossStageAssertion(t *testing.T) {
	expected, report, _, _ := captureFixture()
	report.Disks[0].Target.SHA256 = strings.Repeat("c", 64)
	payload, _ := json.Marshal(report)
	sum := sha256.Sum256(payload)
	summary := copyio.Summary{Protocol: copyio.Protocol, OperationID: expected.Plan.OperationID, PlanHash: expected.Plan.PlanHash, OK: true, ReportBytes: len(payload), ReportSHA256: hex.EncodeToString(sum[:]), DiskCount: 1}
	termination, _ := json.Marshal(summary)
	saved := []CopyEvidence{}
	_, err := CaptureAndValidate(append(payload, '\n'), termination, expected.Identity, expected, func(e CopyEvidence) error { saved = append(saved, e); return nil })
	if !errors.Is(err, copyio.ErrDigest) || len(saved) != 1 || saved[0].Report == nil || saved[0].Report.Disks[0].Target.SHA256 != strings.Repeat("c", 64) {
		t.Fatal("digest mismatch lost successful worker payload before assertion")
	}
}
func TestCaptureRejectsReplacementImageAndTruncation(t *testing.T) {
	expected, _, payload, termination := captureFixture()
	for _, kind := range []string{"pod", "job", "image", "truncated", "summary"} {
		t.Run(kind, func(t *testing.T) {
			actual := expected.Identity
			data := payload
			message := termination
			switch kind {
			case "pod":
				actual.PodUID = "replacement"
			case "job":
				actual.OwnerJobUID = "different"
			case "image":
				actual.ImageID = "other-image"
			case "truncated":
				actual.LogsComplete = false
			case "summary":
				message = []byte(`{}`)
			}
			saved := false
			if _, err := CaptureAndValidate(data, message, actual, expected, func(CopyEvidence) error { saved = true; return nil }); err == nil || !saved {
				t.Fatal("incomplete or untrusted result became success/was not recorded")
			}
		})
	}
}

func TestValidCaptureRequiresExactFullJSONAndAllDiskResults(t *testing.T) {
	expected, _, payload, termination := captureFixture()
	if _, err := CaptureAndValidate(payload, termination, expected.Identity, expected, func(CopyEvidence) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := CaptureAndValidate(append(payload, []byte(`{}`)...), termination, expected.Identity, expected, func(CopyEvidence) error { return nil }); err == nil {
		t.Fatal("multiple/truncated JSON was accepted")
	}
}

func captureWire(report copyio.Report) ([]byte, []byte) {
	payload, _ := json.Marshal(report)
	hash := sha256.Sum256(payload)
	summary := copyio.Summary{Protocol: copyio.Protocol, OperationID: report.OperationID, PlanHash: report.PlanHash,
		OK: report.OK, ReportBytes: len(payload), ReportSHA256: hex.EncodeToString(hash[:]), DiskCount: len(report.Disks), ErrorCode: report.ErrorCode}
	termination, _ := json.Marshal(summary)
	return append(payload, '\n'), termination
}

func TestCaptureRejectsUnsafeSummaryAndContradictorySuccess(t *testing.T) {
	expected, report, payload, termination := captureFixture()
	var summary copyio.Summary
	if err := json.Unmarshal(termination, &summary); err != nil {
		t.Fatal(err)
	}
	summary.ErrorCode = "untrusted-detail"
	unsafe, _ := json.Marshal(summary)
	var saved CopyEvidence
	_, err := CaptureAndValidate(payload, unsafe, expected.Identity, expected, func(e CopyEvidence) error { saved = e; return nil })
	if !errors.Is(err, ErrResultIncomplete) || saved.Summary != nil {
		t.Error("arbitrary termination detail was persisted or accepted")
	}
	report.ErrorCode = "copy_io_error"
	report.Errno = 5
	payload, termination = captureWire(report)
	_, err = CaptureAndValidate(payload, termination, expected.Identity, expected, func(e CopyEvidence) error { saved = e; return nil })
	if err == nil || saved.Report == nil || saved.Report.ErrorCode != "copy_io_error" {
		t.Fatal("contradictory success was accepted or safe evidence lost")
	}
}

func TestCaptureBindsPlanFrameToExpectedJobAndChecksInspectShape(t *testing.T) {
	expected, report, _, _ := captureFixture()
	expected.Identity.FrameHash = strings.Repeat("e", 64)
	payload, termination := captureWire(report)
	if _, err := CaptureAndValidate(payload, termination, expected.Identity, expected, func(CopyEvidence) error { return nil }); !errors.Is(err, ErrResultIdentity) {
		t.Error("job identity frame and plan frame were allowed to disagree")
	}
	expected, report, _, _ = captureFixture()
	expected.Plan.Mode = "inspect"
	var err error
	expected.Plan, err = copyio.SealPlan(expected.Plan)
	if err != nil {
		t.Fatal(err)
	}
	report.Mode, report.PlanHash = "inspect", expected.Plan.PlanHash
	report.Disks[0].WrittenBytes = 0
	report.Disks[0].Published = false
	report.Disks[0].Target = copyio.FileResult{}
	payload, termination = captureWire(report)
	if _, err := CaptureAndValidate(payload, termination, expected.Identity, expected, func(CopyEvidence) error { return nil }); err != nil {
		t.Fatal("valid readonly inspect control was rejected")
	}
	for _, kind := range []string{"empty-previous", "oversize-source", "write", "publish"} {
		t.Run(kind, func(t *testing.T) {
			bad := report
			bad.Disks = append([]copyio.DiskResult(nil), report.Disks...)
			switch kind {
			case "empty-previous":
				bad.Disks[0].PreviousBefore = copyio.FileResult{}
			case "oversize-source":
				bad.Disks[0].Source.LogicalBytes++
				bad.Disks[0].Source.ReadBytes++
			case "write":
				bad.Disks[0].WrittenBytes = 1
			case "publish":
				bad.Disks[0].Published = true
			}
			log, end := captureWire(bad)
			if _, err := CaptureAndValidate(log, end, expected.Identity, expected, func(CopyEvidence) error { return nil }); err == nil {
				t.Fatal("invalid inspect report was accepted")
			}
		})
	}
}

func TestCaptureSaveFailureAndWorkerExitNeverBecomeSuccess(t *testing.T) {
	expected, _, log, termination := captureFixture()
	saveError := errors.New("evidence store unavailable")
	if _, err := CaptureAndValidate(log, termination, expected.Identity, expected, func(CopyEvidence) error { return saveError }); !errors.Is(err, saveError) {
		t.Fatal("evidence persistence failure was discarded")
	}
	if _, err := CaptureAndValidate(log, termination, expected.Identity, expected, nil); !errors.Is(err, ErrResultIncomplete) {
		t.Fatal("result was accepted without durable evidence sink")
	}
	observed := expected.Identity
	observed.ExitCode = 1
	saved := false
	_, err := CaptureAndValidate(log, termination, observed, expected, func(CopyEvidence) error { saved = true; return nil })
	if !saved || !errors.Is(err, ErrResultIncomplete) {
		t.Fatal("nonzero worker exit was accepted or lost evidence")
	}
}
