package dao

import (
	"testing"

	"github.com/goodrain/rainbond/db/model"
)

// capability_id: rainbond.worker.volume-type.from-storageclass
func TestShouldBackfillStorageClassAccessMode(t *testing.T) {
	tests := []struct {
		name     string
		existing *model.TenantServiceVolumeType
		incoming *model.TenantServiceVolumeType
		want     bool
	}{
		{
			name: "legacy_default_rwo_for_shared_storage",
			existing: &model.TenantServiceVolumeType{
				VolumeType:   "nfs-storage",
				NameShow:     "nfs-storage",
				AccessMode:   "RWO",
				SharePolicy:  "exclusive",
				BackupPolicy: "exclusive",
				Provisioner:  "cluster.local/nfs-subdir-external-provisioner",
				Enable:       true,
				Sort:         999,
			},
			incoming: &model.TenantServiceVolumeType{
				VolumeType:   "nfs-storage",
				NameShow:     "nfs-storage",
				AccessMode:   "RWO,ROX,RWX",
				SharePolicy:  "exclusive",
				BackupPolicy: "exclusive",
				Provisioner:  "cluster.local/nfs-subdir-external-provisioner",
			},
			want: true,
		},
		{
			name: "customized_name_show_is_preserved",
			existing: &model.TenantServiceVolumeType{
				VolumeType:   "nfs-storage",
				NameShow:     "Team NFS",
				AccessMode:   "RWO",
				SharePolicy:  "exclusive",
				BackupPolicy: "exclusive",
				Provisioner:  "cluster.local/nfs-subdir-external-provisioner",
				Enable:       true,
				Sort:         999,
			},
			incoming: &model.TenantServiceVolumeType{
				VolumeType:   "nfs-storage",
				NameShow:     "nfs-storage",
				AccessMode:   "RWO,ROX,RWX",
				SharePolicy:  "exclusive",
				BackupPolicy: "exclusive",
				Provisioner:  "cluster.local/nfs-subdir-external-provisioner",
			},
			want: false,
		},
		{
			name: "already_custom_access_mode_is_preserved",
			existing: &model.TenantServiceVolumeType{
				VolumeType:   "nfs-storage",
				NameShow:     "nfs-storage",
				AccessMode:   "RWX",
				SharePolicy:  "exclusive",
				BackupPolicy: "exclusive",
				Provisioner:  "cluster.local/nfs-subdir-external-provisioner",
				Enable:       true,
				Sort:         999,
			},
			incoming: &model.TenantServiceVolumeType{
				VolumeType:   "nfs-storage",
				NameShow:     "nfs-storage",
				AccessMode:   "RWO,ROX,RWX",
				SharePolicy:  "exclusive",
				BackupPolicy: "exclusive",
				Provisioner:  "cluster.local/nfs-subdir-external-provisioner",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldBackfillStorageClassAccessMode(tt.existing, tt.incoming)
			if got != tt.want {
				t.Fatalf("expected shouldBackfillStorageClassAccessMode=%t, got %t", tt.want, got)
			}
		})
	}
}
