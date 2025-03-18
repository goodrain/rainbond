package filepersistence

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	testAccessKey     = ""
	testSecretKey     = ""
	testRegion        = "cn-shanghai"
	testZoneID        = "cn-shanghai-b"
	vpcID             = "vpc-22jma6cmqya687r2qr1rxiruc"
	subnetID          = "subnet-3qdqedpvcrxfk7prmkzy65nb3"
	permissionGroupID = "pgroup-default"
)

func skipIfCredentialsNotSet(t *testing.T) {
	if testAccessKey == "" || testSecretKey == "" || testRegion == "" {
		t.Skip("Skipping test because credentials are not set. Please set VOLCENGINE_ACCESS_KEY, VOLCENGINE_SECRET_KEY, VOLCENGINE_REGION, and VOLCENGINE_ZONE_ID environment variables.")
	}
}

func TestVolcengineProvider_FindNonExistentFileSystem(t *testing.T) {
	skipIfCredentialsNotSet(t)

	ctx := context.Background()
	provider := &VolcengineProvider{
		config: &VolcengineConfig{
			AccessKey: testAccessKey,
			SecretKey: testSecretKey,
			Region:    testRegion,
		},
	}

	tt, err := provider.FindFileSystem(ctx, "gr8b2f8b")
	fmt.Println(tt)
	if err != nil {
		fmt.Println(err)
	}
}

func TestVolcengineProvider_CreateFileSystemValidation(t *testing.T) {
	skipIfCredentialsNotSet(t)

	ctx := context.Background()
	provider := &VolcengineProvider{
		config: &VolcengineConfig{
			AccessKey:         testAccessKey,
			SecretKey:         testSecretKey,
			Region:            testRegion,
			VpcID:             vpcID,
			SubnetID:          subnetID,
			PermissionGroupID: permissionGroupID,
			ZoneID:            testZoneID,
		},
	}

	testCases := []struct {
		name        string
		opts        *CreateFileSystemOptions
		expectError bool
	}{
		{
			name: "Zero Size",
			opts: &CreateFileSystemOptions{
				Name:           "zq-gr534828",
				ProtocolType:   "NFS",
				StorageType:    "Standard",
				Size:           100 * 1024 * 1024 * 1024,
				FileSystemType: "Capacity",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ret, err := provider.CreateFileSystem(ctx, tc.opts)
			fmt.Println(ret)
			if err != nil {
				fmt.Println(err)
			}
		})
	}
}

func TestVolcengineProvider_DeleteFileSystem(t *testing.T) {
	skipIfCredentialsNotSet(t)

	ctx := context.Background()
	provider := &VolcengineProvider{
		config: &VolcengineConfig{
			AccessKey:         testAccessKey,
			SecretKey:         testSecretKey,
			Region:            testRegion,
			VpcID:             vpcID,
			SubnetID:          subnetID,
			PermissionGroupID: permissionGroupID,
			ZoneID:            testZoneID,
		},
	}

	fileSystemName := "afaafa" // Replace with a valid file system ID for a real test
	err := provider.DeleteFileSystem(ctx, fileSystemName)
	assert.NoError(t, err, "failed to delete file system")
}

func TestVolcengineProvider_SetDirQuota(t *testing.T) {
	skipIfCredentialsNotSet(t)
	provider := &VolcengineProvider{
		config: &VolcengineConfig{
			AccessKey: testAccessKey,
			SecretKey: testSecretKey,
			Region:    testRegion,
		},
	}
	fmt.Println("------------------------设置---------------------------")
	err := provider.SetDirQuota("cnas-b06597312179ec", "/pvc-9a94fa6c-ab4a-4778-b9e5-e2ad2e89e34f")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("------------------------结束-------------------------------")
}

func TestVolcengineProvider_CancelDirQuota(t *testing.T) {
	skipIfCredentialsNotSet(t)
	provider := &VolcengineProvider{
		config: &VolcengineConfig{
			AccessKey: testAccessKey,
			SecretKey: testSecretKey,
			Region:    testRegion,
		},
	}
	fmt.Println("------------------------取消-------------------------------")
	err := provider.CancelDirQuota("cnas-b06597312179ec", "/pvc-9a94fa6c-ab4a-4778-b9e5-e2ad2e89e34f")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("------------------------结束-------------------------------")
}

func TestVolcengineProvider_DescribeDirQuota(t *testing.T) {
	skipIfCredentialsNotSet(t)
	provider := &VolcengineProvider{
		config: &VolcengineConfig{
			AccessKey: testAccessKey,
			SecretKey: testSecretKey,
			Region:    testRegion,
		},
	}
	fileQuotas, err := provider.DescribeDirQuota("cnas-b06597312179ec", "/pvc-9a94fa6c-ab4a-4778-b9e5-e2ad2e89e34f")
	if err != nil {
		fmt.Println(err)
	}
	for _, fileQuota := range fileQuotas {
		fmt.Println(*fileQuota.Path)
		fmt.Println(*fileQuota.UserQuotaInfos[0].SizeReal)
	}
	fmt.Println("------------------------结束-------------------------------")
}
