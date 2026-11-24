// Package restoreplan performs pure restore resource and capacity planning.
// All capability/space inputs come from trusted Region providers, never UI claims.
package restoreplan

import (
	"errors"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/goodrain/rainbond/pkg/vmrestore/copyio"
)

var (
	// ErrCapability keeps execution disabled until the exact storage/tool path is certified.
	ErrCapability = errors.New("restore_storage_capability_unverified")
	// ErrCapacity means fresh, reserved physical or logical space is insufficient.
	ErrCapacity   = errors.New("restore_capacity_insufficient")
	image         = regexp.MustCompile(`^[a-zA-Z0-9./:_-]+@sha256:[a-f0-9]{64}$`)
	operationName = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
)

// Capability is privileged server-owned certification bound to real class/driver/tool identities.
type Capability struct {
	StorageClassName          string
	StorageClassUID           string
	Driver                    string
	DriverVersion             string
	Mounter                   string
	ROXVerified               bool
	SparsePersistenceVerified bool
	Protocol                  int
	WorkerImage               string
	RawCopies                 uint64
	AllocationNumerator       uint64
	AllocationDenominator     uint64
	MetadataReserveBytes      uint64
	LogicalPaddingBytes       uint64
	TargetUID                 uint32
	TargetGID                 uint32
	MaxLogicalBytes           uint64
}

// SpaceEvidence is a fresh storage-provider observation, not CephFS st_blocks/statfs quota.
type SpaceEvidence struct {
	StorageClassUID        string
	Source                 string
	ObservedAt             time.Time
	ValidFor               time.Duration
	PoolUsableBytes        uint64
	HostFreeBytes          uint64
	HostFloorBytes         uint64
	ReservedPoolBytes      uint64
	ReservedHostBytes      uint64
	ReservationOperationID string
	ReservationHeld        bool
}

// DiskInput combines an exact readonly snapshot inspect result with its resource quota.
type DiskInput struct {
	VolumeID             uint
	BackupRequestBytes   uint64
	SnapshotRestoreBytes uint64
	Source               copyio.FileResult
	Previous             copyio.FileResult
}

// DiskBudget is passed to the blank target builder and bounded copy plan.
type DiskBudget struct {
	VolumeID        uint
	PVCRequestBytes uint64
	WriteLimitBytes uint64
	SourceBytes     uint64
	PreviousBytes   uint64
	SourceSHA256    string
	PreviousSHA256  string
}

// Budget is a checked reservation assessment; it is not a claim that copying has succeeded.
type Budget struct {
	WorkerImage       string
	PoolRequiredBytes uint64
	HostRequiredBytes uint64
	Disks             []DiskBudget
	TargetUID         uint32
	TargetGID         uint32
}

func validSHA(value string) bool {
	matched, _ := regexp.MatchString(`^[a-f0-9]{64}$`, value)
	return matched
}
func add(a, b uint64) (uint64, error) {
	if a > math.MaxUint64-b {
		return 0, ErrCapacity
	}
	return a + b, nil
}
func mul(a, b uint64) (uint64, error) {
	if b > 0 && a > math.MaxUint64/b {
		return 0, ErrCapacity
	}
	return a * b, nil
}

// Assess never substitutes reported stat blocks or subvolume quota for physical evidence.
func Assess(now time.Time, operationID string, cap Capability, space SpaceEvidence, disks []DiskInput) (Budget, error) {
	result := Budget{WorkerImage: cap.WorkerImage, TargetUID: cap.TargetUID, TargetGID: cap.TargetGID, Disks: []DiskBudget{}}
	if !cap.ROXVerified || !cap.SparsePersistenceVerified || cap.Protocol != copyio.Protocol || (cap.Driver != "cephfs.csi.ceph.com" && !strings.HasSuffix(cap.Driver, ".cephfs.csi.ceph.com")) || cap.StorageClassUID == "" || cap.StorageClassName == "" || cap.DriverVersion == "" || cap.Mounter == "" || !image.MatchString(cap.WorkerImage) || cap.RawCopies < 1 || cap.AllocationNumerator < cap.AllocationDenominator || cap.AllocationDenominator == 0 || cap.MetadataReserveBytes == 0 || cap.MaxLogicalBytes == 0 {
		return result, ErrCapability
	}
	if !operationName.MatchString(operationID) || !space.ReservationHeld || space.ReservationOperationID != operationID || space.StorageClassUID != cap.StorageClassUID || space.Source != "ceph-pool-and-host-verified" || space.ValidFor <= 0 || now.Before(space.ObservedAt) || now.Sub(space.ObservedAt) > space.ValidFor || len(disks) == 0 {
		return result, ErrCapacity
	}
	totalWrites := uint64(0)
	seen := map[uint]bool{}
	for _, disk := range disks {
		if !validSHA(disk.Source.SHA256) || !validSHA(disk.Previous.SHA256) || disk.VolumeID == 0 || seen[disk.VolumeID] || disk.Source.LogicalBytes < 1 || disk.Previous.LogicalBytes < 1 || disk.Source.ReadBytes != disk.Source.LogicalBytes || disk.Previous.ReadBytes != disk.Previous.LogicalBytes || disk.Source.NonzeroChunkBytes < 0 || disk.Source.NonzeroChunkBytes > disk.Source.LogicalBytes || uint64(disk.Source.LogicalBytes) > cap.MaxLogicalBytes || uint64(disk.Previous.LogicalBytes) > cap.MaxLogicalBytes {
			return result, ErrCapacity
		}
		seen[disk.VolumeID] = true
		bytes := uint64(disk.Source.LogicalBytes)
		request, err := add(bytes, cap.LogicalPaddingBytes)
		if err != nil {
			return result, err
		}
		if request < disk.BackupRequestBytes {
			request = disk.BackupRequestBytes
		}
		if request < disk.SnapshotRestoreBytes {
			request = disk.SnapshotRestoreBytes
		}
		if request > math.MaxInt64 {
			return result, ErrCapacity
		}
		writes := uint64(disk.Source.NonzeroChunkBytes)
		totalWrites, err = add(totalWrites, writes)
		if err != nil {
			return result, err
		}
		result.Disks = append(result.Disks, DiskBudget{VolumeID: disk.VolumeID, PVCRequestBytes: request, WriteLimitBytes: writes, SourceBytes: bytes, PreviousBytes: uint64(disk.Previous.LogicalBytes), SourceSHA256: disk.Source.SHA256, PreviousSHA256: disk.Previous.SHA256})
	}
	amplified, err := mul(totalWrites, cap.AllocationNumerator)
	if err != nil {
		return result, err
	}
	amplified, err = add(amplified, cap.AllocationDenominator-1)
	if err != nil {
		return result, err
	}
	amplified /= cap.AllocationDenominator
	result.PoolRequiredBytes, err = add(amplified, cap.MetadataReserveBytes)
	if err != nil {
		return result, err
	}
	raw, err := mul(amplified, cap.RawCopies)
	if err != nil {
		return result, err
	}
	metadataRaw, err := mul(cap.MetadataReserveBytes, cap.RawCopies)
	if err != nil {
		return result, err
	}
	result.HostRequiredBytes, err = add(raw, metadataRaw)
	if err != nil {
		return result, err
	}
	neededPool, err := add(result.PoolRequiredBytes, space.ReservedPoolBytes)
	if err != nil {
		return result, err
	}
	host, err := add(result.HostRequiredBytes, space.ReservedHostBytes)
	if err != nil {
		return result, err
	}
	host, err = add(host, space.HostFloorBytes)
	if err != nil {
		return result, err
	}
	if neededPool > space.PoolUsableBytes || host > space.HostFreeBytes {
		return result, ErrCapacity
	}
	return result, nil
}
