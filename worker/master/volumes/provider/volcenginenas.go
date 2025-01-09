package provider

//
//import (
//	"fmt"
//	"os"
//	"path"
//	"strconv"
//
//	"github.com/goodrain/rainbond/util"
//	"github.com/goodrain/rainbond/worker/master/volumes/provider/lib/controller"
//	"github.com/sirupsen/logrus"
//	"github.com/volcengine/volcengine-go-sdk/service/filenas"
//	"github.com/volcengine/volcengine-go-sdk/volcengine"
//	"github.com/volcengine/volcengine-go-sdk/volcengine/credentials"
//	v1 "k8s.io/api/core/v1"
//	"k8s.io/apimachinery/pkg/api/resource"
//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
//)
//
//type volcengineNasProvisioner struct {
//	client    *filenas.FILENAS
//	region    string
//	zoneID    string
//	pvDir     string
//	name      string
//}
//
//// NewVolcengineNasProvisioner creates a new volcengine nas provisioner
//func NewVolcengineNasProvisioner() controller.Provisioner {
//	accessKey := os.Getenv("VOLCENGINE_ACCESS_KEY")
//	secretKey := os.Getenv("VOLCENGINE_SECRET_KEY")
//	region := os.Getenv("VOLCENGINE_REGION")
//	zoneID := os.Getenv("VOLCENGINE_ZONE_ID")
//	if accessKey == "" || secretKey == "" || region == "" || zoneID == "" {
//		logrus.Error("volcengine credentials not found")
//		return nil
//	}
//
//	creds := credentials.NewStaticCredentials(accessKey, secretKey, "")
//	config := volcengine.NewConfig().
//		WithCredentials(creds).
//		WithRegion(region)
//
//	client := filenas.New(config)
//
//	sharePath := os.Getenv("SHARE_DATA_PATH")
//	if sharePath == "" {
//		sharePath = "/grdata"
//	}
//
//	return &volcengineNasProvisioner{
//		client: client,
//		region: region,
//		zoneID: zoneID,
//		pvDir:  sharePath,
//		name:   "nas.csi.volcengine.com",
//	}
//}
//
//func (p *volcengineNasProvisioner) Provision(options controller.VolumeOptions) (*v1.PersistentVolume, error) {
//	logrus.Debugf("[volcengineNasProvisioner] start creating PV object. parameters: %+v", options.Parameters)
//
//	capacity := options.PVC.Spec.Resources.Requests[v1.ResourceStorage]
//	capacityGB := int32(capacity.Value() / (1024 * 1024 * 1024)) // convert to GB
//	if capacityGB < 100 {
//		capacityGB = 100 // minimum size is 100GB for Volcengine NAS
//	}
//
//	fileSystemName := fmt.Sprintf("rainbond-%s", options.PVName)
//	fileSystemType := "Extreme" // 可以从 options.Parameters 中获取，这里使用默认值
//	protocolType := "NFS"
//	chargeType := "PayAsYouGo"
//
//	// 创建文件系统
//	createResp, err := p.client.CreateFileSystem(&filenas.CreateFileSystemInput{
//		ZoneId:         &p.zoneID,
//		FileSystemName: &fileSystemName,
//		FileSystemType: &fileSystemType,
//		ProtocolType:   &protocolType,
//		ChargeType:     &chargeType,
//		Capacity:       &capacityGB,
//	})
//	if err != nil {
//		logrus.Errorf("create volcengine nas filesystem error: %v", err)
//		return nil, err
//	}
//
//	// 等待文件系统创建完成
//	fileSystemID := createResp.FileSystem.FileSystemId
//	err = p.waitForFileSystemReady(fileSystemID)
//	if err != nil {
//		return nil, err
//	}
//
//	// 获取挂载点
//	descResp, err := p.client.DescribeFileSystems(&filenas.DescribeFileSystemsInput{
//		FileSystemIds: []string{fileSystemID},
//	})
//	if err != nil {
//		return nil, err
//	}
//	if len(descResp.FileSystems) == 0 {
//		return nil, fmt.Errorf("file system %s not found", fileSystemID)
//	}
//
//	mountPoint := descResp.FileSystems[0].MountPoint
//	if mountPoint == "" {
//		return nil, fmt.Errorf("mount point is empty for file system %s", fileSystemID)
//	}
//
//	// 创建 PV
//	pv := &v1.PersistentVolume{
//		ObjectMeta: metav1.ObjectMeta{
//			Name:   options.PVName,
//			Labels: options.PVC.Labels,
//		},
//		Spec: v1.PersistentVolumeSpec{
//			PersistentVolumeReclaimPolicy: options.PersistentVolumeReclaimPolicy,
//			AccessModes:                   options.PVC.Spec.AccessModes,
//			Capacity: v1.ResourceList{
//				v1.ResourceStorage: options.PVC.Spec.Resources.Requests[v1.ResourceStorage],
//			},
//			PersistentVolumeSource: v1.PersistentVolumeSource{
//				CSI: &v1.CSIPersistentVolumeSource{
//					Driver: p.name,
//					VolumeAttributes: map[string]string{
//						"server":        mountPoint,
//						"fileSystemId": fileSystemID,
//						"path":         "/",
//					},
//				},
//			},
//		},
//	}
//
//	logrus.Infof("created volcengine nas pv %s for pvc %s", pv.Name, options.PVC.Name)
//	return pv, nil
//}
//
//func (p *volcengineNasProvisioner) Delete(volume *v1.PersistentVolume) error {
//	if volume.Spec.CSI == nil {
//		return fmt.Errorf("volume %s is not a CSI volume", volume.Name)
//	}
//
//	fileSystemID := volume.Spec.CSI.VolumeAttributes["fileSystemId"]
//	if fileSystemID == "" {
//		return fmt.Errorf("fileSystemId not found in volume %s", volume.Name)
//	}
//
//	// 删除文件系统
//	_, err := p.client.DeleteFileSystem(&filenas.DeleteFileSystemInput{
//		FileSystemId: &fileSystemID,
//	})
//	if err != nil {
//		return fmt.Errorf("delete file system %s error: %v", fileSystemID, err)
//	}
//
//	logrus.Infof("deleted volcengine nas filesystem %s for volume %s", fileSystemID, volume.Name)
//	return nil
//}
//
//func (p *volcengineNasProvisioner) waitForFileSystemReady(fileSystemID string) error {
//	for i := 0; i < 60; i++ { // 最多等待 5 分钟
//		resp, err := p.client.DescribeFileSystems(&filenas.DescribeFileSystemsInput{
//			FileSystemIds: []string{fileSystemID},
//		})
//		if err != nil {
//			return err
//		}
//		if len(resp.FileSystems) == 0 {
//			return fmt.Errorf("file system %s not found", fileSystemID)
//		}
//
//		status := resp.FileSystems[0].Status
//		if status == "Running" {
//			return nil
//		}
//
//		if status == "Error" {
//			return fmt.Errorf("file system %s creation failed", fileSystemID)
//		}
//
//		// 等待 5 秒后重试
//		time.Sleep(5 * time.Second)
//	}
//
//	return fmt.Errorf("timeout waiting for file system %s to be ready", fileSystemID)
//}
//
//func (p *volcengineNasProvisioner) Name() string {
//	return p.name
//}
