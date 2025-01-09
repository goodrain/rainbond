package configs

import "github.com/spf13/pflag"

type FilePersistenceConfig struct {
	FilePersistenceType              string `json:"file_persistence_type"`
	FilePersistenceAccessKeyID       string `json:"file_persistence_access_key_id"`
	FilePersistenceSecretAccessKey   string `json:"file_persistence_secret_access_key"`
	FilePersistenceRegion            string `json:"file_persistence_region"`
	FilePersistenceZoneID            string `json:"file_persistence_zone_id"`
	FilePersistenceVpcID             string `json:"file_persistence_vpc_id"`
	FilePersistenceSubnetID          string `json:"file_persistence_subnet_id"`
	FilePersistencePermissionGroupID string `json:"file_persistence_permission_group_id"`
	FilePersistenceEnable            string `json:"file_persistence_enable"`
}

func AddFilePersistenceFlags(fs *pflag.FlagSet, fpc *FilePersistenceConfig) {
	fs.StringVar(&fpc.FilePersistenceType, "file-persistence-type", "volcengine", "volcengine、aliyun or tencentcloud")
	fs.StringVar(&fpc.FilePersistenceAccessKeyID, "file-persistence-access-key-id", "", "access key id")
	fs.StringVar(&fpc.FilePersistenceSecretAccessKey, "file-persistence-secret-access-key", "", "secret access key")
	fs.StringVar(&fpc.FilePersistenceRegion, "file-persistence-region", "cn-shanghai", "region")
	fs.StringVar(&fpc.FilePersistenceZoneID, "file-persistence-zone-id", "cn-shanghai-b", "zone id")
	fs.StringVar(&fpc.FilePersistenceVpcID, "file-persistence-vpc-id", "", "file persistence vpc id")
	fs.StringVar(&fpc.FilePersistenceSubnetID, "file-persistence-subnet-id", "", "file persistence subnet id")
	fs.StringVar(&fpc.FilePersistencePermissionGroupID, "file-persistence-permission-group-id", "", "file persistence permission group id")
	fs.StringVar(&fpc.FilePersistenceEnable, "file-persistence-enable", "open", "open or close")
}
