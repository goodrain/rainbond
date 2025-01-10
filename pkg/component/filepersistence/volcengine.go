package filepersistence

import (
	"context"
	"fmt"
	"time"

	"github.com/volcengine/volcengine-go-sdk/service/filenas"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials"
	"github.com/volcengine/volcengine-go-sdk/volcengine/session"
)

// VolcengineProvider implements the Provider interface for Volcengine NAS
type VolcengineProvider struct {
	client *filenas.FILENAS
	config *VolcengineConfig
}

// VolcengineConfig contains configuration for Volcengine NAS
type VolcengineConfig struct {
	AccessKey         string
	SecretKey         string
	Region            string
	ZoneID            string
	VpcID             string
	SubnetID          string
	PermissionGroupID string
}

func (p *VolcengineProvider) init() error {
	if p.client != nil {
		return nil
	}
	config := volcengine.NewConfig().
		WithCredentials(credentials.NewStaticCredentials(p.config.AccessKey, p.config.SecretKey, "")).
		WithRegion(p.config.Region)

	sess, err := session.NewSession(config)
	if err != nil {
		return fmt.Errorf("failed to create Volcengine session: %v", err)
	}

	p.client = filenas.New(sess)
	return nil
}

// FindFileSystem finds a file system by ID
func (p *VolcengineProvider) FindFileSystem(ctx context.Context, name string) (*FileSystem, error) {
	if err := p.init(); err != nil {
		return nil, err
	}
	key := "FileSystemName"
	input := &filenas.DescribeFileSystemsInput{
		Filters: []*filenas.FilterForDescribeFileSystemsInput{
			{
				Key:   &key,
				Value: &name,
			},
		},
	}

	output, err := p.client.DescribeFileSystems(input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe file system: %v", err)
	}

	if len(output.FileSystems) == 0 {
		return nil, fmt.Errorf("file system %s not found", name)
	}

	fs := output.FileSystems[0]
	var size int64
	if fs.Capacity != nil && fs.Capacity.Total != nil {
		size = *fs.Capacity.Total
	}

	return &FileSystem{
		ID:             *fs.FileSystemId,
		Name:           *fs.FileSystemName,
		Status:         *fs.Status,
		ProtocolType:   *fs.ProtocolType,
		StorageType:    *fs.StorageType,
		FileSystemType: *fs.FileSystemType,
		ZoneID:         *fs.ZoneId,
		Region:         p.config.Region,
		Size:           size,
	}, nil
}

// CreateFileSystem creates a new file system
func (p *VolcengineProvider) CreateFileSystem(ctx context.Context, opts *CreateFileSystemOptions) (string, error) {
	if err := p.init(); err != nil {
		return "", err
	}

	capacityGB := int32(opts.Size / (1024 * 1024 * 1024)) // Convert bytes to GB
	chargeType := "PayAsYouGo"
	input := &filenas.CreateFileSystemInput{
		ZoneId:         &p.config.ZoneID,
		FileSystemName: &opts.Name,
		FileSystemType: &opts.FileSystemType,
		ProtocolType:   &opts.ProtocolType,
		ChargeType:     &chargeType,
		Description:    &opts.Description,
		Capacity:       &capacityGB,
	}
	output, err := p.client.CreateFileSystem(input)
	if err != nil {
		return "", fmt.Errorf("failed to create file system: %v", err)
	}
	if output.Metadata.Error != nil {
		return "", fmt.Errorf("failed to create file system: %v", output.Metadata.Error)
	}
	for i := 0; i < 60; i++ {
		mpInput := &filenas.CreateMountPointInput{
			FileSystemId:      output.FileSystemId,
			MountPointName:    &opts.Name,
			PermissionGroupId: &p.config.PermissionGroupID,
			SubnetId:          &p.config.SubnetID,
			VpcId:             &p.config.VpcID,
		}
		mpOutput, _ := p.client.CreateMountPoint(mpInput)
		if mpOutput.Metadata.Error != nil {
			if mpOutput.Metadata.Error.Message != "The specified FileSystem is unhealthy." {
				return "", fmt.Errorf("failed to create mount point: %v", mpOutput.Metadata.Error)
			}
		} else {
			break
		}
		time.Sleep(time.Second)
	}
	for t := 0; t < 600; t++ {
		mpInput := &filenas.DescribeMountPointsInput{
			FileSystemId:   output.FileSystemId,
			MountPointName: &opts.Name,
		}
		mpi, err := p.client.DescribeMountPoints(mpInput)
		if err != nil {
			return "", err
		}
		if mpi.MountPoints != nil && len(mpi.MountPoints) > 0 {
			if *mpi.MountPoints[0].Domain != "" {
				return *mpi.MountPoints[0].Domain, nil
			}
		}
	}
	return "", fmt.Errorf("failed to describe mount point: %v", opts.Name)
}
