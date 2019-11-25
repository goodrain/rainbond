package util

import (
	"strings"

	dbmodel "github.com/goodrain/rainbond/db/model"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/goodrain/rainbond/worker/server/pb"
)

// ParseVolumeProviderKind parse volume provider kind
func ParseVolumeProviderKind(detail *pb.StorageClassDetail) string {
	if detail.Name == v1.RainbondStatefuleShareStorageClass {
		return dbmodel.ShareFileVolumeType.String()
	}
	if detail.Name == v1.RainbondStatefuleLocalStorageClass {
		return dbmodel.LocalVolumeType.String()
	}
	if strings.HasSuffix(detail.Provisioner, "rbd") {
		if detail.Parameters != nil {
			if detail.Parameters["adminId"] != "" && detail.Parameters["monitors"] != "" && detail.Parameters["pool"] != "" && detail.Parameters["userId"] != "" {
				return dbmodel.CephRBDVolumeType.String()
			}
		}
	}
	return ""
}
