package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	ctxutil "github.com/goodrain/rainbond/api/util/ctx"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/jinzhu/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/api/core/v1"
	kubecli "kubevirt.io/client-go/kubecli"
)

type wrapELTestManager struct {
	db.Manager
	eventDao dbdao.EventDao
}

func (m wrapELTestManager) ServiceEventDao() dbdao.EventDao {
	return m.eventDao
}

type wrapELTestEventDao struct {
	dbdao.EventDao
	added int
}

func (d *wrapELTestEventDao) GetLastASyncEvent(target, targetID string) (*dbmodel.ServiceEvent, error) {
	return nil, gorm.ErrRecordNotFound
}

func (d *wrapELTestEventDao) AddModel(arg dbmodel.Interface) error {
	d.added++
	return nil
}

// capability_id: rainbond.vm-live-update.running-shrink-rejected-before-event
func TestWrapELRejectsRunningVMShrinkBeforeCreatingEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eventDao := &wrapELTestEventDao{}
	db.SetTestManager(wrapELTestManager{eventDao: eventDao})
	defer db.SetTestManager(nil)

	mockClient := kubecli.NewMockKubevirtClient(ctrl)
	mockVMInterface := kubecli.NewMockVirtualMachineInterface(ctrl)
	k8sComponent := k8s.New()
	k8sComponent.KubevirtCli = mockClient

	mockClient.EXPECT().VirtualMachine("").Return(mockVMInterface).Times(2)
	mockVMInterface.EXPECT().List(gomock.Any(), metav1.ListOptions{LabelSelector: "service_id=service-vm"}).Return(&v1.VirtualMachineList{
		Items: []v1.VirtualMachine{
			{
				Status: v1.VirtualMachineStatus{PrintableStatus: v1.VirtualMachineStatusRunning},
			},
		},
	}, nil).Times(2)

	testCases := []struct {
		name           string
		body           string
		expectedReason string
	}{
		{
			name:           "cpu_shrink",
			body:           `{"container_cpu":4000,"container_memory":12288}`,
			expectedReason: "虚拟机 CPU 热更新仅支持扩容，不支持缩容，请停机后再修改规格。",
		},
		{
			name:           "memory_shrink",
			body:           `{"container_cpu":6000,"container_memory":8192}`,
			expectedReason: "虚拟机内存热更新仅支持扩容，不支持缩容，请停机后再修改规格。",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			handler := WrapEL(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}, dbmodel.TargetTypeService, "vertical-service", dbmodel.ASYNEVENTTYPE, false)

			req := httptest.NewRequest(http.MethodPut, "/v2/tenants/demo/services/service-vm/vertical", bytes.NewBufferString(tc.body))
			ctx := context.WithValue(req.Context(), ctxutil.ContextKey("tenant_id"), "tenant-1")
			ctx = context.WithValue(ctx, ctxutil.ContextKey("service_id"), "service-vm")
			ctx = context.WithValue(ctx, ctxutil.ContextKey("service"), &dbmodel.TenantServices{
				ServiceID:       "service-vm",
				ExtendMethod:    "vm",
				ContainerCPU:    6000,
				ContainerMemory: 12288,
			})
			req = req.WithContext(ctx)

			resp := httptest.NewRecorder()
			handler(resp, req)

			if resp.Code != http.StatusConflict {
				t.Fatalf("expected status 409, got %d", resp.Code)
			}
			if called {
				t.Fatal("expected request to be rejected before reaching wrapped handler")
			}
			if !strings.Contains(resp.Body.String(), tc.expectedReason) {
				t.Fatalf("expected rejection reason %q, got %s", tc.expectedReason, resp.Body.String())
			}
		})
	}

	if eventDao.added != 0 {
		t.Fatalf("expected no events to be created for rejected vm shrink requests, got %d", eventDao.added)
	}
}
