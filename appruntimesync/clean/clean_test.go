package clean

//type TestManager struct {
//	ctx           context.Context
//	kubeclient    *kubernetes.Clientset
//	waiting       []Resource
//	queryResource []func(*Manager) []Resource
//	cancel        context.CancelFunc
//	//l             list.List
//	//dclient       *client.Client
//}

//func TestNewManager(t *testing.T) (*Manager, error) {
//	c, err := clientcmd.BuildConfigFromFlags("", "../../../test/admin.kubeconfig")
//	if err != nil {
//		logrus.Error("read kube config file error.", err)
//		return nil,err
//	}
//	clientset, err := kubernetes.NewForConfig(c)
//	if err != nil {
//		logrus.Error("create kube api client error", err)
//		return nil,err
//	}
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	m := &Manager{
//		ctx:        ctx,
//		kubeclient: clientset,
//		cancel:     cancel,
//
//	}
//	m.CollectingTasks()
//	return m,nil
//}

