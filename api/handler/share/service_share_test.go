package share

import (
	"context"
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	mqpb "github.com/goodrain/rainbond/mq/api/grpc/pb"
	mqclient "github.com/goodrain/rainbond/mq/client"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

type shareTestManager struct {
	db.Manager
	tenantServiceDao dbdao.TenantServiceDao
	versionInfoDao   dbdao.VersionInfoDao
	labelDao         dbdao.TenantServiceLabelDao
}

func (m shareTestManager) TenantServiceDao() dbdao.TenantServiceDao {
	return m.tenantServiceDao
}

func (m shareTestManager) VersionInfoDao() dbdao.VersionInfoDao {
	return m.versionInfoDao
}

func (m shareTestManager) TenantServiceLabelDao() dbdao.TenantServiceLabelDao {
	return m.labelDao
}

type shareTestTenantServiceDao struct {
	dbdao.TenantServiceDao
	service *dbmodel.TenantServices
}

func (d *shareTestTenantServiceDao) GetServiceByID(serviceID string) (*dbmodel.TenantServices, error) {
	return d.service, nil
}

type shareTestVersionInfoDao struct {
	dbdao.VersionInfoDao
	versions         map[string]*dbmodel.VersionInfo
	requestedVersion string
}

func (d *shareTestVersionInfoDao) GetVersionByDeployVersion(version, serviceID string) (*dbmodel.VersionInfo, error) {
	d.requestedVersion = version
	return d.versions[version], nil
}

type shareTestLabelDao struct {
	dbdao.TenantServiceLabelDao
}

func (d *shareTestLabelDao) GetLabelByNodeSelectorKey(serviceID string, labelValue string) (*dbmodel.TenantServiceLable, error) {
	return nil, nil
}

type recordingMQClient struct {
	tasks []mqclient.TaskStruct
}

func (m *recordingMQClient) Enqueue(ctx context.Context, in *mqpb.EnqueueRequest, opts ...grpc.CallOption) (*mqpb.TaskReply, error) {
	return &mqpb.TaskReply{}, nil
}

func (m *recordingMQClient) Topics(ctx context.Context, in *mqpb.TopicRequest, opts ...grpc.CallOption) (*mqpb.TaskReply, error) {
	return &mqpb.TaskReply{}, nil
}

func (m *recordingMQClient) Dequeue(ctx context.Context, in *mqpb.DequeueRequest, opts ...grpc.CallOption) (*mqpb.TaskMessage, error) {
	return &mqpb.TaskMessage{}, nil
}

func (m *recordingMQClient) Close() {}

func (m *recordingMQClient) SendBuilderTopic(t mqclient.TaskStruct) error {
	m.tasks = append(m.tasks, t)
	return nil
}

// capability_id: rainbond.share.image-from-snapshot-deploy-version
func TestServiceShareUsesRequestedDeployVersionForImageShare(t *testing.T) {
	service := &dbmodel.TenantServices{
		ServiceID:     "service-id",
		ServiceAlias:  "service-alias",
		TenantID:      "tenant-id",
		DeployVersion: "current-deploy-version",
	}
	versionDao := &shareTestVersionInfoDao{
		versions: map[string]*dbmodel.VersionInfo{
			"snapshot-deploy-version": {
				ServiceID:     service.ServiceID,
				BuildVersion:  "snapshot-build-version",
				DeliveredType: "image",
				DeliveredPath: "registry.local/source/ns/original:image",
				FinalStatus:   "success",
			},
		},
	}
	db.SetTestManager(shareTestManager{
		tenantServiceDao: &shareTestTenantServiceDao{service: service},
		versionInfoDao:   versionDao,
		labelDao:         &shareTestLabelDao{},
	})
	defer db.SetTestManager(nil)

	mq := &recordingMQClient{}
	handle := &ServiceShareHandle{MQClient: mq}
	req := apimodel.ServiceShare{
		TenantName:   "demo-team",
		ServiceAlias: service.ServiceAlias,
	}
	req.Body.ServiceKey = "service-key"
	req.Body.AppVersion = "1.2.3"
	req.Body.DeployVersion = "snapshot-deploy-version"
	req.Body.EventID = "event-id"
	req.Body.ImageInfo.HubURL = "registry.target"
	req.Body.ImageInfo.Namespace = "snapshot-space"

	res, apiErr := handle.Share(service.ServiceID, req)

	assert.Nil(t, apiErr)
	if assert.NotNil(t, res) {
		assert.Equal(t, "registry.target/snapshot-space/service-id:snapshot-build-version", res.ImageName)
	}
	if assert.Len(t, mq.tasks, 1) {
		body := mq.tasks[0].TaskBody.(map[string]interface{})
		assert.Equal(t, "registry.local/source/ns/original:image", body["local_image_name"])
	}
	assert.Equal(t, "snapshot-deploy-version", versionDao.requestedVersion)
}

// capability_id: rainbond.share.slug-from-snapshot-deploy-version
func TestServiceShareUsesRequestedDeployVersionForSlugShare(t *testing.T) {
	service := &dbmodel.TenantServices{
		ServiceID:     "service-id",
		ServiceAlias:  "service-alias",
		ServiceKey:    "service-key",
		TenantID:      "tenant-id",
		DeployVersion: "current-deploy-version",
	}
	versionDao := &shareTestVersionInfoDao{
		versions: map[string]*dbmodel.VersionInfo{
			"snapshot-deploy-version": {
				ServiceID:     service.ServiceID,
				BuildVersion:  "snapshot-build-version",
				DeliveredType: "slug",
				DeliveredPath: "",
				FinalStatus:   "success",
			},
		},
	}
	db.SetTestManager(shareTestManager{
		tenantServiceDao: &shareTestTenantServiceDao{service: service},
		versionInfoDao:   versionDao,
		labelDao:         &shareTestLabelDao{},
	})
	defer db.SetTestManager(nil)

	mq := &recordingMQClient{}
	handle := &ServiceShareHandle{MQClient: mq}
	req := apimodel.ServiceShare{
		TenantName:   "demo-team",
		ServiceAlias: service.ServiceAlias,
	}
	req.Body.ServiceKey = service.ServiceKey
	req.Body.AppVersion = "1.2.3"
	req.Body.DeployVersion = "snapshot-deploy-version"
	req.Body.EventID = "event-id"
	req.Body.SlugInfo.Namespace = "snapshot-space"

	res, apiErr := handle.Share(service.ServiceID, req)

	assert.Nil(t, apiErr)
	assert.NotNil(t, res)
	if assert.Len(t, mq.tasks, 1) {
		body := mq.tasks[0].TaskBody.(map[string]interface{})
		assert.Equal(t, "/grdata/build/tenant/tenant-id/slug/service-id/snapshot-deploy-version.tgz", body["local_slug_path"])
		assert.Equal(t, "/grdata/build/tenant/snapshot-space/service-key/1.2.3_snapshot-deploy-version.tgz", body["slug_path"])
	}
	assert.Equal(t, "snapshot-deploy-version", versionDao.requestedVersion)
}
