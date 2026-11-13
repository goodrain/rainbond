package restoreplan

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"regexp"
	"strings"

	"github.com/goodrain/rainbond/pkg/vmrestore/copyio"
)

var (
	// ErrResultIdentity prevents a different/recreated Job, Pod or tool from supplying results.
	ErrResultIdentity = errors.New("restore_result_identity_conflict")
	// ErrResultIncomplete prevents truncated logs or partial/unknown JSON from counting as success.
	ErrResultIncomplete = errors.New("restore_result_incomplete")
)

// ResultIdentity must be checked before reading logs, then checked again here.
type ResultIdentity struct {
	Namespace    string `json:"namespace"`
	JobName      string `json:"job_name"`
	JobUID       string `json:"job_uid"`
	PodName      string `json:"pod_name"`
	PodUID       string `json:"pod_uid"`
	OwnerJobUID  string `json:"owner_job_uid"`
	ImageID      string `json:"image_id"`
	FrameHash    string `json:"frame_hash"`
	ExitCode     int    `json:"exit_code"`
	LogsComplete bool   `json:"logs_complete"`
}

// ResultExpectation is generated from the persisted operation resource receipt.
type ResultExpectation struct {
	Identity ResultIdentity
	Plan     copyio.Plan
}

// CopyEvidence is persisted in Region state before cross-stage/digest assertions.
// Raw logs and arbitrary exception strings are intentionally not stored.
type CopyEvidence struct {
	Identity  ResultIdentity  `json:"identity"`
	LogBytes  int             `json:"log_bytes"`
	LogSHA256 string          `json:"log_sha256"`
	ParseCode string          `json:"parse_code,omitempty"`
	Report    *copyio.Report  `json:"report,omitempty"`
	Summary   *copyio.Summary `json:"summary,omitempty"`
}

func decode(data []byte, target interface{}) error {
	// Struct decoding otherwise silently keeps the last duplicate member. Check
	// decoded names before projecting evidence so contradictory values survive
	// neither as a successful result nor as a misleading normalized report.
	members := json.NewDecoder(bytes.NewReader(data))
	members.UseNumber()
	schema := reflect.TypeOf(target)
	if schema == nil || schema.Kind() != reflect.Ptr {
		return ErrResultIncomplete
	}
	if err := uniqueJSONValue(members, schema.Elem()); err != nil {
		return ErrResultIncomplete
	}
	if _, err := members.Token(); err != io.EOF {
		return ErrResultIncomplete
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return ErrResultIncomplete
	}
	if err := decoder.Decode(new(interface{})); err != io.EOF {
		return ErrResultIncomplete
	}
	return nil
}

func uniqueJSONValue(decoder *json.Decoder, schema reflect.Type) error {
	token, err := decoder.Token()
	if err != nil {
		return ErrResultIncomplete
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		return nil
	}
	var end json.Delim
	switch delimiter {
	case '{':
		if schema.Kind() != reflect.Struct {
			return ErrResultIncomplete
		}
		end = '}'
		fields := make(map[string]reflect.Type)
		for i := 0; i < schema.NumField(); i++ {
			field := schema.Field(i)
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if field.IsExported() && name != "" && name != "-" {
				fields[name] = field.Type
			}
		}
		seen := make(map[string]bool)
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return ErrResultIncomplete
			}
			name, ok := key.(string)
			field, canonical := fields[name]
			// The wire protocol uses exact JSON tag names. Reject every alias
			// (including Unicode folds) before encoding/json can assign it to the
			// same struct field. Escapes decoding to a canonical name remain legal.
			if !ok || !canonical || seen[name] {
				return ErrResultIncomplete
			}
			seen[name] = true
			if err := uniqueJSONValue(decoder, field); err != nil {
				return err
			}
		}
	case '[':
		if schema.Kind() != reflect.Slice {
			return ErrResultIncomplete
		}
		end = ']'
		for decoder.More() {
			if err := uniqueJSONValue(decoder, schema.Elem()); err != nil {
				return err
			}
		}
	default:
		return ErrResultIncomplete
	}
	if token, err := decoder.Token(); err != nil || token != end {
		return ErrResultIncomplete
	}
	return nil
}

func identityMatches(a, b ResultIdentity) bool {
	return a.Namespace == b.Namespace && a.JobName == b.JobName && a.JobUID == b.JobUID && a.PodName == b.PodName && a.PodUID == b.PodUID && a.OwnerJobUID == b.JobUID && a.ImageID == b.ImageID && a.FrameHash == b.FrameHash && a.PodUID != "" && a.JobUID != ""
}

var safeID = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
var safeUID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)

func safeCopyErrorCode(code string) bool {
	switch code {
	case "", "copy_plan_invalid", "copy_file_identity_changed", "copy_write_budget_exceeded", "copy_full_digest_mismatch", "copy_cancelled", "copy_deadline_exceeded", "copy_io_error", "copy_result_persistence_failed":
		return true
	default:
		return false
	}
}

func safeReportShape(report copyio.Report) bool {
	phases := map[string]bool{"validate": true, "source-scan": true, "previous-before": true, "copy-stream": true, "target-scan": true, "previous-after": true, "publish-target": true, "complete": true}
	if report.Protocol != copyio.Protocol || !phases[report.Phase] || !safeCopyErrorCode(report.ErrorCode) || len(report.Disks) > 128 || !safeID.MatchString(report.OperationID) || !validSHA(report.PlanHash) || (report.Mode != "inspect" && report.Mode != "copy" && report.Mode != "verify") || report.Errno < 0 || report.Errno > 4095 {
		return false
	}
	for _, disk := range report.Disks {
		for _, uid := range []string{disk.SourcePVCUID, disk.PreviousPVCUID, disk.TargetPVCUID} {
			if uid != "" && !safeUID.MatchString(uid) {
				return false
			}
		}
		for _, file := range []copyio.FileResult{disk.Source, disk.PreviousBefore, disk.PreviousAfter, disk.Target} {
			for _, hash := range []string{file.SHA256, file.HeadSHA256, file.TailSHA256} {
				if hash != "" && !validSHA(hash) {
					return false
				}
			}
		}
	}
	return true
}

// CaptureAndValidate requires bounded complete GetLogs, a matching termination
// summary and exact server-tracked identities. Evidence is saved before assertions.
func CaptureAndValidate(log, termination []byte, observed ResultIdentity, expected ResultExpectation, save func(CopyEvidence) error) (copyio.Report, error) {
	evidence := CopyEvidence{Identity: observed, LogBytes: len(log)}
	digest := sha256.Sum256(log)
	evidence.LogSHA256 = hex.EncodeToString(digest[:])
	persistFailure := func(code string, err error) (copyio.Report, error) {
		evidence.ParseCode = code
		if save == nil {
			return copyio.Report{}, ErrResultIncomplete
		}
		if saveErr := save(evidence); saveErr != nil {
			return copyio.Report{}, saveErr
		}
		return copyio.Report{}, err
	}
	if !identityMatches(observed, expected.Identity) {
		return persistFailure("identity_conflict", ErrResultIdentity)
	}
	if !observed.LogsComplete || len(log) > copyio.MaxReportBytes+1 || len(termination) > 3500 {
		return persistFailure("truncated_or_oversize", ErrResultIncomplete)
	}
	payload := bytes.TrimSuffix(log, []byte{'\n'})
	var report copyio.Report
	var summary copyio.Summary
	if decode(payload, &report) != nil || decode(termination, &summary) != nil {
		return persistFailure("json_incomplete", ErrResultIncomplete)
	}
	// Structural projection only, before any cross-stage correctness decision.
	if !safeReportShape(report) || !safeID.MatchString(summary.OperationID) || !validSHA(summary.PlanHash) || !validSHA(summary.ReportSHA256) || !safeCopyErrorCode(summary.ErrorCode) {
		return persistFailure("schema_invalid", ErrResultIncomplete)
	}
	evidence.Report = &report
	evidence.Summary = &summary
	if save == nil {
		return copyio.Report{}, ErrResultIncomplete
	}
	if err := save(evidence); err != nil {
		return copyio.Report{}, err
	}
	actual := sha256.Sum256(payload)
	if expected.Plan.Validate() != nil || expected.Plan.FrameHash != expected.Identity.FrameHash || summary.Protocol != copyio.Protocol || summary.OperationID != expected.Plan.OperationID || summary.PlanHash != expected.Plan.PlanHash || summary.ReportSHA256 != hex.EncodeToString(actual[:]) || summary.ReportBytes != len(payload) || summary.DiskCount != len(report.Disks) || summary.ErrorCode != report.ErrorCode || report.OperationID != expected.Plan.OperationID || report.PlanHash != expected.Plan.PlanHash || report.Mode != expected.Plan.Mode || len(report.Disks) != len(expected.Plan.Disks) {
		return report, ErrResultIdentity
	}
	if observed.ExitCode != 0 || !summary.OK || !report.OK || report.Phase != "complete" || report.ErrorCode != "" || report.Errno != 0 {
		return report, ErrResultIncomplete
	}
	for index, disk := range report.Disks {
		plan := expected.Plan.Disks[index]
		if disk.VolumeID != plan.VolumeID || disk.SourcePVCUID != plan.SourcePVCUID || disk.PreviousPVCUID != plan.PreviousPVCUID || disk.TargetPVCUID != plan.TargetPVCUID || disk.Source.LogicalBytes < 1 || disk.Source.LogicalBytes > plan.SourceMaxBytes || disk.Source.ReadBytes != disk.Source.LogicalBytes || !validSHA(disk.Source.SHA256) || disk.PreviousBefore.LogicalBytes < 1 || disk.PreviousBefore.LogicalBytes > plan.PreviousMaxBytes || disk.PreviousBefore.ReadBytes != disk.PreviousBefore.LogicalBytes || !validSHA(disk.PreviousBefore.SHA256) {
			return report, ErrResultIdentity
		}
		if expected.Plan.Mode == "inspect" && (disk.WrittenBytes != 0 || disk.Published || disk.Target.LogicalBytes != 0 || disk.Target.ReadBytes != 0 || disk.Target.SHA256 != "") {
			return report, ErrResultIdentity
		}
		if (expected.Plan.Mode == "copy" || expected.Plan.Mode == "verify") && (disk.Source.LogicalBytes != plan.ExpectedSourceBytes || disk.Source.SHA256 != plan.ExpectedSourceSHA256 || disk.PreviousBefore.LogicalBytes != plan.ExpectedPreviousBytes || disk.PreviousBefore.SHA256 != plan.ExpectedPreviousSHA256 || disk.Source.SHA256 != disk.Target.SHA256 || disk.Target.LogicalBytes != disk.Source.LogicalBytes || disk.Target.ReadBytes != disk.Target.LogicalBytes || disk.PreviousBefore.SHA256 != disk.PreviousAfter.SHA256 || disk.PreviousBefore.LogicalBytes != disk.PreviousAfter.LogicalBytes || disk.PreviousAfter.ReadBytes != disk.PreviousAfter.LogicalBytes || disk.WrittenBytes < 0 || disk.WrittenBytes > plan.WriteLimitBytes || disk.Target.UID != expected.Plan.TargetUID || disk.Target.GID != expected.Plan.TargetGID || disk.Target.Mode != 0660 || !disk.Published) {
			return report, copyio.ErrDigest
		}
	}
	return report, nil
}
