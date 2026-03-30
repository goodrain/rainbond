package util

import (
	storagev1 "k8s.io/api/storage/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		wantNameShow    string
		wantProvisioner string
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
			wantNameShow:    "ali-disk-sc",
			wantProvisioner: "aaa",
		},
		{
			name: "with_wrong_annotation",
			args: args{sc: &storagev1.StorageClass{
				ObjectMeta: v1.ObjectMeta{
					Name:        "ali-disk-sc",
					Annotations: map[string]string{"volume_show": "123"},
				},
			}},
			wantNameShow: "ali-disk-sc",
		},
		{
			name: "with_annotation",
			args: args{sc: &storagev1.StorageClass{
				ObjectMeta: v1.ObjectMeta{
					Name:        "ali-disk-sc",
					Annotations: map[string]string{"rbd_volume_name": "new-volume-type"},
				},
			}},
			wantNameShow: "new-volume-type",
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
			if got.VolumeType != "ali-disk-sc" {
				t.Fatalf("expected VolumeType ali-disk-sc, got %q", got.VolumeType)
			}
			if got.ReclaimPolicy != "Retain" {
				t.Fatalf("expected ReclaimPolicy Retain, got %q", got.ReclaimPolicy)
			}
			if tt.wantProvisioner != "" && got.Provisioner != tt.wantProvisioner {
				t.Fatalf("expected Provisioner %q, got %q", tt.wantProvisioner, got.Provisioner)
			}
		})
	}
}
