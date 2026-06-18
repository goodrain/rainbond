package handler

import (
	"context"
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util/bcode"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

type applicationPortsTestManager struct {
	db.Manager
	tenantServiceDao      dbdao.TenantServiceDao
	tenantServicesPortDao dbdao.TenantServicesPortDao
}

func (m applicationPortsTestManager) TenantServiceDao() dbdao.TenantServiceDao {
	return m.tenantServiceDao
}

func (m applicationPortsTestManager) TenantServicesPortDao() dbdao.TenantServicesPortDao {
	return m.tenantServicesPortDao
}

type applicationPortsTenantServiceDao struct {
	dbdao.TenantServiceDao
	services       []*dbmodel.TenantServices
	err            error
	requestedAppID string
}

func (d *applicationPortsTenantServiceDao) ListByAppID(appID string) ([]*dbmodel.TenantServices, error) {
	d.requestedAppID = appID
	return d.services, d.err
}

type applicationPortsTenantServicesPortDao struct {
	dbdao.TenantServicesPortDao
	ports          []*dbmodel.TenantServicesPort
	err            error
	requestedNames []string
}

func (d *applicationPortsTenantServicesPortDao) ListByK8sServiceNames(names []string) ([]*dbmodel.TenantServicesPort, error) {
	d.requestedNames = append([]string(nil), names...)
	return d.ports, d.err
}

// capability_id: rainbond.application.check-port-k8s-service-name-duplicate
func TestApplicationActionCheckPortsRejectsDuplicateK8sServiceName(t *testing.T) {
	tenantServiceDao := &applicationPortsTenantServiceDao{
		services: []*dbmodel.TenantServices{
			{ServiceID: "service-1"},
			{ServiceID: "service-2"},
		},
	}
	tenantServicesPortDao := &applicationPortsTenantServicesPortDao{
		ports: []*dbmodel.TenantServicesPort{
			{
				ServiceID:      "service-2",
				ContainerPort:  5000,
				K8sServiceName: "shared-service",
			},
		},
	}
	db.SetTestManager(applicationPortsTestManager{
		tenantServiceDao:      tenantServiceDao,
		tenantServicesPortDao: tenantServicesPortDao,
	})
	defer db.SetTestManager(nil)

	action := &ApplicationAction{}
	err := action.checkPorts("app-1", []*apimodel.AppPort{
		{
			ServiceID:      "service-1",
			ContainerPort:  5000,
			PortAlias:      "WEB",
			K8sServiceName: "shared-service",
		},
	})

	assert.Equal(t, "app-1", tenantServiceDao.requestedAppID)
	assert.Equal(t, []string{"shared-service"}, tenantServicesPortDao.requestedNames)
	assert.Error(t, err)
	assert.True(t, bcode.ErrK8sServiceNameExists.Equal(err))
}

func TestApplicationActionGetPVCDiskRequestsKBByAppID(t *testing.T) {
	kubeClient := k8sfake.NewSimpleClientset(
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "data-mysql-0",
				Namespace: "demo",
				Labels: map[string]string{
					"app_id": "app-1",
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("10Gi"),
					},
				},
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "data-other-0",
				Namespace: "demo",
				Labels: map[string]string{
					"app_id": "app-2",
				},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("20Gi"),
					},
				},
			},
		},
	)
	action := &ApplicationAction{kubeClient: kubeClient}

	got, err := action.getPVCDiskRequestsKB(context.Background(), "app-1")

	assert.NoError(t, err)
	assert.Equal(t, int64(10*1024*1024), got)
}

func TestApplicationActionGetPVCDiskRequestsKBFromAppPods(t *testing.T) {
	kubeClient := k8sfake.NewSimpleClientset(
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "data-mysql-0",
				Namespace: "demo",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("10Gi"),
					},
				},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mysql-0",
				Namespace: "demo",
				Labels: map[string]string{
					"app_id": "app-1",
				},
			},
			Spec: corev1.PodSpec{
				Volumes: []corev1.Volume{
					{
						Name: "data",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "data-mysql-0",
							},
						},
					},
				},
			},
		},
	)
	action := &ApplicationAction{kubeClient: kubeClient}

	got, err := action.getPVCDiskRequestsKB(context.Background(), "app-1")

	assert.NoError(t, err)
	assert.Equal(t, int64(10*1024*1024), got)
}
