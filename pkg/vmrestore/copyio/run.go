package copyio

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

func codeFor(err error) string {
	switch {
	case errors.Is(err, ErrInvalidPlan):
		return "copy_plan_invalid"
	case errors.Is(err, ErrIdentity):
		return "copy_file_identity_changed"
	case errors.Is(err, ErrWriteBudget):
		return "copy_write_budget_exceeded"
	case errors.Is(err, ErrDigest):
		return "copy_full_digest_mismatch"
	case errors.Is(err, context.Canceled):
		return "copy_cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "copy_deadline_exceeded"
	default:
		return "copy_io_error"
	}
}

// Run saves every complete intermediate metadata result before asserting its
// cross-stage correctness. No failure record contains raw paths or guest bytes.
func Run(ctx context.Context, plan Plan, base string, save func(Report) error) (report Report, err error) {
	report = Report{Protocol: Protocol, OperationID: plan.OperationID, PlanHash: plan.PlanHash, Mode: plan.Mode, Phase: "validate", Disks: []DiskResult{}}
	defer func() {
		if err != nil {
			report.OK = false
			report.ErrorCode = codeFor(err)
			var errno syscall.Errno
			if errors.As(err, &errno) {
				report.Errno = int(errno)
			}
			if save != nil {
				if saveErr := save(report); saveErr != nil {
					report.ErrorCode = "copy_result_persistence_failed"
				}
			}
		}
	}()
	if err = plan.Validate(); err != nil {
		return report, err
	}
	if save == nil {
		return report, ErrInvalidPlan
	}
	for index, disk := range plan.Disks {
		sourceDir, previousDir, targetDir := Directories(base, index)
		sourcePath := filepath.Join(sourceDir, "disk.img")
		previousPath := filepath.Join(previousDir, "disk.img")
		partialPath := filepath.Join(targetDir, "disk.img.partial")
		result := DiskResult{VolumeID: disk.VolumeID, SourcePVCUID: disk.SourcePVCUID, PreviousPVCUID: disk.PreviousPVCUID, TargetPVCUID: disk.TargetPVCUID}
		report.Disks = append(report.Disks, result)
		position := len(report.Disks) - 1
		report.Phase = "source-scan"
		report.Disks[position].Source, err = Scan(ctx, sourcePath, disk.SourceMaxBytes)
		if err != nil {
			return report, err
		}
		if err = save(report); err != nil {
			return report, err
		}
		report.Phase = "previous-before"
		report.Disks[position].PreviousBefore, err = Scan(ctx, previousPath, disk.PreviousMaxBytes)
		if err != nil {
			return report, err
		}
		if err = save(report); err != nil {
			return report, err
		}
		if plan.Mode == "inspect" {
			continue
		}
		actual := report.Disks[position]
		if actual.Source.LogicalBytes != disk.ExpectedSourceBytes || actual.Source.SHA256 != disk.ExpectedSourceSHA256 || actual.PreviousBefore.LogicalBytes != disk.ExpectedPreviousBytes || actual.PreviousBefore.SHA256 != disk.ExpectedPreviousSHA256 {
			return report, ErrDigest
		}
		if plan.Mode == "verify" {
			report.Phase = "target-scan"
			report.Disks[position].Target, err = Scan(ctx, filepath.Join(targetDir, "disk.img"), disk.SourceMaxBytes)
			if err != nil {
				return report, err
			}
			if err = save(report); err != nil {
				return report, err
			}
			report.Phase = "previous-after"
			report.Disks[position].PreviousAfter, err = Scan(ctx, previousPath, disk.PreviousMaxBytes)
			if err != nil {
				return report, err
			}
			if err = save(report); err != nil {
				return report, err
			}
			actual = report.Disks[position]
			if actual.Target.SHA256 != disk.ExpectedSourceSHA256 || actual.Target.LogicalBytes != disk.ExpectedSourceBytes || actual.PreviousAfter.SHA256 != disk.ExpectedPreviousSHA256 || actual.Target.UID != plan.TargetUID || actual.Target.GID != plan.TargetGID || actual.Target.Mode != 0660 {
				return report, ErrDigest
			}
			report.Disks[position].Published = true
			if err = save(report); err != nil {
				return report, err
			}
			continue
		}
		entries, readErr := os.ReadDir(targetDir)
		if readErr != nil {
			return report, readErr
		}
		if len(entries) != 0 {
			return report, ErrIdentity
		}
		report.Phase = "copy-stream"
		report.Disks[position].WrittenBytes, report.Disks[position].SkippedZeroBytes, err = ZeroScanCopy(ctx, sourcePath, partialPath, actual.Source.LogicalBytes, disk.WriteLimitBytes)
		if saveErr := save(report); saveErr != nil {
			return report, saveErr
		}
		if err != nil {
			return report, err
		}
		report.Phase = "target-scan"
		report.Disks[position].Target, err = Scan(ctx, partialPath, disk.SourceMaxBytes)
		if err != nil {
			return report, err
		}
		if err = save(report); err != nil {
			return report, err
		}
		report.Phase = "previous-after"
		report.Disks[position].PreviousAfter, err = Scan(ctx, previousPath, disk.PreviousMaxBytes)
		if err != nil {
			return report, err
		}
		if err = save(report); err != nil {
			return report, err
		}
		actual = report.Disks[position]
		if actual.Source.LogicalBytes != actual.Target.LogicalBytes || actual.Source.SHA256 != actual.Target.SHA256 || actual.PreviousBefore.LogicalBytes != actual.PreviousAfter.LogicalBytes || actual.PreviousBefore.SHA256 != actual.PreviousAfter.SHA256 {
			return report, ErrDigest
		}
		report.Phase = "publish-target"
		if err = ctx.Err(); err != nil {
			return report, err
		}
		// Newly created target only. Source ROX and original B never receive chmod/chown.
		file, partialInfo, openErr := openRegular(partialPath)
		if openErr != nil {
			return report, openErr
		}
		syncErr := file.Chmod(0660)
		if syncErr == nil {
			syncErr = file.Chown(int(plan.TargetUID), int(plan.TargetGID))
		}
		if syncErr == nil {
			syncErr = file.Sync()
		}
		closeErr := file.Close()
		if syncErr != nil {
			return report, syncErr
		}
		if closeErr != nil {
			return report, closeErr
		}
		if err = ctx.Err(); err != nil {
			return report, err
		}
		currentInfo, statErr := os.Lstat(partialPath)
		if statErr != nil {
			return report, statErr
		}
		if !currentInfo.Mode().IsRegular() || !os.SameFile(partialInfo, currentInfo) {
			return report, ErrIdentity
		}
		targetPath := filepath.Join(targetDir, "disk.img")
		// The Job exclusively owns this target directory. This check protects
		// collisions; it is not a concurrent-writer no-replace rename primitive.
		if _, statErr := os.Lstat(targetPath); statErr == nil {
			return report, ErrIdentity
		} else if !os.IsNotExist(statErr) {
			return report, statErr
		}
		if err = os.Rename(partialPath, targetPath); err != nil {
			return report, err
		}
		if err = syncDirectory(targetDir); err != nil {
			return report, err
		}
		report.Phase = "target-scan"
		report.Disks[position].Target, err = Scan(ctx, filepath.Join(targetDir, "disk.img"), disk.SourceMaxBytes)
		if err != nil {
			return report, err
		}
		if err = save(report); err != nil {
			return report, err
		}
		if report.Disks[position].Target.SHA256 != actual.Source.SHA256 {
			return report, ErrDigest
		}
		report.Disks[position].Published = true
		if err = save(report); err != nil {
			return report, err
		}
	}
	report.Phase = "complete"
	report.OK = true
	if err = save(report); err != nil {
		return report, err
	}
	return report, nil
}
