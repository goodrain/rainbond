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
			Model:        "cmd",
			Address:      "lsx",
			TimeInterval: 3,
		},
	}
	h2 := &service.Service{
		Name: "worker",
		ServiceHealth: &service.Health{
			Name:         "worker",
			Model:        "http",
			Address:      "127.0.0.1:6369/worker/health",
			TimeInterval: 3,
		},
	}
	h3 := &service.Service{
		Name: "webcli",
		ServiceHealth: &service.Health{
			Name:         "webcli",
			Model:        "http",
			Address:      "127.0.0.1:7171/health",
			TimeInterval: 3,

		},
	}
	serviceList = append(serviceList, h)
	serviceList = append(serviceList, h2)
	serviceList = append(serviceList, h3)
	m.AddServices(serviceList)
	watcher1 := m.WatchServiceHealthy("webcli")
	watcher2 := m.WatchServiceHealthy("worker")
	watcher3 := m.WatchServiceHealthy("builder")

	m.Start()


for {
	v := <-watcher1.Watch()
	if v!=nil{

		fmt.Println("----",v.Name, v.Status, v.Info,v.ErrorNumber,v.ErrorTime.Seconds())
	}else{
		t.Log("nil nil nil")
	}

	v2 := <-watcher2.Watch()
	fmt.Println("===",v2.Name, v2.Status, v2.Info,v2.ErrorNumber,v2.ErrorTime.Seconds())
	v3 := <-watcher3.Watch()
	fmt.Println("vvvv",v3.Name, v3.Status, v3.Info,v3.ErrorNumber,v3.ErrorTime.Seconds())
}

}


//func TestGetHttpHealth(t *testing.T) {
//	m := CreateManager()
//	serviceList := make([]*service.Service, 0, 10)
//
//	h := &service.Service{
//		Name: "builder",
//		ServiceHealth: &service.Health{
//			Name:         "builder",
//			Model:        "tcp",
//			Address:      "127.0.0.1:3228",
//			TimeInterval: 3,
//		},
//	}
//	h2 := &service.Service{
//		Name: "worker",
//		ServiceHealth: &service.Health{
//			Name:         "worker",
//			Model:        "http",
//			Address:      "127.0.0.1:6369/worker/health",
//			TimeInterval: 3,
//		},
//	}
//	h3 := &service.Service{
//		Name: "webcli",
//		ServiceHealth: &service.Health{
//			Name:         "webcli",
//			Model:        "http",
//			Address:      "127.0.0.1:7171/health",
//			TimeInterval: 3,
//
//		},
//	}
//	serviceList = append(serviceList, h)
//	serviceList = append(serviceList, h2)
//	serviceList = append(serviceList, h3)
//	m.AddServices(serviceList)
//	m.Start()
//
//	for   {
//
//		time.Sleep(time.Second*1)
//		info, ok := m.GetServiceHealthy("builder")
//		if !ok {
//			fmt.Println("cuowu")
//		} else {
//			fmt.Println(info.Name, info.Status, info.Info, info.ErrorNumber, info.ErrorTime)
//
//	}
//
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