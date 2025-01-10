package filepersistence

import (
	"context"
	"github.com/goodrain/rainbond/config/configs"
)

// FileSystem represents a file storage system
type FileSystem struct {
	ID             string
	Name           string
	Status         string
	ProtocolType   string
	StorageType    string
	FileSystemType string
	ZoneID         string
	Region         string
	Size           int64
}

// CreateFileSystemOptions contains options for creating a file system
type CreateFileSystemOptions struct {
	Name           string
	ProtocolType   string
	StorageType    string
	FileSystemType string
	VpcID          string
	VSwitchID      string
	SecurityGroup  string
	Description    string
	Size           int64
}

// CreateStorageClassOptions contains options for creating a storage class
type CreateStorageClassOptions struct {
	Name              string
	ReclaimPolicy     string
	VolumeBindingMode string
	Parameters        map[string]string
}

type InterfaceFilePersistence interface {
	FindFileSystem(ctx context.Context, name string) (*FileSystem, error)
	CreateFileSystem(ctx context.Context, opts *CreateFileSystemOptions) (string, error)
}

// ComponentFilePersistence -
type ComponentFilePersistence struct {
	FilePersistenceCli    InterfaceFilePersistence
	FilePersistenceConfig *configs.FilePersistenceConfig
}

var defaultFilePersistenceComponent *ComponentFilePersistence

// New -
func New() *ComponentFilePersistence {
	fpConfig := configs.Default().FilePersistenceConfig
	defaultFilePersistenceComponent = &ComponentFilePersistence{
		FilePersistenceConfig: fpConfig,
	}
	return defaultFilePersistenceComponent
}

// Start -
func (s *ComponentFilePersistence) Start(ctx context.Context) error {
	var fpCli InterfaceFilePersistence
	switch s.FilePersistenceConfig.FilePersistenceType {
	case "volcengine":
		fpCli = &VolcengineProvider{
			config: &VolcengineConfig{
				AccessKey:         s.FilePersistenceConfig.FilePersistenceAccessKeyID,
				SecretKey:         s.FilePersistenceConfig.FilePersistenceSecretAccessKey,
				Region:            s.FilePersistenceConfig.FilePersistenceRegion,
				ZoneID:            s.FilePersistenceConfig.FilePersistenceZoneID,
				VpcID:             s.FilePersistenceConfig.FilePersistenceVpcID,
				SubnetID:          s.FilePersistenceConfig.FilePersistenceSubnetID,
				PermissionGroupID: s.FilePersistenceConfig.FilePersistencePermissionGroupID,
			},
		}
	}
	s.FilePersistenceCli = fpCli
	return nil
}

// CloseHandle -
func (s *ComponentFilePersistence) CloseHandle() {
}

// Default -
func Default() *ComponentFilePersistence {
	return defaultFilePersistenceComponent
}

type SrcFile interface {
	Read([]byte) (int, error)
}

// DstFile 目标文件接口
type DstFile interface {
	Write([]byte) (int, error)
}
