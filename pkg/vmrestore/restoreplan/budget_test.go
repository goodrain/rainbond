package restoreplan

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/goodrain/rainbond/pkg/vmrestore/copyio"
)

func fixture() (time.Time, Capability, SpaceEvidence, []DiskInput) {
	now := time.Now()
	cap := Capability{StorageClassName: "cephfs", StorageClassUID: "class-uid", Driver: "cephfs.csi.ceph.com", DriverVersion: "verified-version", Mounter: "verified-mounter", ROXVerified: true, SparsePersistenceVerified: true, Protocol: copyio.Protocol, WorkerImage: "registry.example/tool@sha256:" + strings.Repeat("a", 64), RawCopies: 3, AllocationNumerator: 2, AllocationDenominator: 1, MetadataReserveBytes: 1 << 20, LogicalPaddingBytes: 1 << 20, MaxLogicalBytes: 1 << 40, TargetUID: 107, TargetGID: 107}
	space := SpaceEvidence{StorageClassUID: cap.StorageClassUID, Source: "ceph-pool-and-host-verified", ObservedAt: now, ValidFor: time.Minute, PoolUsableBytes: 1 << 30, HostFreeBytes: 10 << 30, HostFloorBytes: 5 << 30, ReservationOperationID: "operation-a", ReservationHeld: true}
	source := copyio.FileResult{LogicalBytes: 31 << 30, ReadBytes: 31 << 30, NonzeroChunkBytes: 8 << 20, SHA256: strings.Repeat("b", 64), ReportedStatBlocks: 100000000, StatBlocksIsAllocation: false}
	return now, cap, space, []DiskInput{{VolumeID: 7, BackupRequestBytes: 32 << 30, Source: source, Previous: source}}
}

// capability_id: rainbond.vm-restore.capacity-capability-gate
func TestBudgetUsesVerifiedWritesAndKeepsRealLogicalQuota(t *testing.T) {
	now, cap, space, disks := fixture()
	budget, err := Assess(now, "operation-a", cap, space, disks)
	if err != nil {
		t.Fatal(err)
	}
	if budget.Disks[0].PVCRequestBytes != 32<<30 || budget.Disks[0].WriteLimitBytes != 8<<20 || budget.TargetUID != 107 || budget.PoolRequiredBytes >= 1<<30 {
		t.Fatal("logical size/stat_blocks was confused with actual planned writes")
	}
}
func TestDefaultUnverifiedCapabilityAndStaleSpaceReject(t *testing.T) {
	now, cap, space, disks := fixture()
	for _, kind := range []string{"rox", "persist", "identity", "stale", "unreserved", "space"} {
		t.Run(kind, func(t *testing.T) {
			c, s := cap, space
			switch kind {
			case "rox":
				c.ROXVerified = false
			case "persist":
				c.SparsePersistenceVerified = false
			case "identity":
				s.StorageClassUID = "replacement"
			case "stale":
				s.ObservedAt = now.Add(-2 * time.Minute)
			case "unreserved":
				s.ReservationHeld = false
			case "space":
				s.PoolUsableBytes = 1
			}
			if _, err := Assess(now, "operation-a", c, s, disks); err == nil {
				t.Fatal("unsupported/unreserved capacity was accepted")
			}
		})
	}
	cap.ROXVerified = false
	if _, err := Assess(now, "operation-a", cap, space, disks); !errors.Is(err, ErrCapability) {
		t.Fatal("unverified capability did not fail closed")
	}
}

func TestOverflowAndPendingReadCannotCreateBudget(t *testing.T) {
	now, cap, space, disks := fixture()
	disks[0].Source.ReadBytes--
	if _, err := Assess(now, "operation-a", cap, space, disks); err == nil {
		t.Fatal("partial inspect was accepted")
	}
	disks[0].Source.ReadBytes++
	cap.LogicalPaddingBytes = ^uint64(0)
	if _, err := Assess(now, "operation-a", cap, space, disks); err == nil {
		t.Fatal("capacity arithmetic overflow was accepted")
	}
}

func TestQualifiedNamespacedCephFSDriverKeepsExactClassCertification(t *testing.T) {
	now, cap, space, disks := fixture()
	cap.Driver = "rook-ceph.cephfs.csi.ceph.com"
	if _, err := Assess(now, "operation-a", cap, space, disks); err != nil {
		t.Fatal("qualified namespace-prefixed CephFS driver was rejected")
	}
	cap.Driver = "unrelated.csi.example.com"
	if _, err := Assess(now, "operation-a", cap, space, disks); !errors.Is(err, ErrCapability) {
		t.Fatal("unqualified non-CephFS driver was accepted")
	}
}

func TestBudgetRequiresExplicitExecutableReservationOwner(t *testing.T) {
	now, cap, space, disks := fixture()
	for _, operationID := range []string{"", " ", "operation\n", "operation/other", "UPPER", "operation-", strings.Repeat("a", 64)} {
		t.Run(operationID, func(t *testing.T) {
			s := space
			s.ReservationOperationID = operationID
			if _, err := Assess(now, operationID, cap, s, disks); !errors.Is(err, ErrCapacity) {
				t.Fatal("invalid operation identity claimed a physical reservation")
			}
		})
	}
	for _, operationID := range []string{"operation-a", strings.Repeat("a", 63)} {
		s := space
		s.ReservationOperationID = operationID
		if _, err := Assess(now, operationID, cap, s, disks); err != nil {
			t.Fatal("valid reservation identity was rejected")
		}
	}
}

func TestBudgetQuotaFitsKubernetesQuantity(t *testing.T) {
	for _, field := range []string{"backup", "snapshot"} {
		t.Run(field, func(t *testing.T) {
			now, cap, space, disks := fixture()
			if field == "backup" {
				disks[0].BackupRequestBytes = ^uint64(0)
			} else {
				disks[0].SnapshotRestoreBytes = ^uint64(0)
			}
			if _, err := Assess(now, "operation-a", cap, space, disks); !errors.Is(err, ErrCapacity) {
				t.Fatal("unrepresentable PVC quota was accepted")
			}
		})
	}
	now, cap, space, disks := fixture()
	disks[0].SnapshotRestoreBytes = 40 << 30
	budget, err := Assess(now, "operation-a", cap, space, disks)
	if err != nil || budget.Disks[0].PVCRequestBytes != 40<<30 {
		t.Fatal("valid larger snapshot restore quota was lost")
	}
}
