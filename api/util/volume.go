package util

import (
	"strings"

	api_model "github.com/goodrain/rainbond/api/model"
	dbmodel "github.com/goodrain/rainbond/db/model"
	v1 "github.com/goodrain/rainbond/worker/appm/types/v1"
	"github.com/goodrain/rainbond/worker/server/pb"
	"github.com/twinj/uuid"
)

// SetVolumeDefaultValue set volume default value
func SetVolumeDefaultValue(info *dbmodel.TenantServiceVolume) {
	if info.VolumeName == "" {
		info.VolumeName = uuid.NewV4().String()
	}

	if info.AccessMode != "" {
		info.AccessMode = strings.ToUpper(info.AccessMode)
	} else {
		info.AccessMode = "RWO"
	}

	if info.SharePolicy == "" {
		info.SharePolicy = "exclusive"
	}

	if info.BackupPolicy == "" {
		info.BackupPolicy = "exclusive"
	}

	if info.ReclaimPolicy == "" {
		info.ReclaimPolicy = "retain"
	}
}

// ParseVolumeProviderKind parse volume provider kind
func ParseVolumeProviderKind(detail *pb.StorageClassDetail) string {
	volumeType := transferVolumeProviderName2Kind(detail.Name, detail.Provisioner, detail.Parameters)
	if volumeType != nil {
		return volumeType.String()
	}
	return ""
}

func transferVolumeProviderName2Kind(name string, opts ...interface{}) *dbmodel.VolumeType {
	if name == v1.RainbondStatefuleShareStorageClass {
		return &dbmodel.ShareFileVolumeType
	}
	if name == v1.RainbondStatefuleLocalStorageClass {
		return &dbmodel.LocalVolumeType
	}
	if len(opts) > 0 {
		return transferCustomVolumeProviderName2Kind(opts...)
	}
	return nil
}

func transferCustomVolumeProviderName2Kind(opts ...interface{}) *dbmodel.VolumeType {
	if len(opts) != 2 {
		return nil
	}
	kind := opts[0].(string)
	if strings.HasSuffix(kind, "rbd") {
		if parameters, ok := opts[1].(map[string]string); ok {
			if parameters["adminId"] != "" && parameters["monitors"] != "" && parameters["pool"] != "" && parameters["userId"] != "" {
				return &dbmodel.CephRBDVolumeType
			}
		}
	}
	return nil
}

// HackVolumeProviderDetail hack volume provider detail, like accessMode, sharePolicy, backupPolicy
func HackVolumeProviderDetail(kind string, detail *api_model.VolumeProviderDetail) {
	/*
		RWO - ReadWriteOnce
		ROX - ReadOnlyMany
		RWX - ReadWriteMany
	*/
	detail.AccessMode = append(detail.AccessMode, hackVolumeProviderAccessMode(kind)...)
	detail.SharePolicy = append(detail.SharePolicy, hackVolumeProviderSharePolicy(kind)...)
	detail.BackupPolicy = append(detail.BackupPolicy, hackVolumeProviderBackupPolicy(kind)...)
}

/*

## volume accessMode
---

Volume Plugin 		| ReadWriteOnce        |    ReadOnlyMany          | ReadWriteMany
--------------------|----------------------|--------------------------|-----------------------
AWSElasticBlockStore| 	✓		           |   	-	      	          |  -
AzureFile			|    ✓			       |   	✓		  	          |  ✓
AzureDisk			|    ✓			       |   	-		  	          |  -
CephFS			    |    ✓		           |      ✓		          	  |  ✓
Cinder			    |    ✓		           |      -		  	          |  -
CSI					| depends on the driver|	depends on the driver |	depends on the driver
FC					|  ✓				   |   ✓					  | -
FlexVolume			| ✓					   |	✓					  | depends on the driver
Flocker				|	✓				   |  -						  | -
GCEPersistentDisk	|	✓				   | ✓						  | -
Glusterfs			|  ✓				   | ✓	                      | ✓
HostPath	        |  ✓				   | -						  | -
iSCSI				| ✓					   | ✓						  | -
Quobyte				| ✓					   | ✓						  | ✓
NFS					| ✓					   | ✓						  | ✓
RBD					| ✓					   | ✓						  | -
VsphereVolume		| ✓					   | -						  | - (works when Pods are collocated)
PortworxVolume		| ✓					   | -						  | ✓
ScaleIO				| ✓					   | ✓						  | -
StorageOS			| ✓					   | -						  | -

*/
func hackVolumeProviderAccessMode(kind string) []string {
	volumeType := dbmodel.VolumeType(kind)
	switch volumeType {
	case dbmodel.ShareFileVolumeType:
		return []string{"RWO", "ROX", "RWX"}
	case dbmodel.LocalVolumeType:
		return []string{"RWO", "ROX", "RWX"}
	case dbmodel.CephRBDVolumeType:
		return []string{"RWO", "ROX"}
	case dbmodel.ConfigFileVolumeType:
		return []string{"ROX"}
	case dbmodel.MemoryFSVolumeType:
		return []string{"ROX"}
	default:
		return []string{"RWO"}
	}
}

// TODO finish volume share policy
func hackVolumeProviderSharePolicy(kind string) []string {
	return []string{"exclusive"}
}

// TODO finish vollume backup policy
func hackVolumeProviderBackupPolicy(kind string) []string {
	return []string{"exclusive"}
}
