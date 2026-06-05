package util

import (
	storagev1 "k8s.io/api/storage/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"testing"
)

// capability_id: rainbond.worker.volume-type.from-storageclass
func TestTransStorageClass2RBDVolumeType(t *testing.T) {
	type args struct {
		sc *storagev1.StorageClass
	}
	tests := []struct {
		name            string
		args            args
		wantVolumeType  string
		wantNameShow    string
		wantProvisioner string
		wantAccessMode  string
	}{
		{
			name: "without_annotation",
			args: args{sc: &storagev1.StorageClass{
				ObjectMeta: v1.ObjectMeta{
					Name: "ali-disk-sc",
				},
				Provisioner: "aaa",
				Parameters:  map[string]string{},
			}},
			wantVolumeType:  "ali-disk-sc",
			wantNameShow:    "ali-disk-sc",
			wantProvisioner: "aaa",
			wantAccessMode:  "RWO",
		},
		{
			name: "with_wrong_annotation",
			args: args{sc: &storagev1.StorageClass{
				ObjectMeta: v1.ObjectMeta{
					Name:        "ali-disk-sc",
					Annotations: map[string]string{"volume_show": "123"},
				},
			}},
			wantVolumeType: "ali-disk-sc",
			wantNameShow:   "ali-disk-sc",
			wantAccessMode: "RWO",
		},
		{
			name: "with_annotation",
			args: args{sc: &storagev1.StorageClass{
				ObjectMeta: v1.ObjectMeta{
					Name:        "ali-disk-sc",
					Annotations: map[string]string{"rbd_volume_name": "new-volume-type"},
				},
			}},
			wantVolumeType: "ali-disk-sc",
			wantNameShow:   "new-volume-type",
			wantAccessMode: "RWO",
		},
		{
			name: "shared_nfs_provisioner",
			args: args{sc: &storagev1.StorageClass{
				ObjectMeta: v1.ObjectMeta{
					Name: "nfs-storage",
				},
				Provisioner: "cluster.local/nfs-subdir-external-provisioner",
			}},
			wantVolumeType:  "nfs-storage",
			wantNameShow:    "nfs-storage",
			wantProvisioner: "cluster.local/nfs-subdir-external-provisioner",
			wantAccessMode:  "RWO,ROX,RWX",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TransStorageClass2RBDVolumeType(tt.args.sc)
			if got == nil {
				t.Fatal("expected volume type, got nil")
			}
			if got.NameShow != tt.wantNameShow {
				t.Fatalf("expected NameShow %q, got %q", tt.wantNameShow, got.NameShow)
			}
			if got.VolumeType != tt.wantVolumeType {
				t.Fatalf("expected VolumeType %q, got %q", tt.wantVolumeType, got.VolumeType)
			}
			if got.ReclaimPolicy != "Retain" {
				t.Fatalf("expected ReclaimPolicy Retain, got %q", got.ReclaimPolicy)
			}
			if tt.wantProvisioner != "" && got.Provisioner != tt.wantProvisioner {
				t.Fatalf("expected Provisioner %q, got %q", tt.wantProvisioner, got.Provisioner)
			}
			if got.AccessMode != tt.wantAccessMode {
				t.Fatalf("expected AccessMode %q, got %q", tt.wantAccessMode, got.AccessMode)
			}
		})
	}
}

func TestSharedProvisionerAccessModes(t *testing.T) {
	tests := []struct {
		name        string
		provisioner string
		want        []string
	}{
		{
			name:        "nfs",
			provisioner: "cluster.local/nfs-subdir-external-provisioner",
			want:        []string{"RWO", "ROX", "RWX"},
		},
		{
			name:        "cephfs",
			provisioner: "rook-ceph.cephfs.csi.ceph.com",
			want:        []string{"RWO", "ROX", "RWX"},
		},
		{
			name:        "local_path",
			provisioner: "rancher.io/local-path",
			want:        []string{"RWO"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferStorageClassAccessModes(&storagev1.StorageClass{
				Provisioner: tt.provisioner,
			})
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("expected AccessModes %#v, got %#v", tt.want, got)
			}
		})
	}
}
