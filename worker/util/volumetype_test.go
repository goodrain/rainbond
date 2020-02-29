package util

import (
	dbmodel "github.com/goodrain/rainbond/db/model"
	storagev1 "k8s.io/api/storage/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestTransStorageClass2RBDVolumeType(t *testing.T) {
	type args struct {
		sc *storagev1.StorageClass
	}
	tests := []struct {
		name string
		args args
		want *dbmodel.TenantServiceVolumeType
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
		},
		{
			name: "with_wrong_annotation",
			args: args{sc: &storagev1.StorageClass{
				ObjectMeta: v1.ObjectMeta{
					Name:        "ali-disk-sc",
					Annotations: map[string]string{"volume_show": "123"},
				},
			}},
		},
		{
			name: "with_annotation",
			args: args{sc: &storagev1.StorageClass{
				ObjectMeta: v1.ObjectMeta{
					Name:        "ali-disk-sc",
					Annotations: map[string]string{"rbd_volume_name": "new-volume-type"},
				},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TransStorageClass2RBDVolumeType(tt.args.sc)
			t.Logf("volume type is : %+v", got)
		})
	}
}
