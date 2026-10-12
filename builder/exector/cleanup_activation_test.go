package exector

import (
	"errors"
	"testing"

	"github.com/goodrain/rainbond/db"
	"github.com/goodrain/rainbond/db/dao"
	"github.com/goodrain/rainbond/db/model"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/mq/api/grpc/pb"
)

type deniedVMActivationServices struct {
	dao.TenantServiceDao
	calls int
}

func (s *deniedVMActivationServices) UpdateDeployVersion(string, string) error {
	s.calls++
	return errors.New("retired version")
}

type vmActivationEvents struct {
	dao.EventDao
	created int
}

func (e *vmActivationEvents) AddModel(model.Interface) error {
	e.created++
	return errors.New("unexpected deployment dispatch")
}

type vmActivationDB struct {
	db.Manager
	events   *vmActivationEvents
	services *deniedVMActivationServices
}

func (m vmActivationDB) TenantServiceDao() dao.TenantServiceDao { return m.services }
func (m vmActivationDB) ServiceEventDao() dao.EventDao          { return m.events }

type vmActivationLogs struct{ stubEventManager }

func (vmActivationLogs) GetLogger(id string) event.Logger {
	return event.NewLogger(id, make(chan []byte, 20))
}

// capability_id: rainbond.cleanup.vm-activation-dispatch
func TestVMBuildCannotDispatchAfterActivationRejected(t *testing.T) {
	events := &vmActivationEvents{}
	services := &deniedVMActivationServices{}
	db.SetTestManager(vmActivationDB{events: events, services: services})
	defer db.SetTestManager(nil)
	event.NewTestManager(&vmActivationLogs{})
	defer event.NewTestManager(nil)
	// An already-built VM image skips conversion. Rejected activation must still
	// prevent creating a deployment event or dispatching the retained task.
	(&exectorManager{}).buildFromVM(&pb.TaskMessage{TaskBody: []byte(`{"event_id":"test-vm-activation","service_id":"service","tenant_id":"tenant","deploy_version":"retired","action":"upgrade"}`)})
	if services.calls != 1 {
		t.Fatal("test did not reach version activation")
	}
	if events.created != 0 {
		t.Fatal("deployment was dispatched after activation rejection")
	}
}
