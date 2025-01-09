package volume

import (
	"context"
	"fmt"
	"time"

	"github.com/goodrain/rainbond/db"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/component/filepersistence"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	workerutil "github.com/goodrain/rainbond/worker/util"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VolcengineVolume VolcengineVolume
type VolcengineVolume struct {
	Base
}

// CreateVolume ceph rbd volume create volume
func (v *VolcengineVolume) CreateVolume(define *Define) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
<<<<<<< HEAD
	sc, err := k8s.Default().Clientset.StorageV1().StorageClasses().Get(ctx, v.as.K8sComponentName, metav1.GetOptions{})
=======
	sc, err := k8s.Default().Clientset.StorageV1().StorageClasses().Get(ctx, define.as.K8sComponentName, metav1.GetOptions{})
>>>>>>> 19d035f8c6ea4bbbb88571c8e156db6f2a9ab732
	if err != nil {
		if k8serror.IsNotFound(err) {
			fileDomain, err := filepersistence.Default().FilePersistenceCli.CreateFileSystem(
				ctx,
				&filepersistence.CreateFileSystemOptions{
<<<<<<< HEAD
					Name:           v.as.K8sComponentName,
=======
					Name:           define.as.K8sComponentName,
>>>>>>> 19d035f8c6ea4bbbb88571c8e156db6f2a9ab732
					ProtocolType:   "NFS",
					StorageType:    "Standard",
					Size:           100 * 1024 * 1024 * 1024,
					FileSystemType: "Capacity",
				},
			)
			if err != nil {
				return fmt.Errorf("create file system failure:%v", err)
			}

			reclaimPolicy := corev1.PersistentVolumeReclaimDelete
			volumeBindingMode := storagev1.VolumeBindingImmediate
			sc = &storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
<<<<<<< HEAD
					Name: v.as.K8sComponentName,
=======
					Name: define.as.K8sComponentName,
>>>>>>> 19d035f8c6ea4bbbb88571c8e156db6f2a9ab732
				},
				Provisioner: "nas.csi.volcengine.com",
				Parameters: map[string]string{
					"ChargeType":      "PostPaid",
					"archiveOnDelete": "false",
					"fsType":          "Capacity",
					"server":          fileDomain,
					"subPath":         "/",
					"volumeAs":        "subpath",
				},
				MountOptions: []string{
					"nolock,proto=tcp,noresvport",
					"vers=3",
				},
				ReclaimPolicy:     &reclaimPolicy,
				VolumeBindingMode: &volumeBindingMode,
			}

			sc, err = k8s.Default().Clientset.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("create storage class failure: %v", err)
			}
		} else {
			return fmt.Errorf("get storage class failure: %v", err)
		}
	}
	v.as.SharedStorageClass = sc.Name

	statefulset := v.as.GetStatefulSet() //有状态组件
	if v.svm.VolumeType == dbmodel.ShareFileVolumeType.String() {
		v.svm.VolumeType = v.as.SharedStorageClass
	}
	volumeType, err := db.GetManager().VolumeTypeDao().GetVolumeTypeByType(v.svm.VolumeType)
	if err != nil {
		logrus.Errorf("get volume type by type error: %s", err.Error())
		return fmt.Errorf("validate volume capacity error")
	}
	if err := workerutil.ValidateVolumeCapacity(volumeType.CapacityValidation, v.svm.VolumeCapacity); err != nil {
		logrus.Errorf("validate volume capacity[%v] error: %s", v.svm.VolumeCapacity, err.Error())
		return err
	}
	v.svm.VolumeProviderName = volumeType.Provisioner
	volumeMountName := fmt.Sprintf("manual%d", v.svm.ID)
	volumeMountPath := v.svm.VolumePath
	volumeReadOnly := v.svm.IsReadOnly
	labels := v.as.GetCommonLabels(map[string]string{"volume_name": v.svm.VolumeName, "version": v.as.DeployVersion, "reclaim_policy": v.svm.ReclaimPolicy})
	annotations := map[string]string{"volume_name": v.svm.VolumeName}
	if statefulset == nil {
		v.svm.AccessMode = "RWX"
	}
	claim := newVolumeClaim(volumeMountName, volumeMountPath, v.svm.AccessMode, v.svm.VolumeType, v.svm.VolumeCapacity, labels, annotations)
	logrus.Debugf("storage class is : %s, claim value is : %s", v.svm.VolumeType, claim.GetName())
	v.as.SetClaim(claim) // store claim to appService
	vo := corev1.Volume{Name: volumeMountName}
	vo.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claim.GetName(), ReadOnly: volumeReadOnly}
	if statefulset != nil {
		statefulset.Spec.VolumeClaimTemplates = append(statefulset.Spec.VolumeClaimTemplates, *claim)
		logrus.Debugf("stateset.Spec.VolumeClaimTemplates: %+v", statefulset.Spec.VolumeClaimTemplates)
	} else {
		v.as.SetClaimManually(claim)
		define.volumes = append(define.volumes, vo)
	}

	vm := corev1.VolumeMount{
		Name:      volumeMountName,
		MountPath: volumeMountPath,
		ReadOnly:  volumeReadOnly,
	}
	define.volumeMounts = append(define.volumeMounts, vm)
	return nil
}

// CreateDependVolume create depend volume
func (v *VolcengineVolume) CreateDependVolume(define *Define) error {
	return nil
}
