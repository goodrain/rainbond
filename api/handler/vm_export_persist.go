package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/goodrain/rainbond/builder/sourceutil"
	"github.com/goodrain/rainbond/config/configs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

const vmExportAssetBucket = "vm-assets"

var (
	vmExportS3ClientFactory = func() (*s3.S3, string, string, error) {
		storageConfig := configs.Default().StorageConfig
		if storageConfig == nil || strings.TrimSpace(storageConfig.StorageType) != "s3" {
			return nil, "", "", fmt.Errorf("vm export object persistence requires s3 storage")
		}
		sess, err := session.NewSession(&aws.Config{
			Endpoint:         aws.String(storageConfig.S3Endpoint),
			Region:           aws.String("rainbond"),
			Credentials:      credentials.NewStaticCredentials(storageConfig.S3AccessKeyID, storageConfig.S3SecretAccessKey, ""),
			S3ForcePathStyle: aws.Bool(true),
		})
		if err != nil {
			return nil, "", "", err
		}
		return s3.New(sess), vmExportAssetBucket, strings.TrimSpace(storageConfig.S3Endpoint), nil
	}
	vmExportHTTPClientFactory = func(rawURL string) *http.Client {
		return sourceutil.NewRemotePackageHTTPClient(rawURL)
	}
)

func (s *ServiceAction) PersistVMExport(serviceID, exportID string, req *VMExportPersistRequest) (*VMExportPersistStatus, error) {
	status, err := BuildVMExportStatus(vmExportDynamicClient(), serviceID, exportID)
	if err != nil {
		return nil, err
	}
	if status.Status != "ready" {
		return &VMExportPersistStatus{
			ExportID: exportID,
			Status:   status.Status,
		}, nil
	}

	s3Client, bucketName, endpoint, err := vmExportS3ClientFactory()
	if err != nil {
		return nil, err
	}
	if err := ensureVMExportBucketExists(s3Client, bucketName); err != nil {
		return nil, err
	}

	uploaded := make(map[string]VMExportUploadedDisk, len(status.Disks))
	for _, disk := range status.Disks {
		meta, err := uploadVMExportDiskToObjectStorage(s3Client, bucketName, endpoint, req, disk)
		if err != nil {
			return nil, err
		}
		uploaded[disk.DiskKey] = meta
	}
	manifest, rootObjectURI, err := buildVMMachineManifest("", "", status, uploaded)
	if err != nil {
		return nil, err
	}
	if err := deleteVMExportResources(vmExportDynamicClient(), serviceID, exportID); err != nil {
		return nil, err
	}
	return &VMExportPersistStatus{
		ExportID:        exportID,
		Status:          "ready",
		StorageBackend:  "s3",
		StorageBucket:   bucketName,
		StoragePrefix:   vmExportAssetPrefix(req),
		RootObjectURI:   rootObjectURI,
		MachineManifest: manifest,
	}, nil
}

func (s *ServiceAction) BuildVMAssetRestorePlan(req *VMAssetRestorePlanRequest) (*VMAssetRestorePlan, error) {
	if req == nil {
		return nil, fmt.Errorf("restore plan request is required")
	}
	s3Client, bucketName, _, err := vmExportS3ClientFactory()
	if err != nil {
		return nil, err
	}
	return buildVMAssetRestorePlan(req.Manifest, func(objectKey string) (string, error) {
		if strings.TrimSpace(objectKey) == "" {
			return "", fmt.Errorf("restore object key is required")
		}
		request, _ := s3Client.GetObjectRequest(&s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(objectKey),
		})
		return request.Presign(30 * time.Minute)
	})
}

func ensureVMExportBucketExists(client *s3.S3, bucketName string) error {
	if client == nil {
		return fmt.Errorf("s3 client is required")
	}
	if strings.TrimSpace(bucketName) == "" {
		return fmt.Errorf("bucket name is required")
	}
	if _, err := client.HeadBucket(&s3.HeadBucketInput{Bucket: aws.String(bucketName)}); err == nil {
		return nil
	}
	_, err := client.CreateBucket(&s3.CreateBucketInput{Bucket: aws.String(bucketName)})
	return err
}

func uploadVMExportDiskToObjectStorage(client *s3.S3, bucketName, endpoint string, req *VMExportPersistRequest, disk VMExportDisk) (VMExportUploadedDisk, error) {
	if strings.TrimSpace(disk.DownloadURL) == "" {
		return VMExportUploadedDisk{}, fmt.Errorf("export disk %s has no download url", disk.DiskKey)
	}
	httpClient := vmExportHTTPClientFactory(disk.DownloadURL)
	response, err := httpClient.Get(disk.DownloadURL)
	if err != nil {
		return VMExportUploadedDisk{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return VMExportUploadedDisk{}, fmt.Errorf("download vm export disk %s failed: status=%d", disk.DiskKey, response.StatusCode)
	}

	objectKey := buildVMExportObjectKey(req, disk)
	tempFile, err := os.CreateTemp("", "vm-export-*")
	if err != nil {
		return VMExportUploadedDisk{}, err
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
	if _, err := io.Copy(tempFile, response.Body); err != nil {
		return VMExportUploadedDisk{}, err
	}
	if _, err := tempFile.Seek(0, 0); err != nil {
		return VMExportUploadedDisk{}, err
	}
	input := &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
		Body:   tempFile,
	}
	if response.ContentLength > 0 {
		input.ContentLength = aws.Int64(response.ContentLength)
	}
	if response.Header.Get("Content-Type") != "" {
		input.ContentType = aws.String(response.Header.Get("Content-Type"))
	}
	if _, err := client.PutObject(input); err != nil {
		return VMExportUploadedDisk{}, err
	}

	format := strings.TrimPrefix(path.Ext(strings.Split(strings.TrimSpace(disk.DownloadURL), "?")[0]), ".")
	if format == "" {
		format = "img"
	}
	return VMExportUploadedDisk{
		DiskKey:    disk.DiskKey,
		ObjectKey:  objectKey,
		ObjectURI:  fmt.Sprintf("s3://%s/%s", bucketName, objectKey),
		StorageURL: buildVMExportObjectStorageURL(endpoint, bucketName, objectKey),
		Format:     format,
		SizeBytes:  response.ContentLength,
	}, nil
}

func buildVMExportObjectKey(req *VMExportPersistRequest, disk VMExportDisk) string {
	fileName := path.Base(strings.Split(strings.TrimSpace(disk.DownloadURL), "?")[0])
	if fileName == "" || fileName == "." || fileName == "/" {
		fileName = sanitizeVMExportName(disk.DiskKey)
		if ext := path.Ext(strings.TrimSpace(disk.DownloadURL)); ext != "" {
			fileName += ext
		}
	}
	return fmt.Sprintf("%s/%s", vmExportAssetPrefix(req), fileName)
}

func vmExportAssetPrefix(req *VMExportPersistRequest) string {
	assetID := int64(0)
	if req != nil {
		assetID = req.AssetID
	}
	return fmt.Sprintf("vm-export/assets/%d", assetID)
}

func buildVMExportObjectStorageURL(endpoint, bucketName, objectKey string) string {
	endpoint = strings.TrimSpace(endpoint)
	bucketName = strings.TrimSpace(bucketName)
	objectKey = strings.TrimLeft(strings.TrimSpace(objectKey), "/")
	if endpoint == "" || bucketName == "" || objectKey == "" {
		return ""
	}
	parsed, err := neturl.Parse(endpoint)
	if err != nil {
		return ""
	}
	parsed.Path = path.Join(parsed.Path, bucketName, objectKey)
	return parsed.String()
}

func deleteVMExportResources(dynamicClient dynamic.Interface, serviceID, exportID string) error {
	if dynamicClient == nil {
		return fmt.Errorf("dynamic client is nil")
	}
	list, err := dynamicClient.Resource(vmDataExportGVR).Namespace(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, item := range list.Items {
		labels := item.GetLabels()
		if labels["service_id"] != serviceID || labels["vm_export_id"] != exportID {
			continue
		}
		if err := dynamicClient.Resource(vmDataExportGVR).Namespace(item.GetNamespace()).Delete(context.Background(), item.GetName(), metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}
