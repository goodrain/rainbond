package healthy

import (
	"testing"
	"github.com/goodrain/rainbond/node/nodem/service"
	"fmt"
)

func TestProbeManager_Start(t *testing.T) {
	m := CreateManager()

	serviceList := make([]*service.Service, 0, 10)

	h := &service.Service{
		Name: "builder",
		ServiceHealth: &service.Health{
			Name:         "builder",
			Model:        "http",
			Address:      "127.0.0.1:6369/worker/health",
			TimeInterval: 3,
			MaxErrorNumber:3,
		},
	}
	h2 := &service.Service{
		Name: "worker",
		ServiceHealth: &service.Health{
			Name:         "worker",
			Model:        "http",
			Address:      "127.0.0.1:6369/worker/health",
			TimeInterval: 3,
			MaxErrorNumber:3,

		},
	}
	h3 := &service.Service{
		Name: "webcli",
		ServiceHealth: &service.Health{
			Name:         "webcli",
			Model:        "http",
			Address:      "127.0.0.1:7171/health",
			TimeInterval: 3,
			MaxErrorNumber:3,

		},
	}
	serviceList = append(serviceList, h)
	serviceList = append(serviceList, h2)
	serviceList = append(serviceList, h3)
	m.AddServices(serviceList)
	watcher := m.WatchServiceHealthy("webcli")

	m.Start()


	for {
		v := watcher.Watch()
		fmt.Println("----",v.Name, v.Status, v.Info)

	}
}


//func TestGetHttpHealth(t *testing.T) {
//	ctx, cancel := context.WithCancel(context.Background())
//	serviceList := make([]*service.Service, 0, 10)
//
//	h := &service.Service{
//		Name: "builder",
//		ServiceHealth: &service.Health{
//			Name:    "builder",
//			Model:   "http",
//			Address: "127.0.0.1:3228",
//			Path:    "/v2/builder/health",
//		},
//	}
//	serviceList = append(serviceList, h)
//	v := ProbeManager{
//		ctx:      ctx,
//		cancel:   cancel,
//		services: serviceList,
//	}
//	result := v.GetServiceHealthy("builder")
//	fmt.Println(result.Name, result.Status, result.Info)
//}

//func TestProbeManager_Start(t *testing.T) {
//		ctx, cancel := context.WithCancel(context.Background())
//		serviceList := make([]*service.Service, 0, 10)
//
//		h := &service.Service{
//			Name: "builder",
//			ServiceHealth: &service.Health{
//				Name:    "builder",
//				Model:   "http",
//				Address: "127.0.0.1:3228",
//				Path:    "/v2/builder/health",
//			},
//		}
//		serviceList = append(serviceList, h)
//		v := ProbeManager{
//			ctx:      ctx,
//			cancel:   cancel,
//			services: serviceList,
//		}
//		v.Start()
//
//
//}