package handler

import (
	"os"
	"testing"

	apimodel "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/db"
	dbdao "github.com/goodrain/rainbond/db/dao"
	dbmodel "github.com/goodrain/rainbond/db/model"
)

type gatewayRouteDeleteEventTestManager struct {
	db.Manager
	portDao  dbdao.TenantServicesPortDao
	eventDao dbdao.EventDao
}

func (m gatewayRouteDeleteEventTestManager) TenantServicesPortDao() dbdao.TenantServicesPortDao {
	return m.portDao
}

func (m gatewayRouteDeleteEventTestManager) ServiceEventDao() dbdao.EventDao {
	return m.eventDao
}

type gatewayRouteDeleteEventPortDao struct {
	dbdao.TenantServicesPortDao
	portsByName map[string]*dbmodel.TenantServicesPort
}

func (d *gatewayRouteDeleteEventPortDao) ListByK8sServiceNames(names []string) ([]*dbmodel.TenantServicesPort, error) {
	var ports []*dbmodel.TenantServicesPort
	for _, name := range names {
		if port, ok := d.portsByName[name]; ok {
			ports = append(ports, port)
		}
	}
	return ports, nil
}

type gatewayRouteDeleteEventDao struct {
	dbdao.EventDao
	events []*dbmodel.ServiceEvent
}

func (d *gatewayRouteDeleteEventDao) AddModel(arg dbmodel.Interface) error {
	d.events = append(d.events, arg.(*dbmodel.ServiceEvent))
	return nil
}

// capability_id: rainbond.gateway.allocate-lb-port
func TestSelectAvailablePort(t *testing.T) {
	// 设置环境变量
	os.Setenv("MIN_LB_PORT", "30000")
	os.Setenv("MAX_LB_PORT", "65535")

	tests := []struct {
		name     string
		used     []int
		expected int
	}{
		{
			name:     "空列表，返回最小端口",
			used:     []int{},
			expected: 30000,
		},
		{
			name:     "连续端口，返回下一个",
			used:     []int{30000, 30001, 30002},
			expected: 30003,
		},
		{
			name:     "有间隙，返回第一个空闲端口",
			used:     []int{30000, 30002, 30003},
			expected: 30001,
		},
		{
			name:     "大间隙，返回第一个空闲端口",
			used:     []int{30000, 32077},
			expected: 30001,
		},
		{
			name:     "乱序输入，返回第一个空闲端口",
			used:     []int{30002, 30000, 30003},
			expected: 30001,
		},
		{
			name:     "从中间开始有间隙",
			used:     []int{30000, 30001, 30002, 30005, 30006},
			expected: 30003,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := selectAvailablePort(tt.used)
			if result != tt.expected {
				t.Errorf("selectAvailablePort(%v) = %d, expected %d", tt.used, result, tt.expected)
			}
		})
	}
}

// capability_id: rainbond.gateway.reassign-conflicting-imported-tcp-port
func TestReassignConflictingTCPRulePorts(t *testing.T) {
	t.Setenv("MIN_LB_PORT", "30000")
	t.Setenv("MAX_LB_PORT", "30010")

	existing := []*dbmodel.TCPRule{
		{
			ServiceID: "source-service",
			IP:        "0.0.0.0",
			Port:      30000,
		},
	}
	incoming := []*dbmodel.TCPRule{
		{
			UUID:          "imported-rule",
			ServiceID:     "installed-service",
			ContainerPort: 8080,
			IP:            "0.0.0.0",
			Port:          30000,
		},
	}

	err := reassignConflictingTCPRulePorts(existing, incoming)

	if err != nil {
		t.Fatalf("reassignConflictingTCPRulePorts returned error: %v", err)
	}
	if incoming[0].Port != 30001 {
		t.Fatalf("incoming TCP rule port = %d, expected 30001", incoming[0].Port)
	}
}

// capability_id: rainbond.gateway.http-route-delete-component-event
func TestCreateGatewayHTTPRouteDeleteEvents(t *testing.T) {
	eventDao := &gatewayRouteDeleteEventDao{}
	portDao := &gatewayRouteDeleteEventPortDao{
		portsByName: map[string]*dbmodel.TenantServicesPort{
			"svc-a": {
				TenantID:      "tenant-a",
				ServiceID:     "component-a",
				ContainerPort: 80,
			},
			"svc-b": {
				TenantID:      "tenant-b",
				ServiceID:     "component-b",
				ContainerPort: 8080,
			},
		},
	}
	db.SetTestManager(gatewayRouteDeleteEventTestManager{portDao: portDao, eventDao: eventDao})
	defer db.SetTestManager(nil)

	route := &apimodel.GatewayHTTPRouteStruct{
		Name:  "route-a",
		Hosts: []string{"example.com"},
		Rules: []*apimodel.Rules{
			{
				BackendRefsRules: []*apimodel.BackendRefsRule{
					{Name: "svc-a", Kind: apimodel.Service, Port: 80},
					{Name: "svc-b", Kind: apimodel.Service, Port: 8080},
					{Name: "svc-a", Kind: apimodel.Service, Port: 80},
				},
			},
		},
	}

	events, err := (&GatewayAction{}).createGatewayHTTPRouteDeleteEvents(route, "alice")

	if err != nil {
		t.Fatalf("createGatewayHTTPRouteDeleteEvents returned error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if len(eventDao.events) != 2 {
		t.Fatalf("expected 2 persisted events, got %d", len(eventDao.events))
	}
	for _, event := range eventDao.events {
		if event.Target != dbmodel.TargetTypeService {
			t.Fatalf("event target = %s, expected %s", event.Target, dbmodel.TargetTypeService)
		}
		if event.OptType != "delete-gateway-http-route" {
			t.Fatalf("event opt_type = %s, expected delete-gateway-http-route", event.OptType)
		}
		if event.UserName != "alice" {
			t.Fatalf("event user = %s, expected alice", event.UserName)
		}
		if event.SynType != dbmodel.SYNEVENTTYPE {
			t.Fatalf("event syn_type = %d, expected %d", event.SynType, dbmodel.SYNEVENTTYPE)
		}
	}
	if eventDao.events[0].TargetID != "component-a" || eventDao.events[1].TargetID != "component-b" {
		t.Fatalf("unexpected event target IDs: %s, %s", eventDao.events[0].TargetID, eventDao.events[1].TargetID)
	}
}
